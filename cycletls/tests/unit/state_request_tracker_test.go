//go:build !integration

package unit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Danny-Dasilva/CycleTLS/cycletls/state"
)

// Note: These tests use only the public API of the state package.
// Internal state verification is done through observable behavior.

func TestRegisterAndCancelRequest(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	requestID := "test-register-cancel-1"

	state.RegisterRequest(requestID, cancel)

	// Verify by cancelling - should return true and cancel the context
	result := state.CancelRequest(requestID)

	if !result {
		t.Error("CancelRequest should return true for existing request")
	}

	// Verify cancel function was called
	select {
	case <-ctx.Done():
		// Expected - context was cancelled
	default:
		t.Error("Cancel function was not called")
	}
}

func TestCancelRequestNotExists(t *testing.T) {
	t.Parallel()

	result := state.CancelRequest("non-existent-request-id")

	if result {
		t.Error("CancelRequest should return false for non-existent request")
	}
}

func TestUnregisterRequest(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requestID := "test-unregister-1"

	state.RegisterRequest(requestID, cancel)

	// Unregister
	state.UnregisterRequest(requestID)

	// Now cancel should return false (request was unregistered)
	result := state.CancelRequest(requestID)

	if result {
		t.Error("CancelRequest should return false after UnregisterRequest")
	}

	// Context should NOT be cancelled (we unregistered, not cancelled)
	select {
	case <-ctx.Done():
		t.Error("Context should not be cancelled after UnregisterRequest")
	default:
		// Expected
	}
}

func TestUnregisterNonExistentRequest(t *testing.T) {
	t.Parallel()

	// Should not panic when unregistering a non-existent request
	state.UnregisterRequest("non-existent-unregister-id")
}

func TestMultipleRequests(t *testing.T) {
	t.Parallel()

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	ctx3, cancel3 := context.WithCancel(context.Background())
	defer cancel1()
	defer cancel2()
	defer cancel3()

	// Register multiple requests
	state.RegisterRequest("multi-req-1", cancel1)
	state.RegisterRequest("multi-req-2", cancel2)
	state.RegisterRequest("multi-req-3", cancel3)

	// Cancel only req-2
	if !state.CancelRequest("multi-req-2") {
		t.Error("Failed to cancel multi-req-2")
	}

	// Verify ctx2 is cancelled, others are not
	select {
	case <-ctx2.Done():
		// Expected
	default:
		t.Error("ctx2 should be cancelled")
	}

	select {
	case <-ctx1.Done():
		t.Error("ctx1 should not be cancelled")
	default:
		// Expected
	}

	select {
	case <-ctx3.Done():
		t.Error("ctx3 should not be cancelled")
	default:
		// Expected
	}

	// Cleanup remaining
	state.UnregisterRequest("multi-req-1")
	state.UnregisterRequest("multi-req-3")
}

func TestRequestTrackerConcurrentAccess(t *testing.T) {
	t.Parallel()

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	var cancelCount int64

	for i := 0; i < numGoroutines; i++ {
		id := "concurrent-req-" + string(rune('A'+i%26)) + string(rune('0'+i/26))

		// Goroutine to register
		go func(reqID string) {
			defer wg.Done()
			_, cancel := context.WithCancel(context.Background())
			state.RegisterRequest(reqID, cancel)
		}(id)

		// Goroutine to cancel (may or may not find the request)
		go func(reqID string) {
			defer wg.Done()
			if state.CancelRequest(reqID) {
				atomic.AddInt64(&cancelCount, 1)
			}
		}(id)

		// Goroutine to unregister (may or may not find the request)
		go func(reqID string) {
			defer wg.Done()
			state.UnregisterRequest(reqID)
		}(id)
	}

	wg.Wait()
	t.Logf("Concurrent test completed: %d successful cancellations", cancelCount)
}

func TestDoubleRegistration(t *testing.T) {
	t.Parallel()

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel1()
	defer cancel2()

	requestID := "double-reg-test"

	// Register first cancel function
	state.RegisterRequest(requestID, cancel1)

	// Register second cancel function with same ID (overwrites)
	state.RegisterRequest(requestID, cancel2)

	// Cancel the request - should cancel ctx2, not ctx1
	result := state.CancelRequest(requestID)

	if !result {
		t.Error("CancelRequest should return true")
	}

	// Verify ctx2 is cancelled (the overwritten one)
	select {
	case <-ctx2.Done():
		// Expected - second cancel func was called
	default:
		t.Error("Second context should be cancelled")
	}

	// ctx1 should NOT be cancelled (it was overwritten)
	select {
	case <-ctx1.Done():
		t.Error("First context should NOT be cancelled - it was overwritten")
	default:
		// Expected
	}
}

func TestRegisterUnregisterCycle(t *testing.T) {
	t.Parallel()

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel1()
	defer cancel2()

	requestID := "cycle-test-id"

	// First registration
	state.RegisterRequest(requestID, cancel1)
	state.UnregisterRequest(requestID)

	// Second registration with same ID
	state.RegisterRequest(requestID, cancel2)

	// Cancel should affect ctx2
	result := state.CancelRequest(requestID)

	if !result {
		t.Error("CancelRequest should return true for re-registered ID")
	}

	select {
	case <-ctx2.Done():
		// Expected
	default:
		t.Error("Second context should be cancelled")
	}

	select {
	case <-ctx1.Done():
		t.Error("First context should not be cancelled")
	default:
		// Expected
	}
}

// Issue #12 (MAJOR): Thread-safe concurrent access patterns.
// Tests rapid register-cancel-unregister cycles on the SAME key from multiple goroutines.
func TestRequestTrackerConcurrentSameKey(t *testing.T) {
	t.Parallel()

	const numGoroutines = 50
	const opsPerGoroutine = 100
	requestID := "same-key-concurrent"

	var wg sync.WaitGroup
	var panicked atomic.Int64

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicked.Add(1)
				}
			}()
			for j := 0; j < opsPerGoroutine; j++ {
				_, cancel := context.WithCancel(context.Background())
				state.RegisterRequest(requestID, cancel)
				state.CancelRequest(requestID)
				state.UnregisterRequest(requestID)
			}
		}()
	}

	wg.Wait()

	if p := panicked.Load(); p > 0 {
		t.Errorf("Concurrent same-key operations panicked %d times", p)
	}
}

// Issue #12 (MAJOR): Interleaved register+cancel vs unregister on overlapping keys.
func TestRequestTrackerInterleavedOperations(t *testing.T) {
	t.Parallel()

	const numKeys = 20
	const numGoroutines = 10
	const opsPerGoroutine = 50

	var wg sync.WaitGroup
	var panicked atomic.Int64

	wg.Add(numGoroutines * 3)

	// Registerers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicked.Add(1)
				}
			}()
			for j := 0; j < opsPerGoroutine; j++ {
				key := "interleaved-" + string(rune('0'+j%numKeys))
				_, cancel := context.WithCancel(context.Background())
				state.RegisterRequest(key, cancel)
			}
		}(i)
	}

	// Cancellers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicked.Add(1)
				}
			}()
			for j := 0; j < opsPerGoroutine; j++ {
				key := "interleaved-" + string(rune('0'+j%numKeys))
				state.CancelRequest(key)
			}
		}(i)
	}

	// Unregisterers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicked.Add(1)
				}
			}()
			for j := 0; j < opsPerGoroutine; j++ {
				key := "interleaved-" + string(rune('0'+j%numKeys))
				state.UnregisterRequest(key)
			}
		}(i)
	}

	wg.Wait()

	if p := panicked.Load(); p > 0 {
		t.Errorf("Interleaved operations panicked %d times", p)
	}
}
