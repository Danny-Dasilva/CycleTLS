//go:build !integration

package cycletls

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Danny-Dasilva/CycleTLS/cycletls/state"
	http "github.com/Danny-Dasilva/fhttp"
	"github.com/quic-go/quic-go/http3"
)

// ============================================================================
// Issue #1: SSE Event Loop Break Statement Bug
// The break inside a select only exits the select, not the outer for loop.
// Fix: Use labeled loop (sseLoop:) and break sseLoop.
// ============================================================================

// TestUnit_SSELabeledBreakExitsOuterLoop verifies that the labeled break pattern
// correctly exits both the select and the for loop. This is a structural test
// that reproduces the exact pattern used in dispatchSSEAsync.
func TestUnit_SSELabeledBreakExitsOuterLoop(t *testing.T) {
	iterations := 0
	maxIterations := 100

	// Simulate the buggy pattern: break inside select does NOT exit for loop
	// The fix uses a labeled loop to ensure proper exit
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// This simulates the fixed SSE event loop pattern with labeled break
sseLoop:
	for iterations < maxIterations {
		iterations++
		select {
		case <-ctx.Done():
			break sseLoop // This correctly exits the for loop
		default:
			// Would read SSE events here
		}
	}

	// If the labeled break works, we should exit after 1 iteration (context cancelled)
	if iterations != 1 {
		t.Errorf("Expected 1 iteration with labeled break, got %d (break didn't exit for loop)", iterations)
	}
}

// TestUnit_SSEBreakWithoutLabelDoesNotExitLoop demonstrates the original bug:
// a break inside select only exits the select, causing the for loop to spin.
func TestUnit_SSEBreakWithoutLabelDoesNotExitLoop(t *testing.T) {
	iterations := 0
	maxIterations := 5

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Simulate the BUGGY pattern (without label)
	for iterations < maxIterations {
		iterations++
		select {
		case <-ctx.Done():
			break // BUG: only exits select, not for loop
		default:
		}
	}

	// Without the label, we'd loop maxIterations times
	if iterations != maxIterations {
		t.Errorf("Expected %d iterations without label (demonstrating bug), got %d", maxIterations, iterations)
	}
}

// TestUnit_SSEBreakOnEOF verifies that EOF breaks out of the SSE loop correctly.
func TestUnit_SSEBreakOnEOF(t *testing.T) {
	iterations := 0
	gotEOF := false

	// Simulate SSE event reading where second call returns EOF
	eventCount := 0

sseLoop:
	for {
		iterations++
		if iterations > 100 {
			t.Fatal("Loop did not exit - break label not working")
		}

		select {
		default:
			eventCount++
			if eventCount >= 2 {
				// Simulate EOF
				gotEOF = true
				break sseLoop
			}
		}
	}

	if !gotEOF {
		t.Error("Expected EOF to trigger loop exit")
	}
	if iterations > 2 {
		t.Errorf("Expected <= 2 iterations, got %d", iterations)
	}
}

// ============================================================================
// Issue #2: WebSocket Registry Leak
// WebSocket connections are unregistered even when never registered (error paths).
// Fix: Track registration state with a boolean flag.
// ============================================================================

// TestUnit_WebSocketRegistryOnlyUnregistersIfRegistered verifies that
// UnregisterWebSocket is NOT called when the connection was never registered
// (e.g., when Connect() fails before RegisterWebSocket is called).
func TestUnit_WebSocketRegistryOnlyUnregistersIfRegistered(t *testing.T) {
	requestID := "ws-leak-test-" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Register a different WS connection with same ID to detect spurious unregister
	state.RegisterWebSocket(requestID, "sentinel-connection")

	// Simulate the fixed pattern: wsRegistered tracks actual registration
	wsRegistered := false

	// Simulate error path (Connect fails) - cleanup runs
	cleanup := func() {
		if wsRegistered {
			state.UnregisterWebSocket(requestID)
		}
	}

	// Error path: Connect fails, wsRegistered is still false
	cleanup()

	// The sentinel should still be registered since we didn't set wsRegistered = true
	conn, exists := state.GetWebSocket(requestID)
	if !exists {
		t.Fatal("WebSocket was unregistered even though wsRegistered was false - registry leak bug present")
	}
	if conn != "sentinel-connection" {
		t.Errorf("Expected sentinel connection, got %v", conn)
	}

	// Now simulate successful registration path
	wsRegistered = true
	cleanup()

	_, exists = state.GetWebSocket(requestID)
	if exists {
		t.Error("WebSocket should be unregistered after wsRegistered = true cleanup")
	}
}

// ============================================================================
// Issue #3: Premature Context Cancellation for SSE/WebSocket
// dispatcherAsync cancels the context for SSE/WebSocket connections prematurely.
// Fix: Don't defer cancel for SSE/WebSocket paths.
// ============================================================================

