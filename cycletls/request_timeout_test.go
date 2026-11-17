package cycletls

import (
	"context"
	"io"
	nhttp "net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	fhttp "github.com/Danny-Dasilva/fhttp"
)

func TestTimeoutSeconds_Default(t *testing.T) {
	d := timeoutSeconds(0)
	expected := time.Duration(defaultTimeoutSeconds) * time.Second
	if d != expected {
		t.Fatalf("expected %v, got %v", expected, d)
	}
}

func TestTimeoutSeconds_Negative(t *testing.T) {
	d := timeoutSeconds(-1)
	if d != 0 {
		t.Fatalf("expected 0, got %v", d)
	}
}

func TestTimeoutSeconds_Positive(t *testing.T) {
	d := timeoutSeconds(5)
	if d != 5*time.Second {
		t.Fatalf("expected 5s, got %v", d)
	}
}

func TestTimeoutMilliseconds_Default(t *testing.T) {
	d := timeoutMilliseconds(0)
	expected := time.Duration(defaultTimeoutMilliseconds) * time.Millisecond
	if d != expected {
		t.Fatalf("expected %v, got %v", expected, d)
	}
}

// Issue #10: ensureContext should not leak when cancel is created internally
func TestEnsureContext_CancelNotNil(t *testing.T) {
	// When both ctx and cancel are provided, they should be returned as-is
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gotCtx, gotCancel := ensureContext(ctx, cancel, nil)
	if gotCtx != ctx {
		t.Fatal("expected same context")
	}
	if gotCancel == nil {
		t.Fatal("expected non-nil cancel")
	}
}

func TestEnsureContext_CreatesCancel(t *testing.T) {
	// When cancel is nil, ensureContext creates one
	ctx := context.Background()
	gotCtx, gotCancel := ensureContext(ctx, nil, nil)
	if gotCancel == nil {
		t.Fatal("expected non-nil cancel function")
	}
	// The returned cancel should work
	gotCancel()
	select {
	case <-gotCtx.Done():
		// expected
	default:
		t.Fatal("cancel should have cancelled the returned context")
	}
}

// delayedBodyServer sends its headers at once and the body after a pause, so a
// caller that stops at the headers is caught out.
func delayedBodyServer(t *testing.T, body string, pause time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(nhttp.StatusOK)
		w.(nhttp.Flusher).Flush()
		select {
		case <-time.After(pause):
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const delayedBody = "body sent after the headers"

// The header timeout covers the wait for headers only. Once they arrive the
// caller's context must still be live and the body must still be readable.
// The function used to defer the caller's cancel, which cut every body off.
func TestDoRequestWithHeaderTimeout_BodyReadableAfterHeaders(t *testing.T) {
	srv := delayedBodyServer(t, delayedBody, 150*time.Millisecond)

	for _, withCancel := range []bool{true, false} {
		name := "caller_cancel"
		if !withCancel {
			name = "nil_cancel"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			passedCancel := cancel
			if !withCancel {
				passedCancel = nil
			}

			req, err := fhttp.NewRequestWithContext(ctx, "GET", srv.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := doRequestWithHeaderTimeout(ctx, passedCancel, fhttp.Client{}, req, 5*time.Second)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if ctx.Err() != nil {
				t.Fatalf("caller context was cancelled when the headers arrived: %v", ctx.Err())
			}

			got, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("body read failed: %v", err)
			}
			if string(got) != delayedBody {
				t.Fatalf("body = %q, want %q", got, delayedBody)
			}

			// Issue #10: the context made for the request must not outlive it.
			// Closing the body is what releases it.
			wrapped, ok := resp.Body.(*cancelOnCloseBody)
			if !ok {
				t.Fatalf("body is %T, want it wrapped so Close releases the request context", resp.Body)
			}
			released := false
			inner := wrapped.cancel
			wrapped.cancel = func() { released = true; inner() }
			if err := resp.Body.Close(); err != nil {
				t.Fatalf("close failed: %v", err)
			}
			if !released {
				t.Fatal("closing the body did not release the request context")
			}
			if ctx.Err() != nil {
				t.Fatalf("closing the body cancelled the caller's context: %v", ctx.Err())
			}
		})
	}
}

// When the headers do not arrive in time the request is stopped, and the
// caller's context is left alone so it can still report the error.
func TestDoRequestWithHeaderTimeout_StopsRequestOnTimeout(t *testing.T) {
	serverSawCancel := make(chan struct{})
	release := make(chan struct{})
	srv := httptest.NewServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		select {
		case <-r.Context().Done():
			close(serverSawCancel)
		case <-release:
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := fhttp.NewRequestWithContext(ctx, "GET", srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := doRequestWithHeaderTimeout(ctx, cancel, fhttp.Client{}, req, 50*time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if resp != nil {
		t.Fatal("got a response with a timeout error")
	}
	if ctx.Err() != nil {
		t.Fatalf("caller context was cancelled on timeout: %v", ctx.Err())
	}

	select {
	case <-serverSawCancel:
	case <-time.After(5 * time.Second):
		t.Fatal("the request was still running at the server 5s after the header timeout")
	}
}

// Both dispatchers must deliver a body that arrives after the headers, and
// the end frame, when the request has an ordinary positive timeout. Before the
// fix they sent the response frame and then nothing: the Node client got its
// headers and waited for a body for ever.
func TestDispatchersStreamBodyWithHeaderTimeout(t *testing.T) {
	srv := delayedBodyServer(t, delayedBody, 150*time.Millisecond)

	for _, path := range dispatchPaths {
		for _, timeout := range []int{0, 5000} {
			t.Run(path.name+"/timeout_"+strconv.Itoa(timeout), func(t *testing.T) {
				chanWrite := make(chan []byte, 64)
				path.start(cycleTLSRequest{
					RequestID: "header-timeout-test",
					Options: Options{
						URL:     srv.URL,
						Method:  "GET",
						Timeout: timeout,
					},
				}, chanWrite)

				var frames [][]byte
				deadline := time.After(10 * time.Second)
			collect:
				for {
					select {
					case f, ok := <-chanWrite:
						if !ok {
							break collect
						}
						frames = append(frames, f)
					case <-deadline:
						t.Fatalf("%s dispatcher did not return", path.name)
					}
				}

				got := parseDispatcherFrames(t, frames)
				if len(got.errors) > 0 {
					t.Fatalf("error frames: %q", got.errors)
				}
				if got.status != 200 {
					t.Fatalf("status = %d, want 200", got.status)
				}
				if string(got.body) != delayedBody {
					t.Errorf("body = %q, want %q", got.body, delayedBody)
				}
				if !got.ended {
					t.Error("no end frame")
				}
			})
		}
	}
}
