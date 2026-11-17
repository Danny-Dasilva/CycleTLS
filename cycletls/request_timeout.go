package cycletls

import (
	"context"
	"io"
	"time"

	fhttp "github.com/Danny-Dasilva/fhttp"
)

const (
	defaultTimeoutSeconds      = 15
	defaultTimeoutMilliseconds = 30000
)

type requestResult struct {
	resp *fhttp.Response
	err  error
}

// timeoutSeconds converts a timeout value in seconds to time.Duration.
// Returns 0 for negative values (no timeout), default for 0, or the specified value.
func timeoutSeconds(timeout int) time.Duration {
	return convertTimeout(timeout, defaultTimeoutSeconds, time.Second)
}

// timeoutMilliseconds converts a timeout value in milliseconds to time.Duration.
// Returns 0 for negative values (no timeout), default for 0, or the specified value.
func timeoutMilliseconds(timeout int) time.Duration {
	return convertTimeout(timeout, defaultTimeoutMilliseconds, time.Millisecond)
}

// convertTimeout is the shared implementation for timeout conversion.
func convertTimeout(timeout, defaultValue int, unit time.Duration) time.Duration {
	if timeout < 0 {
		return 0
	}
	if timeout == 0 {
		return time.Duration(defaultValue) * unit
	}
	return time.Duration(timeout) * unit
}

// doRequestWithHeaderTimeout performs an HTTP request with a timeout for headers to arrive.
// If timeout is <= 0, the request proceeds without a timeout.
//
// The request runs on a context derived from ctx, so this function never
// cancels the caller's context:
//   - On success the derived context stays alive until the response body is
//     closed. The headers have arrived but the body has not been read yet, and
//     cancelling here would cut it off. Closing the body releases the context,
//     so the caller must close it, as with any response.
//   - On an error, a header timeout or a cancelled ctx, the derived context is
//     cancelled before returning, which stops a request still in flight.
//
// cancel is not called. It is kept so callers that have no context of their own
// can pass nil for both and get a cancellable one from ensureContext.
func doRequestWithHeaderTimeout(
	ctx context.Context,
	cancel context.CancelFunc,
	client fhttp.Client,
	req *fhttp.Request,
	timeout time.Duration,
) (*fhttp.Response, error) {
	if timeout <= 0 {
		return client.Do(req)
	}

	if ctx == nil {
		ctx = req.Context()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, reqCancel := context.WithCancel(ctx)

	req = req.WithContext(reqCtx)

	resultCh := make(chan requestResult, 1)
	go func() {
		resp, err := client.Do(req)
		resultCh <- requestResult{resp: resp, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-resultCh:
		if result.err != nil || result.resp == nil {
			reqCancel()
			return result.resp, result.err
		}
		if result.resp.Body == nil {
			// Nothing to read and nothing for the caller to close.
			reqCancel()
			return result.resp, nil
		}
		result.resp.Body = &cancelOnCloseBody{ReadCloser: result.resp.Body, cancel: reqCancel}
		return result.resp, nil
	case <-timer.C:
		reqCancel()
		go discardLateResult(resultCh)
		return nil, context.DeadlineExceeded
	case <-ctx.Done():
		reqCancel()
		go discardLateResult(resultCh)
		return nil, ctx.Err()
	}
}

// cancelOnCloseBody releases the request context when the response body is
// closed. Until then the context has to stay alive for the body to be read.
type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// discardLateResult waits for a request the caller has stopped waiting for and
// closes its response body if one arrives, so the connection is not left
// holding an unread response. The request context is already cancelled, so
// client.Do returns promptly.
func discardLateResult(resultCh <-chan requestResult) {
	result := <-resultCh
	if result.resp != nil && result.resp.Body != nil {
		_ = result.resp.Body.Close()
	}
}

// ensureContext ensures a valid context and cancel function exist.
func ensureContext(ctx context.Context, cancel context.CancelFunc, req *fhttp.Request) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = req.Context()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cancel == nil {
		ctx, cancel = context.WithCancel(ctx)
	}
	return ctx, cancel
}