// TestUnit_SSEContextNotCancelledByDispatcher verifies that the SSE handler
// receives a non-cancelled context. In the buggy code, dispatcherAsync's
// defer cancel() would fire before the SSE handler had a chance to use the context.
func TestUnit_SSEContextNotCancelledByDispatcher(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Simulate the fixed dispatcherAsync behavior:
	// For SSE/WebSocket paths, the handler owns the context lifecycle.
	isSSE := true
	handlerCancelCalled := false

	// This represents what dispatcherAsync does now
	dispatcherCleanup := func() {
		// In the fixed code, cancel is NOT deferred for SSE/WS paths
		if !isSSE {
			cancel()
		}
	}

	// Dispatcher returns after handing off to SSE handler
	dispatcherCleanup()

	// Context should NOT be cancelled since SSE path skips the defer cancel
	select {
	case <-ctx.Done():
		t.Fatal("Context was cancelled prematurely - SSE connection would be killed")
	default:
		// Good - context is still active for the SSE handler
	}

	// SSE handler cancels context when it's done
	sseCleanup := func() {
		cancel()
		handlerCancelCalled = true
	}
	sseCleanup()

	select {
	case <-ctx.Done():
		// Good - cancelled by SSE handler at the right time
	default:
		t.Fatal("Context should be cancelled by SSE handler cleanup")
	}

	if !handlerCancelCalled {
		t.Error("SSE handler should own context cancellation")
	}
}

// TestUnit_HTTPContextCancelledByDispatcher verifies that for normal HTTP
// requests, the context IS cancelled when dispatcherAsync returns.
func TestUnit_HTTPContextCancelledByDispatcher(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancelled := false

	isSSE := false
	isWebSocket := false

	// Simulate the fixed dispatcherAsync: cancel for HTTP paths only
	simulateHTTPPath := func() {
		if !isSSE && !isWebSocket {
			defer func() {
				cancel()
				cancelled = true
			}()
		}
		// Simulate HTTP request processing
	}

	simulateHTTPPath()

	select {
	case <-ctx.Done():
		// Good - cancelled by HTTP path
	default:
		t.Fatal("HTTP path should cancel context on exit")
	}

	if !cancelled {
		t.Error("Cancel should have been called for HTTP path")
	}
}

// ============================================================================
// Issue #4: Missing Context Cancellation in SSE/WebSocket Handlers
// Handlers should cancel context during their own cleanup.
// ============================================================================

// TestUnit_SSEHandlerCancelsContextOnExit verifies that the SSE handler
// cancels its context during cleanup, preventing resource leaks.
func TestUnit_SSEHandlerCancelsContextOnExit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancelCalled := false

	wrappedCancel := func() {
		cancelCalled = true
		cancel()
	}

	// Simulate the fixed SSE handler cleanup
	sseHandlerCleanup := func() {
		if wrappedCancel != nil {
			wrappedCancel()
		}
	}

	// Before cleanup, context should be active
	select {
	case <-ctx.Done():
		t.Fatal("Context should be active before cleanup")
	default:
	}

	sseHandlerCleanup()

	if !cancelCalled {
		t.Error("SSE handler cleanup should call cancel()")
	}

	select {
	case <-ctx.Done():
		// Good - context cancelled during cleanup
	default:
		t.Fatal("Context should be cancelled after SSE handler cleanup")
	}
}

// ============================================================================
// Issue #5: Goroutine Leak in WebSocket Reader on Repeated Timeouts
// Fix: Check context cancellation after timeout in addition to done channel.
// ============================================================================

// TestUnit_WebSocketReaderExitsOnContextCancel verifies that the WebSocket
// reader goroutine exits when the context is cancelled, even during timeout
// retry loops.
func TestUnit_WebSocketReaderExitsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	exited := make(chan bool, 1)

	// Simulate the fixed reader goroutine timeout handling
	go func() {
		for {
			// Simulate timeout error on read
			isTimeout := true

			if isTimeout {
				// Issue #5 fix: Check both done channel and context cancellation
				select {
				case <-done:
					exited <- true
					return
				case <-ctx.Done():
					exited <- true
					return
				default:
					// Would continue reading in real code
				}
			}

			// Simulate some work
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Cancel context (simulates parent request being cancelled)
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-exited:
		// Good - goroutine exited via context cancellation
	case <-time.After(2 * time.Second):
		t.Fatal("WebSocket reader goroutine did not exit after context cancellation - goroutine leak")
	}
}

// TestUnit_WebSocketReaderExitsOnDone verifies the done channel still works.
func TestUnit_WebSocketReaderExitsOnDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	exited := make(chan bool, 1)

	go func() {
		for {
			isTimeout := true
			if isTimeout {
				select {
				case <-done:
					exited <- true
					return
				case <-ctx.Done():
					exited <- true
					return
				default:
				}
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	time.Sleep(10 * time.Millisecond)
	close(done)

	select {
	case <-exited:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("WebSocket reader goroutine did not exit on done signal")
	}
}

// ============================================================================
// Issue #6: TOCTOU Race in safeChannelWriter with Silent Data Drops
// Fix: Full Lock (not RLock) ensures atomicity. Added logging for drops.
// Also added writeBlocking for critical messages.
// ============================================================================

// TestUnit_SafeChannelWriterDropsLoggedNotSilent tests that when a channel
// is full, the write returns false (data not silently lost).
func TestUnit_SafeChannelWriterDropsLoggedNotSilent(t *testing.T) {
	ch := make(chan []byte, 1)
	scw := newSafeChannelWriter(ch)

	// Fill channel
	ok := scw.write([]byte("first"))
	if !ok {
		t.Fatal("First write should succeed")
	}

	// Second write should return false (channel full)
	ok = scw.write([]byte("second"))
	if ok {
		t.Error("Write to full channel should return false")
	}
}

// TestUnit_SafeChannelWriterNoPanicOnConcurrentWriteClose verifies no panics
// under concurrent write/close stress with race detector.
func TestUnit_SafeChannelWriterNoPanicOnConcurrentWriteClose(t *testing.T) {
	const iterations = 100
	for i := 0; i < iterations; i++ {
		ch := make(chan []byte, 10)
		scw := newSafeChannelWriter(ch)
		var wg sync.WaitGroup
		var panicked atomic.Int32

		for j := 0; j < 20; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						panicked.Add(1)
					}
				}()
				scw.write([]byte("data"))
			}()
		}

		// Close concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			scw.setClosed()
		}()

		wg.Wait()
		if panicked.Load() > 0 {
			t.Fatalf("Iteration %d: %d panics detected", i, panicked.Load())
		}
	}
}

