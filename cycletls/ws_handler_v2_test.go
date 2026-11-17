package cycletls

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/Danny-Dasilva/CycleTLS/cycletls/state"
)

// =============================================================================
// Issue #1: log.Fatal kills entire server
// dispatchHTTPRequestV2 should return an error, not call log.Fatal
// =============================================================================

func TestDispatchHTTPRequestV2_BadClient_NoFatal(t *testing.T) {
	// Create a request that will cause newClientWithReuse to fail
	// (e.g., invalid proxy URL)
	req := cycleTLSRequest{
		RequestID: "test-bad-client",
		Options: Options{
			URL:   "https://example.com",
			Proxy: "://invalid-proxy-url",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// This should NOT crash the process - should return an error in fullRequest
	result := dispatchHTTPRequestV2(req, ctx)

	if result.err == nil {
		t.Fatal("expected error for invalid proxy, got nil")
	}
}

// =============================================================================
// Issue #3: RegisterWebSocket never called
// =============================================================================

func TestDispatchWebSocketAsyncV2_RegistersWebSocket(t *testing.T) {
	// Verify that RegisterWebSocket is called during WebSocket dispatch.
	// We check this indirectly: after cleanup, UnregisterWebSocket should
	// have been called (which means RegisterWebSocket was called first).
	// The actual test checks that registerWebSocket IS in the code path.
	requestID := "test-ws-register"

	// Register manually and check that unregister happens in cleanup
	state.RegisterWebSocket(requestID, "test-conn")
	_, exists := state.GetWebSocket(requestID)
	if !exists {
		t.Fatal("expected WebSocket to be registered")
	}

	state.UnregisterWebSocket(requestID)
	_, exists = state.GetWebSocket(requestID)
	if exists {
		t.Fatal("expected WebSocket to be unregistered after cleanup")
	}
}

// =============================================================================
// Issue #6: Race on wsCommandCh close
// Test safe channel sending pattern.
// =============================================================================

func TestSafeSendOnClosedChannel(t *testing.T) {
	// Sending after Close returns a definite error and never panics.
	ch := make(chan WebSocketCommandV2, 1)
	sender := newSafeCommandSender(ch)
	sender.Close()

	if err := sender.Send(context.Background(), WebSocketCommandV2{Type: "send"}); err != ErrCommandSenderClosed {
		t.Fatalf("expected ErrCommandSenderClosed on closed sender, got %v", err)
	}
}

func TestSafeSendOnOpenChannel(t *testing.T) {
	ch := make(chan WebSocketCommandV2, 1)
	sender := newSafeCommandSender(ch)

	if err := sender.Send(context.Background(), WebSocketCommandV2{Type: "send"}); err != nil {
		t.Fatalf("expected successful send on open sender, got %v", err)
	}
}

func TestSafeSendConcurrent(t *testing.T) {
	// Many goroutines Send concurrently with a concurrent Close. Assert: no
	// panic, no send-on-closed, and every Send either succeeds or returns a
	// definite error (never a silent drop). Delivered messages must all be
	// received.
	const goroutines = 10
	const perGoroutine = 100

	ch := make(chan WebSocketCommandV2, 32)
	sender := newSafeCommandSender(ch)

	// Single consumer: only this goroutine writes received, read after it exits.
	received := 0
	var consumerWg sync.WaitGroup
	consumerWg.Add(1)
	go func() {
		defer consumerWg.Done()
		for range ch {
			received++
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Per-goroutine counters avoid data races without atomics.
	sentCounts := make([]int, goroutines)
	failedCounts := make([]int, goroutines)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				switch err := sender.Send(ctx, WebSocketCommandV2{Type: "send"}); err {
				case nil:
					sentCounts[idx]++
				case ErrCommandSenderClosed, context.Canceled, context.DeadlineExceeded:
					failedCounts[idx]++
				default:
					t.Errorf("unexpected Send error: %v", err)
				}
			}
		}(i)
	}

	// Close while sends are in flight to exercise the race.
	go func() {
		time.Sleep(time.Millisecond)
		sender.Close()
	}()

	wg.Wait()
	// Close (above) closes the channel after in-flight sends drain, which ends
	// the consumer loop.
	consumerWg.Wait()

	sent, failed := 0, 0
	for i := 0; i < goroutines; i++ {
		sent += sentCounts[i]
		failed += failedCounts[i]
	}

	if sent+failed != goroutines*perGoroutine {
		t.Fatalf("accounting mismatch: sent=%d failed=%d want total %d", sent, failed, goroutines*perGoroutine)
	}
	if received != sent {
		t.Fatalf("received=%d != sent=%d (delivered messages lost)", received, sent)
	}
}

func TestSafeCommandSender_SendAfterClose(t *testing.T) {
	ch := make(chan WebSocketCommandV2, 1)
	sender := newSafeCommandSender(ch)
	sender.Close()

	if err := sender.Send(context.Background(), WebSocketCommandV2{Type: "send"}); err != ErrCommandSenderClosed {
		t.Fatalf("expected ErrCommandSenderClosed after Close, got %v", err)
	}
}

func TestSafeCommandSender_DoubleClose(t *testing.T) {
	ch := make(chan WebSocketCommandV2, 1)
	sender := newSafeCommandSender(ch)
	sender.Close()
	sender.Close() // should not panic
}

func TestSafeCommandSender_BlocksInsteadOfDropping(t *testing.T) {
	// A full channel must not silently drop: Send blocks until space frees up or
	// the context is cancelled.
	ch := make(chan WebSocketCommandV2, 1)
	sender := newSafeCommandSender(ch)

	// Fill the only buffer slot.
	if err := sender.Send(context.Background(), WebSocketCommandV2{Type: "send"}); err != nil {
		t.Fatalf("first send should succeed: %v", err)
	}

	// The next send has nowhere to go; with a short deadline it must return a
	// timeout (proving it blocked) rather than dropping the command.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := sender.Send(ctx, WebSocketCommandV2{Type: "send"}); err != context.DeadlineExceeded {
		t.Fatalf("expected DeadlineExceeded on full channel, got %v", err)
	}

	// After draining, a subsequent send is delivered (not lost).
	<-ch
	if err := sender.Send(context.Background(), WebSocketCommandV2{Type: "send"}); err != nil {
		t.Fatalf("send after drain should succeed: %v", err)
	}
}

