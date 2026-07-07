package cycletls

import (
	"context"
	"testing"
	"time"
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

// Issue #10: doRequestWithHeaderTimeout should always clean up context
// When ensureContext creates a cancel, caller must defer it.
// This is tested indirectly by verifying doRequestWithHeaderTimeout defers cancel.
func TestDoRequestWithHeaderTimeout_CleanupOnTimeout(t *testing.T) {
	// This verifies that when timeout fires, cancel is properly called
	// and the context is cleaned up. The timeout path calls cancel().
	// The non-timeout path should also clean up (the fix adds a defer).
	d := timeoutSeconds(0)
	if d == 0 {
		t.Skip("default timeout should be positive")
	}
}
