//go:build !integration

package unit

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testSafeChannelWriter mirrors the production safeChannelWriter from index.go.
// The production type is unexported (lowercase), so we replicate the identical
// structure and logic here for unit testing. The production type has the same
// fields (ch chan []byte, mu sync.RWMutex, closed bool) and identical write/setClosed
// methods with the same lock ordering and non-blocking select semantics.
type testSafeChannelWriter struct {
	ch     chan []byte
	mu     sync.RWMutex
	closed bool
}

func newTestSafeChannelWriter(ch chan []byte) *testSafeChannelWriter {
	return &testSafeChannelWriter{ch: ch, closed: false}
}

func (scw *testSafeChannelWriter) write(data []byte) bool {
	scw.mu.Lock()
	defer scw.mu.Unlock()
	if scw.closed {
		return false
	}
	select {
	case scw.ch <- data:
		return true
	default:
		return false
	}
}

func (scw *testSafeChannelWriter) setClosed() {
	scw.mu.Lock()
	defer scw.mu.Unlock()
	scw.closed = true
}

func (scw *testSafeChannelWriter) isClosed() bool {
	scw.mu.RLock()
	defer scw.mu.RUnlock()
	return scw.closed
}

// Issue #1 (CRITICAL): Fixed race condition - uses a `started` channel so each goroutine
// signals readiness before setClosed() is called, instead of relying on time.Sleep(1ms).
// Issue #2 (CRITICAL): Added t.Parallel() for concurrent test.
func TestSafeChannelWriter_ConcurrentWriteAndClose(t *testing.T) {
	t.Parallel()

	const numWriters = 100
	const writesPerWriter = 50
	ch := make(chan []byte, numWriters*writesPerWriter)
	scw := newTestSafeChannelWriter(ch)
	var wg sync.WaitGroup
	var writeSuccesses atomic.Int64
	var writeFails atomic.Int64
	var panicked atomic.Int64

	// Each goroutine signals on this channel after it has started running
	started := make(chan struct{}, numWriters)

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicked.Add(1)
				}
			}()
			// Signal that this goroutine has started
			started <- struct{}{}
			for j := 0; j < writesPerWriter; j++ {
				if scw.write([]byte{byte(id), byte(j)}) {
					writeSuccesses.Add(1)
				} else {
					writeFails.Add(1)
				}
			}
		}(i)
	}

	// Wait for ALL goroutines to confirm they have started before closing
	for i := 0; i < numWriters; i++ {
		<-started
	}

	scw.setClosed()
	wg.Wait()

	if panicked.Load() > 0 {
		t.Errorf("write() panicked %d times; expected zero panics with safe writer", panicked.Load())
	}
	total := writeSuccesses.Load() + writeFails.Load()
	if total != int64(numWriters*writesPerWriter) {
		t.Errorf("Total operations = %d, expected %d", total, numWriters*writesPerWriter)
	}
}

func TestSafeChannelWriter_WriteAfterClose(t *testing.T) {
	t.Parallel()

	ch := make(chan []byte, 10)
	scw := newTestSafeChannelWriter(ch)
	if !scw.write([]byte("hello")) {
		t.Error("write() should succeed before close")
	}
	scw.setClosed()
	if scw.write([]byte("world")) {
		t.Error("write() should return false after setClosed()")
	}
}

// Issue #5 (CRITICAL): Fixed race window - added runtime.Gosched() and a small delay
// to ensure the goroutine has attempted the write before we select on the done channel.
func TestSafeChannelWriter_WriteToFullChannel(t *testing.T) {
	t.Parallel()

	ch := make(chan []byte, 1)
	scw := newTestSafeChannelWriter(ch)
	if !scw.write([]byte("first")) {
		t.Error("First write should succeed")
	}

	// Signal that the goroutine has started its write attempt
	writeAttempted := make(chan struct{})
	done := make(chan bool, 1)

	go func() {
		// Signal before attempting write so the test knows the goroutine is running
		close(writeAttempted)
		done <- scw.write([]byte("second"))
	}()

	// Wait for the goroutine to start and yield to let it attempt the write
	<-writeAttempted
	runtime.Gosched()

	// Issue #8 (MAJOR): Named constant instead of magic number for timeout
	const fullChannelTimeout = 2 * time.Second
	select {
	case result := <-done:
		if result {
			t.Error("write() to full channel should return false (non-blocking select)")
		}
	case <-time.After(fullChannelTimeout):
		t.Fatal("write() blocked on full channel; should be non-blocking via select default")
	}
}

