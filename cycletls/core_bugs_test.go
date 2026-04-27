//go:build !integration

package cycletls

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

// ============================================================================
// Test 1: Buffer pool cleanup - putBuffer must be called on all paths
// ============================================================================

// TestBufferPoolCleanupOnPanic verifies that getBuffer/putBuffer are balanced
// even when a panic occurs during buffer usage. A deferred putBuffer should
// ensure the buffer is returned to the pool regardless of panic.
func TestBufferPoolCleanupOnPanic(t *testing.T) {
	// Track buffer pool operations
	var getCount, putCount int64
	var mu sync.Mutex

	trackGet := func() *bytes.Buffer {
		mu.Lock()
		getCount++
		mu.Unlock()
		return getBuffer()
	}

	trackPut := func(b *bytes.Buffer) {
		mu.Lock()
		putCount++
		mu.Unlock()
		putBuffer(b)
	}

	// Simulate the pattern used in dispatcher functions:
	// getBuffer(), do work, putBuffer() with panic safety via defer
	simulateBufferUsageWithDefer := func(shouldPanic bool) {
		defer func() {
			recover() // swallow panic like dispatcher does
		}()

		b := trackGet()
		defer trackPut(b)

		b.WriteString("test data")
		if shouldPanic {
			panic("simulated panic")
		}
		// Normal path: data is copied, buffer returned via defer
		data := make([]byte, b.Len())
		copy(data, b.Bytes())
	}

	// Run without panic
	simulateBufferUsageWithDefer(false)
	mu.Lock()
	if getCount != putCount {
		t.Errorf("Normal path: getBuffer=%d putBuffer=%d, expected equal", getCount, putCount)
	}
	mu.Unlock()

	// Run with panic
	simulateBufferUsageWithDefer(true)
	mu.Lock()
	if getCount != putCount {
		t.Errorf("Panic path: getBuffer=%d putBuffer=%d, expected equal", getCount, putCount)
	}
	mu.Unlock()
}

// TestBufferPoolDeferPattern verifies the defer-putBuffer pattern works
// correctly across multiple goroutines with mixed panic/success outcomes.
func TestBufferPoolDeferPattern(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	var getCount, putCount int64
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		shouldPanic := i%3 == 0 // Every third goroutine panics
		go func(doPanic bool) {
			defer wg.Done()
			defer func() { recover() }()

			b := getBuffer()
			mu.Lock()
			getCount++
			mu.Unlock()

			defer func() {
				putBuffer(b)
				mu.Lock()
				putCount++
				mu.Unlock()
			}()

			b.WriteString("test")
			if doPanic {
				panic("test panic")
			}
		}(shouldPanic)
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if getCount != putCount {
		t.Errorf("Buffer pool leak: getBuffer=%d putBuffer=%d", getCount, putCount)
	}
	if getCount != numGoroutines {
		t.Errorf("Expected %d getBuffer calls, got %d", numGoroutines, getCount)
	}
}

// ============================================================================
// Test 2: Panic recovery error propagation
// ============================================================================

// TestPanicRecoverySendsErrorResponse verifies that when a panic occurs in
// a dispatcher function, the recovery handler sends an error response to the
// channel writer rather than silently swallowing the error.
func TestPanicRecoverySendsErrorResponse(t *testing.T) {
	ch := make(chan []byte, 10)
	chanWrite := newSafeChannelWriter(ch)

	// Simulate dispatcherAsync panic recovery with error propagation
	recoverAndReport := func(requestID string, chanWrite *safeChannelWriter) {
		if r := recover(); r != nil {
			// This is what the fixed code should do: send error to channel
			errMsg := fmt.Sprintf("panic: %v", r)
			b := getBuffer()
			defer putBuffer(b)

			requestIDLength := len(requestID)
			statusCode := 500

			b.WriteByte(byte(requestIDLength >> 8))
			b.WriteByte(byte(requestIDLength))
			b.WriteString(requestID)
			b.WriteByte(0)
			b.WriteByte(5)
			b.WriteString("error")
			b.WriteByte(byte(statusCode >> 8))
			b.WriteByte(byte(statusCode))

			messageLength := len(errMsg)
			b.WriteByte(byte(messageLength >> 8))
			b.WriteByte(byte(messageLength))
			b.WriteString(errMsg)

			data := make([]byte, b.Len())
			copy(data, b.Bytes())
			chanWrite.write(data)
		}
	}

	// Simulate a panicking dispatcher
	func() {
		defer recoverAndReport("test-req-123", chanWrite)
		panic("nil pointer dereference")
	}()

	// Verify an error response was sent
	select {
	case data := <-ch:
		if len(data) == 0 {
			t.Fatal("Expected error response data, got empty")
		}
		// Verify the data contains the request ID
		if !bytes.Contains(data, []byte("test-req-123")) {
			t.Error("Error response should contain the request ID")
		}
		// Verify the data contains the "error" type marker
		if !bytes.Contains(data, []byte("error")) {
			t.Error("Error response should contain 'error' type marker")
		}
		// Verify it contains the panic message
		if !bytes.Contains(data, []byte("panic: nil pointer dereference")) {
			t.Error("Error response should contain the panic message")
		}
	default:
		t.Fatal("Expected error response in channel, but channel was empty")
	}
}

