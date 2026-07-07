package cycletls

import (
	"context"
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

	ctx, cancel = ensureContext(ctx, cancel, req)
	// Always clean up the context when this function returns.
	// This prevents context leaks when ensureContext creates a new cancel function.
	defer cancel()

	req = req.WithContext(ctx)

	resultCh := make(chan requestResult, 1)
	go func() {
		resp, err := client.Do(req)
		resultCh <- requestResult{resp: resp, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-resultCh:
		return result.resp, result.err
	case <-timer.C:
		return nil, context.DeadlineExceeded
	case <-ctx.Done():
		return nil, ctx.Err()
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