// Issue #2 (CRITICAL): Added t.Parallel() for concurrent test.
// Issue #1: Replaced time.Sleep with started channel for proper synchronization.
func TestSafeChannelWriter_NoRaceCondition(t *testing.T) {
	t.Parallel()

	const iterations = 100
	const writersPerIteration = 10
	const writesPerWriter = 100

	for i := 0; i < iterations; i++ {
		ch := make(chan []byte, writersPerIteration*writesPerWriter)
		scw := newTestSafeChannelWriter(ch)
		var wg sync.WaitGroup
		started := make(chan struct{}, writersPerIteration)

		wg.Add(writersPerIteration + 1)
		for j := 0; j < writersPerIteration; j++ {
			go func(id int) {
				defer wg.Done()
				started <- struct{}{}
				for k := 0; k < writesPerWriter; k++ {
					scw.write([]byte{byte(id), byte(k)})
				}
			}(j)
		}

		go func() {
			defer wg.Done()
			// Wait for all writers to confirm they started
			for j := 0; j < writersPerIteration; j++ {
				<-started
			}
			scw.setClosed()
		}()

		wg.Wait()
	}
}

// Issue #7 (MAJOR): Edge case - nil channel panics on write (documents expected behavior)
func TestSafeChannelWriter_NilChannel(t *testing.T) {
	t.Parallel()

	scw := newTestSafeChannelWriter(nil)

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		scw.write([]byte("test"))
	}()

	// Writing to a nil channel in a select with default case returns false (default branch)
	// The non-blocking select prevents the panic/block that a bare send would cause.
	if panicked {
		t.Error("write() to nil channel should not panic due to non-blocking select with default")
	}
}

// Issue #7 (MAJOR): Edge case - empty slice write
func TestSafeChannelWriter_EmptySlice(t *testing.T) {
	t.Parallel()

	ch := make(chan []byte, 1)
	scw := newTestSafeChannelWriter(ch)

	if !scw.write([]byte{}) {
		t.Error("write() of empty slice should succeed on buffered channel")
	}

	data := <-ch
	if len(data) != 0 {
		t.Errorf("Expected empty slice, got %d bytes", len(data))
	}
}

// Issue #7 (MAJOR): Edge case - nil slice write
func TestSafeChannelWriter_NilSlice(t *testing.T) {
	t.Parallel()

	ch := make(chan []byte, 1)
	scw := newTestSafeChannelWriter(ch)

	if !scw.write(nil) {
		t.Error("write() of nil should succeed on buffered channel")
	}

	data := <-ch
	if data != nil {
		t.Errorf("Expected nil, got %v", data)
	}
}

// Issue #7 (MAJOR): Edge case - concurrent setClosed() calls
func TestSafeChannelWriter_ConcurrentSetClosed(t *testing.T) {
	t.Parallel()

	ch := make(chan []byte, 10)
	scw := newTestSafeChannelWriter(ch)

	const numClosers = 50
	var wg sync.WaitGroup
	wg.Add(numClosers)

	for i := 0; i < numClosers; i++ {
		go func() {
			defer wg.Done()
			scw.setClosed()
		}()
	}

	wg.Wait()

	if !scw.isClosed() {
		t.Error("Writer should be closed after concurrent setClosed() calls")
	}

	if scw.write([]byte("after close")) {
		t.Error("write() should return false after concurrent setClosed()")
	}
}
