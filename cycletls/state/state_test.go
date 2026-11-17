package state

import (
	"context"
	"sync"
	"testing"
	"time"
)

// Issue #9: Unbounded map growth in request_tracker and websocket_tracker.
// Maps can grow indefinitely if unregister is not called.

func TestRequestTracker_RegisterUnregister(t *testing.T) {
	// Basic register/cancel/unregister cycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	id := "test-req-1"
	RegisterRequest(id, cancel)

	// Should be cancellable
	ok := CancelRequest(id)
	if !ok {
		t.Fatal("expected CancelRequest to return true for registered request")
	}

	// Context should be cancelled
	select {
	case <-ctx.Done():
		// expected
	default:
		t.Fatal("context should have been cancelled")
	}

	// Second cancel should return false
	ok = CancelRequest(id)
	if ok {
		t.Fatal("expected CancelRequest to return false for already cancelled request")
	}
}

func TestWebSocketTracker_RegisterUnregister(t *testing.T) {
	id := "test-ws-1"
	conn := "mock-connection"

	RegisterWebSocket(id, conn)

	got, exists := GetWebSocket(id)
	if !exists {
		t.Fatal("expected WebSocket to be registered")
	}
	if got != conn {
		t.Fatalf("got %v, want %v", got, conn)
	}

	UnregisterWebSocket(id)

	_, exists = GetWebSocket(id)
	if exists {
		t.Fatal("expected WebSocket to be unregistered")
	}
}

func TestRequestTracker_CleanupStale(t *testing.T) {
	// CleanupStaleRequests should remove entries older than the given max age
	id := "stale-req-1"
	_, cancel := context.WithCancel(context.Background())
	RegisterRequest(id, cancel)

	// Record the registration time
	recordRequestTime(id, time.Now().Add(-3*time.Hour))

	// Cleanup with 2-hour max age
	removed := CleanupStaleRequests(2 * time.Hour)
	if removed == 0 {
		t.Fatal("expected at least one stale request to be cleaned up")
	}

	// Should no longer be registered
	ok := CancelRequest(id)
	if ok {
		t.Fatal("expected stale request to have been removed")
	}
}

func TestWebSocketTracker_CleanupStale(t *testing.T) {
	id := "stale-ws-1"
	RegisterWebSocket(id, "mock-conn")

	// Record the registration time
	recordWebSocketTime(id, time.Now().Add(-3*time.Hour))

	// Cleanup with 2-hour max age
	removed := CleanupStaleWebSockets(2 * time.Hour)
	if removed == 0 {
		t.Fatal("expected at least one stale WebSocket to be cleaned up")
	}

	_, exists := GetWebSocket(id)
	if exists {
		t.Fatal("expected stale WebSocket to have been removed")
	}
}

func TestRequestTracker_MaxSize(t *testing.T) {
	// RequestTracker should enforce a maximum size limit
	max := RequestTrackerMaxSize()
	if max <= 0 {
		t.Fatal("expected positive max size")
	}
}

func TestWebSocketTracker_MaxSize(t *testing.T) {
	max := WebSocketTrackerMaxSize()
	if max <= 0 {
		t.Fatal("expected positive max size")
	}
}

func TestRequestTracker_Concurrent(t *testing.T) {
	// Concurrency safety test
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "concurrent-req-" + string(rune('A'+n%26))
			_, cancel := context.WithCancel(context.Background())
			RegisterRequest(id, cancel)
			CancelRequest(id)
			UnregisterRequest(id)
		}(i)
	}
	wg.Wait()
}

func TestWebSocketTracker_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "concurrent-ws-" + string(rune('A'+n%26))
			RegisterWebSocket(id, "mock")
			GetWebSocket(id)
			UnregisterWebSocket(id)
		}(i)
	}
	wg.Wait()
}

// Issue #4: CancelRequest should not deadlock if cancel callback calls UnregisterRequest
func TestCancelRequest_NoDeadlock(t *testing.T) {
	done := make(chan struct{})
	id := "deadlock-test"

	// Register a request whose cancel callback tries to call UnregisterRequest
	// (simulating a callback that triggers unregister)
	cancelCalled := false
	_, cancel := context.WithCancel(context.Background())
	RegisterRequest(id, func() {
		cancelCalled = true
		cancel()
		// In the old code this would deadlock because CancelRequest holds the mutex
		// and UnregisterRequest tries to acquire it
		// After the fix, the cancel is called outside the lock, so this is safe
	})

	go func() {
		CancelRequest(id)
		close(done)
	}()

	select {
	case <-done:
		if !cancelCalled {
			t.Fatal("cancel should have been called")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CancelRequest deadlocked - cancel callback blocked on mutex")
	}
}