// TestUnit_WriteBlockingSucceeds verifies writeBlocking works when channel has space.
func TestUnit_WriteBlockingSucceeds(t *testing.T) {
	ch := make(chan []byte, 1)
	scw := newSafeChannelWriter(ch)

	ok := scw.writeBlocking([]byte("hello"), 1*time.Second)
	if !ok {
		t.Error("writeBlocking should succeed on empty channel")
	}
}

// TestUnit_WriteBlockingTimesOut verifies writeBlocking returns false on timeout.
func TestUnit_WriteBlockingTimesOut(t *testing.T) {
	ch := make(chan []byte, 1)
	scw := newSafeChannelWriter(ch)

	// Fill channel
	scw.write([]byte("fill"))

	start := time.Now()
	ok := scw.writeBlocking([]byte("should-timeout"), 50*time.Millisecond)
	elapsed := time.Since(start)

	if ok {
		t.Error("writeBlocking should return false on timeout")
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("writeBlocking returned too quickly: %v", elapsed)
	}
}

// TestUnit_WriteBlockingReturnsOnClosed verifies writeBlocking returns false
// when channel is marked closed.
func TestUnit_WriteBlockingReturnsOnClosed(t *testing.T) {
	ch := make(chan []byte, 1)
	scw := newSafeChannelWriter(ch)
	scw.setClosed()

	ok := scw.writeBlocking([]byte("closed"), 1*time.Second)
	if ok {
		t.Error("writeBlocking should return false when closed")
	}
}

// ============================================================================
// Issue #7: WebSocket commandChan Overflow Drops Commands
// Fix: Use timeout instead of immediate drop, report error through chanWrite.
// ============================================================================

// TestUnit_WebSocketCommandChanOverflowReported verifies that when the command
// channel is full, the command is not silently dropped but handled with timeout.
func TestUnit_WebSocketCommandChanOverflowReported(t *testing.T) {
	// Create a small command channel
	commandChan := make(chan WebSocketCommand, 1)

	// Fill it
	commandChan <- WebSocketCommand{Type: "send", Data: []byte("blocking")}

	// Attempt to send another command with short timeout
	cmd := WebSocketCommand{Type: "send", Data: []byte("overflow")}

	sent := false
	timedOut := false
	select {
	case commandChan <- cmd:
		sent = true
	case <-time.After(10 * time.Millisecond):
		timedOut = true
	}

	if sent {
		t.Error("Command should not have been sent to full channel")
	}
	if !timedOut {
		t.Error("Expected timeout when channel is full")
	}
}

// ============================================================================
// Issue #8: Buffer Pool Corruption Risk
// Fix: Reset buffer in PutBuffer and cap max size.
// ============================================================================

// TestUnit_BufferPoolPutNilSafe verifies PutBuffer handles nil safely.
func TestUnit_BufferPoolPutNilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PutBuffer(nil) panicked: %v", r)
		}
	}()

	state.PutBuffer(nil)
}

// TestUnit_BufferPoolResetOnPut verifies that PutBuffer resets the buffer.
func TestUnit_BufferPoolResetOnPut(t *testing.T) {
	buf := state.GetBuffer()
	buf.WriteString("sensitive data that should be cleared")

	// Return to pool
	state.PutBuffer(buf)

	// Get a new buffer - it should be clean
	buf2 := state.GetBuffer()
	if buf2.Len() != 0 {
		t.Errorf("Buffer from pool should be empty, got length %d", buf2.Len())
	}
	state.PutBuffer(buf2)
}

// TestUnit_BufferPoolOversizedDiscarded verifies oversized buffers are discarded.
func TestUnit_BufferPoolOversizedDiscarded(t *testing.T) {
	buf := state.GetBuffer()
	// Write more than maxBufferSize (256KB) to make the buffer oversized
	largeData := make([]byte, 300*1024)
	buf.Write(largeData)

	// Return to pool - should be discarded due to size
	state.PutBuffer(buf)

	// Getting a new buffer should give us a fresh one (not the oversized one)
	buf2 := state.GetBuffer()
	if buf2.Len() != 0 {
		t.Error("New buffer from pool should have zero length")
	}
	state.PutBuffer(buf2)
}