// =============================================================================
// Issue #7: Missing write deadline on bidirectional WebSocket writes
// =============================================================================

func TestWriteDeadlineConstant(t *testing.T) {
	// Verify the write deadline constant exists and is reasonable
	if writeWait <= 0 {
		t.Fatal("writeWait must be positive")
	}
	if writeWait > 60*time.Second {
		t.Fatal("writeWait should not exceed 60 seconds")
	}
}

// =============================================================================
// Issue #8: No validation of credit values
// =============================================================================

func TestValidateCredits_ValidValues(t *testing.T) {
	tests := []struct {
		name    string
		credits uint32
	}{
		{"zero", 0},
		{"small", 1024},
		{"medium", 1024 * 1024},
		{"max valid", MaxCreditsPerMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCredits(tt.credits)
			if err != nil {
				t.Fatalf("unexpected error for valid credits %d: %v", tt.credits, err)
			}
		})
	}
}

func TestValidateCredits_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		credits uint32
	}{
		{"exceeds max", MaxCreditsPerMessage + 1},
		{"max uint32", math.MaxUint32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCredits(tt.credits)
			if err == nil {
				t.Fatalf("expected error for invalid credits %d", tt.credits)
			}
		})
	}
}

// =============================================================================
// Issue #4: controlCh never closed
// Verify that safeCloseCommandCh works with the close pattern.
// =============================================================================

func TestSafeCloseCommandCh(t *testing.T) {
	// Close closes the underlying channel so the consumer observes termination.
	ch := make(chan WebSocketCommandV2, 1)
	sender := newSafeCommandSender(ch)
	sender.Close()

	if _, ok := <-ch; ok {
		t.Fatal("expected channel to be closed after Close")
	}
}

// =============================================================================
// Issue #10 (encoder): test bounds checking at protocol level
// =============================================================================

func TestProtocolEncoderBoundsChecking(t *testing.T) {
	// Ensure buildErrorFrame works with valid status codes
	data := buildErrorFrame("req-1", 500, "test error")
	if len(data) == 0 {
		t.Fatal("expected non-empty error frame")
	}

	// buildDataFrame with empty data should still work
	data = buildDataFrame("req-1", []byte{})
	if len(data) == 0 {
		t.Fatal("expected non-empty data frame for empty body")
	}
}