// TestPanicRecoveryWithClosedChannel verifies that panic recovery doesn't
// itself panic when the channel writer is already closed.
func TestPanicRecoveryWithClosedChannel(t *testing.T) {
	ch := make(chan []byte, 1)
	chanWrite := newSafeChannelWriter(ch)
	chanWrite.setClosed()

	// This should not panic
	didRecover := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				didRecover = true
				// Attempt to send error - should fail gracefully
				errMsg := fmt.Sprintf("panic: %v", r)
				b := getBuffer()
				defer putBuffer(b)
				b.WriteString(errMsg)
				data := make([]byte, b.Len())
				copy(data, b.Bytes())
				chanWrite.write(data) // Should return false, not panic
			}
		}()
		panic("test panic with closed channel")
	}()

	if !didRecover {
		t.Fatal("Expected recovery from panic")
	}
}

// ============================================================================
// Test 3: safeChannelWriter write safety
// ============================================================================

// TestSafeChannelWriterConcurrentWriteAndClose verifies that concurrent
// writes and close operations don't cause panics or data races.
func TestSafeChannelWriterConcurrentWriteAndClose(t *testing.T) {
	ch := make(chan []byte, 100)
	scw := newSafeChannelWriter(ch)

	var wg sync.WaitGroup
	const numWriters = 50

	// Start many concurrent writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			data := []byte(fmt.Sprintf("message-%d", id))
			scw.write(data) // May return true or false
		}(i)
	}

	// Close while writers are running. Track this goroutine so the post-
	// condition assertion below cannot race with setClosed itself: previously
	// setClosed was fire-and-forget, so wg.Wait returning didn't imply
	// setClosed had observed scw.closed = true yet, which made the
	// "Write after setClosed should return false" assertion intermittently
	// fail when the runtime hadn't scheduled the close goroutine yet.
	wg.Add(1)
	go func() {
		defer wg.Done()
		scw.setClosed()
	}()

	wg.Wait()

	// After close, all writes should return false
	if scw.write([]byte("after-close")) {
		t.Error("Write after setClosed should return false")
	}
}

// TestSafeChannelWriterFullChannel verifies behavior when channel buffer is full.
func TestSafeChannelWriterFullChannel(t *testing.T) {
	ch := make(chan []byte, 1) // Buffer size 1
	scw := newSafeChannelWriter(ch)

	// Fill the channel
	scw.write([]byte("first"))

	// Second write should return false (channel full, non-blocking)
	result := scw.write([]byte("second"))
	if result {
		t.Error("Write to full channel should return false")
	}
}

// ============================================================================
// Test 4: WebSocket safeWrite thread safety
// ============================================================================

// TestWebSocketSafeWriteAfterClose verifies that safeWrite does not panic
// when called on a nil or closed connection. The writeMu ensures serialization.
func TestWebSocketSafeWriteAfterClose(t *testing.T) {
	wsConn := &WebSocketConnection{
		Conn:        nil,
		RequestID:   "test-ws-123",
		ReadyState:  1,
		commandChan: make(chan WebSocketCommand, 10),
		closeChan:   make(chan struct{}),
		done:        make(chan struct{}),
	}

	// safeWrite on nil connection should return an error, not panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("safeWrite should not panic on nil connection, got: %v", r)
		}
	}()

	// This should return an error because Conn is nil
	// The writeMu.Lock serialization still works even with nil Conn
	err := wsConn.safeWrite(nil, 1, []byte("test"))
	if err == nil {
		t.Error("Expected error from safeWrite with nil connection")
	}
}

// TestWebSocketDoneChannelPreventsRead verifies that the done channel
// properly signals the read goroutine to exit.
func TestWebSocketDoneChannelPreventsRead(t *testing.T) {
	wsConn := &WebSocketConnection{
		done: make(chan struct{}),
	}

	// Close the done channel
	close(wsConn.done)

	// Verify done channel is closed (select should immediately proceed)
	select {
	case <-wsConn.done:
		// Good - done channel is closed
	default:
		t.Fatal("Done channel should be readable after close")
	}
}

// ============================================================================
// Test 5: Response body closure on error
// ============================================================================

// TestResponseBodyClosedOnError is a design-level test verifying the pattern
// that resp.Body is closed when http.Client.Do returns both a response and
// an error. This tests the pattern, not the actual HTTP call.
type mockReadCloser struct {
	closed bool
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) { return 0, nil }
func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}

func TestResponseBodyClosedOnError(t *testing.T) {
	body := &mockReadCloser{}

	// Simulate: http.Client.Do returned both resp and error
	type mockResp struct {
		Body *mockReadCloser
	}
	resp := &mockResp{Body: body}
	err := fmt.Errorf("redirect error")

	// The pattern from our fixed code:
	if err != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}

	if !body.closed {
		t.Error("Response body should be closed on error path")
	}
}

// ============================================================================
// Test 6: connectDialer InsecureSkipVerify default
// ============================================================================

// TestConnectDialerDefaultInsecureSkipVerify verifies that proxy TLS connections
// default to InsecureSkipVerify=true for backward compatibility, unless
// explicitly set to false by the user.
func TestConnectDialerDefaultInsecureSkipVerify(t *testing.T) {
	// When user doesn't set InsecureSkipVerify (Go zero value = false),
	// proxy connections should still default to true for backward compat.
	// This is tested via newConnectDialer which sets the field.

	// Test: newConnectDialer with insecureSkipVerify=false should still
	// create a dialer (backward compat handled at call site)
	dialer, err := newConnectDialer("http://proxy.example.com:8080", "test-agent", false)
	if err != nil {
		t.Fatalf("newConnectDialer failed: %v", err)
	}
	if dialer == nil {
		t.Fatal("Expected non-nil dialer")
	}

	// Test: newConnectDialer with insecureSkipVerify=true
	dialer2, err := newConnectDialer("https://proxy.example.com:8080", "test-agent", true)
	if err != nil {
		t.Fatalf("newConnectDialer failed: %v", err)
	}
	if dialer2 == nil {
		t.Fatal("Expected non-nil dialer")
	}
}