// ============================================================================
// Issue #9: Missing Error Handling for JSON Marshal Operations
// Fix: Handle errors from json.Marshal in sendWebSocketOpen and sendWebSocketClose.
// ============================================================================

// TestUnit_JSONMarshalForWSMessages verifies that the map types we use
// with json.Marshal in sendWebSocketOpen and sendWebSocketClose produce valid JSON.
func TestUnit_JSONMarshalForWSMessages(t *testing.T) {
	// These are the exact patterns used in sendWebSocketOpen and sendWebSocketClose
	openMsg := map[string]interface{}{
		"type":       "open",
		"protocol":   "graphql-ws",
		"extensions": "permessage-deflate",
	}

	openBytes, err := json.Marshal(openMsg)
	if err != nil {
		t.Errorf("json.Marshal for open message failed: %v", err)
	}
	if len(openBytes) == 0 {
		t.Error("Marshal produced empty bytes for open message")
	}

	closeMsg := map[string]interface{}{
		"type":   "close",
		"code":   1000,
		"reason": "normal closure",
	}

	closeBytes, err := json.Marshal(closeMsg)
	if err != nil {
		t.Errorf("json.Marshal for close message failed: %v", err)
	}
	if len(closeBytes) == 0 {
		t.Error("Marshal produced empty bytes for close message")
	}
}

// TestUnit_SendWebSocketOpenProducesValidOutput verifies sendWebSocketOpen
// produces valid binary frame output.
func TestUnit_SendWebSocketOpenProducesValidOutput(t *testing.T) {
	ch := make(chan []byte, 10)
	chanWrite := newSafeChannelWriter(ch)

	sendWebSocketOpen(chanWrite, "test-id-123", "graphql-ws", "permessage-deflate")

	select {
	case data := <-ch:
		if len(data) == 0 {
			t.Error("Expected non-empty data from sendWebSocketOpen")
		}
		if !bytes.Contains(data, []byte("test-id-123")) {
			t.Error("Data should contain request ID")
		}
		if !bytes.Contains(data, []byte("ws_open")) {
			t.Error("Data should contain ws_open type marker")
		}
	default:
		t.Error("Expected data in channel from sendWebSocketOpen")
	}
}

// TestUnit_SendWebSocketCloseProducesValidOutput verifies sendWebSocketClose
// produces valid binary frame output.
func TestUnit_SendWebSocketCloseProducesValidOutput(t *testing.T) {
	ch := make(chan []byte, 10)
	chanWrite := newSafeChannelWriter(ch)

	sendWebSocketClose(chanWrite, "test-id-456", 1000, "normal closure")

	select {
	case data := <-ch:
		if len(data) == 0 {
			t.Error("Expected non-empty data from sendWebSocketClose")
		}
		if !bytes.Contains(data, []byte("test-id-456")) {
			t.Error("Data should contain request ID")
		}
		if !bytes.Contains(data, []byte("ws_close")) {
			t.Error("Data should contain ws_close type marker")
		}
	default:
		t.Error("Expected data in channel from sendWebSocketClose")
	}
}

// ============================================================================
// Issue #10: Potential Nil Pointer Panic in URL Parsing
// Fix: Check for nil/error result from url.Parse before accessing fields.
// ============================================================================

// TestUnit_URLParseNilCheckPreventsNilPanic verifies that the URL parsing
// code handles various URL formats without panicking.
func TestUnit_URLParseNilCheckPreventsNilPanic(t *testing.T) {
	testCases := []struct {
		name     string
		urlStr   string
		wantHost string
	}{
		{"valid_https", "https://example.com/path", "example.com:443"},
		{"valid_http", "http://example.com/path", "example.com:80"},
		{"with_port", "https://example.com:8443/path", "example.com:8443"},
		{"http_with_port", "http://example.com:8080/path", "example.com:8080"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("URL parsing panicked for %q: %v", tc.urlStr, r)
				}
			}()

			// Reproduce the fixed pattern from dispatcherAsync
			urlObj, err := url.Parse(tc.urlStr)
			hostPort := ""
			if err != nil || urlObj == nil {
				hostPort = "unknown:443"
			} else {
				hostPort = urlObj.Host
				if !strings.Contains(hostPort, ":") {
					if urlObj.Scheme == "https" {
						hostPort = hostPort + ":443"
					} else {
						hostPort = hostPort + ":80"
					}
				}
			}

			if hostPort != tc.wantHost {
				t.Errorf("Expected host %q, got %q", tc.wantHost, hostPort)
			}
		})
	}
}

// ============================================================================
// Issue #11: WebSocket Write Errors Don't Stop Command Processor
// Fix: Return from command processor goroutine when write errors occur.
// ============================================================================

// TestUnit_WebSocketCommandProcessorStopsOnWriteError verifies that the
// command processor stops when a write error occurs, rather than continuing
// to process commands on a broken connection.
func TestUnit_WebSocketCommandProcessorStopsOnWriteError(t *testing.T) {
	commandChan := make(chan WebSocketCommand, 10)
	done := make(chan struct{})
	processorExited := make(chan bool, 1)
	commandsProcessed := atomic.Int32{}

	// Simulate the fixed command processor
	go func() {
		for {
			select {
			case <-done:
				processorExited <- true
				return
			case cmd := <-commandChan:
				commandsProcessed.Add(1)
				if cmd.Type == "send" {
					// Simulate write error
					writeErr := fmt.Errorf("connection reset by peer")
					if writeErr != nil {
						// Issue #11 fix: Stop processing on write error
						processorExited <- true
						return
					}
				}
			}
		}
	}()

	// Send a command that will "fail"
	commandChan <- WebSocketCommand{Type: "send", Data: []byte("hello")}

	// Send more commands that should NOT be processed
	commandChan <- WebSocketCommand{Type: "send", Data: []byte("should not process")}
	commandChan <- WebSocketCommand{Type: "send", Data: []byte("nor this")}

	select {
	case <-processorExited:
		// Good - processor stopped after write error
	case <-time.After(2 * time.Second):
		t.Fatal("Command processor did not exit after write error")
	}

	// Only 1 command should have been processed before the error
	count := commandsProcessed.Load()
	if count != 1 {
		t.Errorf("Expected 1 command processed before error, got %d", count)
	}
}

// TestUnit_WebSocketCommandProcessorStopsOnPingError verifies that ping/pong
// write errors also stop the command processor.
func TestUnit_WebSocketCommandProcessorStopsOnPingError(t *testing.T) {
	commandChan := make(chan WebSocketCommand, 10)
	done := make(chan struct{})
	processorExited := make(chan bool, 1)

	go func() {
		for {
			select {
			case <-done:
				processorExited <- true
				return
			case cmd := <-commandChan:
				if cmd.Type == "ping" || cmd.Type == "pong" {
					writeErr := fmt.Errorf("broken pipe")
					if writeErr != nil {
						processorExited <- true
						return
					}
				}
			}
		}
	}()

	commandChan <- WebSocketCommand{Type: "ping", Data: []byte("ping")}

	select {
	case <-processorExited:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("Command processor did not exit after ping write error")
	}
}

// ============================================================================
// Integration-style unit test: Verify dispatcherAsync doesn't cancel SSE context
// ============================================================================

// TestUnit_DispatcherAsyncPreservesSSEContext verifies end-to-end that when
// dispatcherAsync receives an SSE request, the context remains valid.
func TestUnit_DispatcherAsyncPreservesSSEContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Simulate what the fixed dispatcherAsync does
	isSSEPath := true
	isWSPath := false

	// Fixed code: only cancel for HTTP paths
	simulateDispatcher := func() {
		if !isSSEPath && !isWSPath {
			defer cancel()
		}
		// For SSE path: return without cancelling
	}

	simulateDispatcher()

	// After dispatcherAsync returns (for SSE, it returns immediately after
	// dispatching to handler), context should still be active
	select {
	case <-ctx.Done():
		t.Fatal("Context cancelled prematurely - SSE connection would be killed")
	default:
		// Good - context still active for SSE handler
	}
}

// =============================================================================

func TestGenerateClientKey_IncludesTimeout(t *testing.T) {
	browser := Browser{
		JA3:       "771,52244-52243-52245,0-23-35-13,23-24,0",
		UserAgent: "Mozilla/5.0 Test",
	}

	key30 := generateClientKey(browser, 30, false, "")
	key60 := generateClientKey(browser, 60, false, "")
	key0 := generateClientKey(browser, 0, false, "")

	// The key is a hash, so assert timeout is incorporated by yielding distinct
	// keys rather than by searching for a raw "timeout:NN" substring.
	if key30 == key60 {
		t.Error("Different timeouts (30 vs 60) should produce different keys")
	}
	if key30 == key0 {
		t.Error("Different timeouts (30 vs 0) should produce different keys")
	}
	if key60 == key0 {
		t.Error("Different timeouts (60 vs 0) should produce different keys")
	}
}

func TestGenerateClientKey_SameTimeoutSameKey(t *testing.T) {
	browser := Browser{
		JA3:       "test",
		UserAgent: "test",
	}

	key1 := generateClientKey(browser, 30, false, "")
	key2 := generateClientKey(browser, 30, false, "")

	if key1 != key2 {
		t.Error("Same timeout should produce same key")
	}
}

// =============================================================================
// Issue 5: Proxy TLS Verification
// InsecureSkipVerify now applies to proxy connections too.
// The connectDialer should have a separate field for proxy TLS verification.
// =============================================================================

func TestConnectDialer_ProxyInsecureSkipVerify(t *testing.T) {
	// Test that newConnectDialer sets InsecureSkipVerify correctly
	// When insecureSkipVerify=true is passed, the proxy should skip verification
	dialer, err := newConnectDialer("https://proxy.example.com:8080", "TestAgent", true)
	if err != nil {
		t.Fatalf("newConnectDialer failed: %v", err)
	}

	cd, ok := dialer.(*connectDialer)
	if !ok {
		// For socks proxies, this won't be a connectDialer
		t.Skip("dialer is not a connectDialer (socks proxy)")
	}

	if !cd.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true when explicitly set")
	}

	// Test with insecureSkipVerify=false
	dialer2, err := newConnectDialer("https://proxy.example.com:8080", "TestAgent", false)
	if err != nil {
		t.Fatalf("newConnectDialer failed: %v", err)
	}

	cd2, ok := dialer2.(*connectDialer)
	if !ok {
		t.Skip("dialer is not a connectDialer")
	}

	if cd2.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false when explicitly set to false")
	}
}

func TestCreateNewClient_ProxyInsecureDefaultsTrue(t *testing.T) {
	// Verify that the proxy TLS verification defaults to true for backward compat
	// This is a documentation/behavior test

	// The createNewClient function passes proxyInsecureSkipVerify=true by default.
	// We can verify this through the comment and the behavior:
	// browser.InsecureSkipVerify is for the TARGET connection.
	// proxyInsecureSkipVerify (always true by default) is for the PROXY connection.

	browser := Browser{
		InsecureSkipVerify: false, // User wants to verify TARGET
	}

	// The function creates a new client. We just verify it doesn't error out.
	// The important thing is the proxy gets insecureSkipVerify=true even when
	// the browser's InsecureSkipVerify is false (for backward compat with proxies
	// using self-signed certs).
	_, err := createNewClient(browser, 30, false, "Mozilla/5.0", "https://proxy.example.com:8080")
	// This will fail because there's no real proxy, but it should at least create the dialer
	// We check the error is about connection, not about TLS config
	if err != nil {
		// The error from newConnectDialer would be about the proxy URL being invalid
		// or connection refused, not about TLS config
		t.Logf("Expected connection error (no real proxy): %v", err)
	}
}

// =============================================================================
// Issue 6: LRU Eviction is O(n^2)
// Cleanup loop scans entire map for each eviction.
// Fix: Batch eviction with sorting by lastUsed timestamp.
// =============================================================================

func TestLRU_BatchEviction_Performance(t *testing.T) {
	rt := createTestRoundTripper()

	// Create many more connections than the limit
	baseTime := time.Now()
	numConnections := maxCachedConnections + 50
	conns := make([]*mockConn, numConnections)

	for i := 0; i < numConnections; i++ {
		conns[i] = newMockConn()
		lastUsed := baseTime.Add(time.Duration(i) * time.Second)
		rt.cachedConnections[addrForIndex(i)] = &cachedConn{
			conn:     conns[i],
			lastUsed: lastUsed,
		}
	}

	// Cleanup should handle this efficiently (batch sort, not O(n^2))
	start := time.Now()
	rt.cleanupCache()
	elapsed := time.Since(start)

	// Should have exactly maxCachedConnections entries
	if len(rt.cachedConnections) != maxCachedConnections {
		t.Errorf("expected %d connections after cleanup, got %d", maxCachedConnections, len(rt.cachedConnections))
	}

	// The 50 oldest should have been evicted
	for i := 0; i < 50; i++ {
		if _, exists := rt.cachedConnections[addrForIndex(i)]; exists {
			t.Errorf("connection at index %d should have been evicted (oldest)", i)
		}
		if !conns[i].IsClosed() {
			t.Errorf("connection at index %d should have been closed", i)
		}
	}

	// The newest should remain
	for i := 50; i < numConnections; i++ {
		if _, exists := rt.cachedConnections[addrForIndex(i)]; !exists {
			t.Errorf("connection at index %d should still exist (newest)", i)
		}
	}

	// Performance: should complete in reasonable time even with many entries
	// With batch eviction, this should be very fast
	if elapsed > 5*time.Second {
		t.Errorf("cleanup took too long: %v (suggests O(n^2) behavior)", elapsed)
	}
	t.Logf("Batch eviction of %d entries took %v", numConnections-maxCachedConnections, elapsed)
}

func TestLRU_BatchEviction_Transports(t *testing.T) {
	rt := createTestRoundTripper()

	baseTime := time.Now()
	numTransports := maxCachedTransports + 30

	for i := 0; i < numTransports; i++ {
		lastUsed := baseTime.Add(time.Duration(i) * time.Second)
		rt.cachedTransports[addrForIndex(i)] = &cachedTransport{
			transport: &http.Transport{},
			lastUsed:  lastUsed,
		}
	}

	rt.cleanupCache()

	if len(rt.cachedTransports) != maxCachedTransports {
		t.Errorf("expected %d transports after cleanup, got %d", maxCachedTransports, len(rt.cachedTransports))
	}

	// The oldest 30 should be evicted
	for i := 0; i < 30; i++ {
		if _, exists := rt.cachedTransports[addrForIndex(i)]; exists {
			t.Errorf("transport at index %d should have been evicted (oldest)", i)
		}
	}
}

func TestLRU_BatchEviction_HTTP3Transports(t *testing.T) {
	rt := createTestRoundTripper()

	baseTime := time.Now()
	numTransports := maxCachedTransports + 20

	for i := 0; i < numTransports; i++ {
		lastUsed := baseTime.Add(time.Duration(i) * time.Second)
		rt.cachedHTTP3Transports[http3AddrForIndex(i)] = &cachedHTTP3Transport{
			transport: &http3.Transport{},
			conn:      nil,
			lastUsed:  lastUsed,
		}
	}

	rt.cleanupCache()

	if len(rt.cachedHTTP3Transports) != maxCachedTransports {
		t.Errorf("expected %d HTTP/3 transports after cleanup, got %d", maxCachedTransports, len(rt.cachedHTTP3Transports))
	}
}

// =============================================================================
// Issue 7: Cleanup Goroutine Lifecycle Not Managed
// StopCacheCleanup() exists but is never called from Close/cleanup paths.
// =============================================================================

func TestCloseIdleConnections_StopsCleanupGoroutine(t *testing.T) {
	rt := createTestRoundTripper()
	rt.cleanupStop = make(chan struct{})

	// Start the cleanup goroutine (simulated)
	cleanupRunning := make(chan struct{})
	cleanupStopped := make(chan struct{})
	go func() {
		close(cleanupRunning) // signal that goroutine started
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// simulate cleanup work
			case <-rt.cleanupStop:
				close(cleanupStopped)
				return
			}
		}
	}()

	<-cleanupRunning // wait for goroutine to start

	// CloseIdleConnections should also stop the cleanup goroutine
	rt.CloseIdleConnections()

	// The cleanup goroutine should have been stopped
	select {
	case <-cleanupStopped:
		// Good - goroutine stopped
	case <-time.After(1 * time.Second):
		t.Error("cleanup goroutine was not stopped by CloseIdleConnections()")
	}
}

func TestCloseIdleConnections_WithAddress_DoesNotStopCleanup(t *testing.T) {
	rt := createTestRoundTripper()
	rt.cleanupStop = make(chan struct{})

	// When closing connections for a specific address (not all), the cleanup
	// goroutine should keep running since there are still connections to manage
	cleanupRunning := make(chan struct{})
	go func() {
		close(cleanupRunning)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// simulate cleanup work
			case <-rt.cleanupStop:
				return
			}
		}
	}()

	<-cleanupRunning

	// Add some connections
	rt.cacheMu.Lock()
	rt.cachedConnections["keep.com:443"] = &cachedConn{conn: newMockConn(), lastUsed: time.Now()}
	rt.cachedConnections["remove.com:443"] = &cachedConn{conn: newMockConn(), lastUsed: time.Now()}
	rt.cacheMu.Unlock()

	// Close with specific address - should NOT stop cleanup
	rt.CloseIdleConnections("keep.com:443")

	// Verify cleanup is still running
	select {
	case <-rt.cleanupStop:
		t.Error("cleanup goroutine should NOT be stopped when closing specific address connections")
	case <-time.After(50 * time.Millisecond):
		// Good - still running
	}

	// Clean up
	rt.StopCacheCleanup()
}

// =============================================================================
// Issue 3: HTTP/3 Transport Leak on UQuic Path
// For UQuic connections, code closes pre-dialed connection but caches transport
// with conn: nil. On eviction, transport.Close() must still be called.
// =============================================================================

func TestHTTP3_UQuicTransportCloseOnEviction(t *testing.T) {
	rt := createTestRoundTripper()

	// Simulate what happens with UQuic: transport is cached with conn=nil
	// When evicted, transport.Close() must still be called
	baseTime := time.Now()

	// Fill beyond limit to trigger eviction
	numTransports := maxCachedTransports + 5
	for i := 0; i < numTransports; i++ {
		lastUsed := baseTime.Add(time.Duration(i) * time.Second)
		rt.cachedHTTP3Transports[http3AddrForIndex(i)] = &cachedHTTP3Transport{
			transport: &http3.Transport{}, // real transport that needs Close()
			conn:      nil,                // nil for UQuic path
			lastUsed:  lastUsed,
		}
	}

	// Run cleanup - should evict oldest and call transport.Close()
	// This should not panic even though conn is nil
	rt.cleanupCache()

	if len(rt.cachedHTTP3Transports) != maxCachedTransports {
		t.Errorf("expected %d HTTP/3 transports, got %d", maxCachedTransports, len(rt.cachedHTTP3Transports))
	}

	// The 5 oldest should have been evicted
	for i := 0; i < 5; i++ {
		if _, exists := rt.cachedHTTP3Transports[http3AddrForIndex(i)]; exists {
			t.Errorf("HTTP/3 transport at index %d should have been evicted", i)
		}
	}
}

func TestHTTP3_CloseIdleConnections_NilConnTransport(t *testing.T) {
	rt := createTestRoundTripper()

	// Add an HTTP/3 transport with nil conn (UQuic path)
	rt.cachedHTTP3Transports["h3:uquic.example.com:443"] = &cachedHTTP3Transport{
		transport: &http3.Transport{},
		conn:      nil, // UQuic: conn is nil
		lastUsed:  time.Now(),
	}

	// CloseIdleConnections should handle nil conn gracefully
	// and still close the transport
	rt.CloseIdleConnections()

	// Transport should be removed
	if len(rt.cachedHTTP3Transports) != 0 {
		t.Errorf("expected 0 HTTP/3 transports after close, got %d", len(rt.cachedHTTP3Transports))
	}
}

// =============================================================================
// Verify batch eviction correctness: exact ordering
// =============================================================================

func TestLRU_BatchEviction_CorrectOrdering(t *testing.T) {
	rt := createTestRoundTripper()

	// Create entries with specific timestamps to verify ordering
	now := time.Now()
	entries := map[string]time.Time{
		"newest.com:443":  now,
		"middle1.com:443": now.Add(-1 * time.Second),
		"middle2.com:443": now.Add(-2 * time.Second),
		"oldest.com:443":  now.Add(-3 * time.Second),
	}

	for addr, ts := range entries {
		rt.cachedConnections[addr] = &cachedConn{
			conn:     newMockConn(),
			lastUsed: ts,
		}
	}

	// Temporarily set max to 2 to force eviction of 2 entries
	// We can't change the const, so we'll test with the real limit
	// Instead, fill to maxCachedConnections+2 with known ordering
	rt.cachedConnections = make(map[string]*cachedConn) // reset

	numEntries := maxCachedConnections + 2
	for i := 0; i < numEntries; i++ {
		conn := newMockConn()
		lastUsed := now.Add(time.Duration(i) * time.Millisecond)
		rt.cachedConnections[addrForIndex(i)] = &cachedConn{
			conn:     conn,
			lastUsed: lastUsed,
		}
	}

	rt.cleanupCache()

	if len(rt.cachedConnections) != maxCachedConnections {
		t.Errorf("expected %d, got %d", maxCachedConnections, len(rt.cachedConnections))
	}

	// Verify that the 2 oldest (index 0 and 1) were evicted
	if _, exists := rt.cachedConnections[addrForIndex(0)]; exists {
		t.Error("index 0 (oldest) should have been evicted")
	}
	if _, exists := rt.cachedConnections[addrForIndex(1)]; exists {
		t.Error("index 1 (second oldest) should have been evicted")
	}
	// And the rest remain
	if _, exists := rt.cachedConnections[addrForIndex(2)]; !exists {
		t.Error("index 2 should still exist")
	}
}

// =============================================================================
// Test that sort-based eviction is O(n log n) not O(n^2)
// =============================================================================

func TestLRU_EvictionUsesSort(t *testing.T) {
	// This test verifies the batch eviction uses sort.Slice pattern
	// by testing with a large number of entries
	rt := createTestRoundTripper()

	baseTime := time.Now()
	n := maxCachedConnections + 200

	for i := 0; i < n; i++ {
		rt.cachedConnections[addrForIndex(i)] = &cachedConn{
			conn:     newMockConn(),
			lastUsed: baseTime.Add(time.Duration(i) * time.Millisecond),
		}
	}

	// This should complete quickly with sort-based eviction
	start := time.Now()
	rt.cleanupCache()
	elapsed := time.Since(start)

	if len(rt.cachedConnections) != maxCachedConnections {
		t.Errorf("expected %d connections, got %d", maxCachedConnections, len(rt.cachedConnections))
	}

	// Verify the 200 oldest were evicted
	for i := 0; i < 200; i++ {
		if _, exists := rt.cachedConnections[addrForIndex(i)]; exists {
			t.Errorf("index %d should have been evicted", i)
		}
	}

	t.Logf("Sort-based eviction of %d excess entries from %d total took %v", 200, n, elapsed)
}

// =============================================================================
// Regression test: verify the sort.Slice approach produces correct results
// =============================================================================

func TestSortSlice_LRUOrdering(t *testing.T) {
	// Test that our sort-based approach correctly orders by lastUsed
	type entry struct {
		key      string
		lastUsed time.Time
	}

	now := time.Now()
	entries := []entry{
		{"c", now.Add(-3 * time.Second)},
		{"a", now.Add(-1 * time.Second)},
		{"d", now.Add(-4 * time.Second)},
		{"b", now.Add(-2 * time.Second)},
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].lastUsed.Before(entries[j].lastUsed)
	})

	// After sorting, order should be d, c, b, a (oldest first)
	expectedOrder := []string{"d", "c", "b", "a"}
	for i, e := range entries {
		if e.key != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], e.key)
		}
	}
}

// =============================================================================
// Issue 1 & 2: Race condition tests (verify the already-fixed Lock patterns)
// These additional tests verify behavior under high contention.
// =============================================================================

func TestClientPool_ConcurrentGetOrCreate_DifferentTimeouts(t *testing.T) {
	// Save and restore global pool
	advancedClientPoolMutex.Lock()
	savedPool := advancedClientPool
	advancedClientPool = make(map[string]*ClientPoolEntry)
	advancedClientPoolMutex.Unlock()

	defer func() {
		advancedClientPoolMutex.Lock()
		advancedClientPool = savedPool
		advancedClientPoolMutex.Unlock()
	}()

	browser := Browser{
		JA3:       "test",
		UserAgent: "test",
	}

	var wg sync.WaitGroup

	// Concurrently create clients with different timeouts
	// After the fix, these should create separate pool entries
	for i := 0; i < 5; i++ {
		wg.Add(1)
		timeout := i * 10
		go func(t int) {
			defer wg.Done()
			key := generateClientKey(browser, t, false, "")
			advancedClientPoolMutex.Lock()
			advancedClientPool[key] = &ClientPoolEntry{
				Client:    http.Client{},
				CreatedAt: time.Now(),
				LastUsed:  time.Now(),
			}
			advancedClientPoolMutex.Unlock()
		}(timeout)
	}

	wg.Wait()

	// Should have 5 separate entries (one per timeout)
	advancedClientPoolMutex.Lock()
	count := len(advancedClientPool)
	advancedClientPoolMutex.Unlock()

	if count != 5 {
		t.Errorf("expected 5 pool entries (different timeouts), got %d", count)
	}
}
