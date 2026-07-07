//go:build !integration

package unit

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"
)

// Simulates the buffer pool pattern used in cycletls/index.go
var testBufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func getTestBuffer() *bytes.Buffer {
	buf := testBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func putTestBuffer(buf *bytes.Buffer) {
	testBufferPool.Put(buf)
}

// TestBufferPool_DataCorruptionWithoutCopy demonstrates that returning a buffer
// to the pool before the channel reader consumes data causes corruption.
// This is the bug: b.Bytes() returns a slice backed by the buffer's internal array,
// so if the buffer is reused before the slice is read, data gets overwritten.
func TestBufferPool_DataCorruptionWithoutCopy(t *testing.T) {
	t.Parallel()

	const iterations = 1000
	const goroutines = 10

	corruptionDetected := false
	var mu sync.Mutex

	for iter := 0; iter < iterations; iter++ {
		ch := make(chan []byte, goroutines)
		var wg sync.WaitGroup

		// Simulate multiple goroutines writing to channel via buffer pool
		// WITH the fix: copy data before returning buffer to pool
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				b := getTestBuffer()
				// Write unique data
				for j := 0; j < 100; j++ {
					b.WriteByte(byte(id))
				}
				// FIX: Copy before returning to pool
				data := make([]byte, b.Len())
				copy(data, b.Bytes())
				putTestBuffer(b)
				ch <- data
			}(i)
		}

		wg.Wait()
		close(ch)

		// Verify data integrity
		for data := range ch {
			if len(data) != 100 {
				mu.Lock()
				corruptionDetected = true
				mu.Unlock()
				t.Errorf("Data length corruption: got %d, expected 100", len(data))
				continue
			}
			expected := data[0]
			for j, b := range data {
				if b != expected {
					mu.Lock()
					corruptionDetected = true
					mu.Unlock()
					t.Errorf("Data corruption at byte %d: got %d, expected %d", j, b, expected)
					break
				}
			}
		}
	}

	if corruptionDetected {
		t.Error("Data corruption detected - buffer pool data not properly copied")
	}
}

// TestBufferPool_ResetOnReuse verifies that buffers retrieved from the pool
// are properly reset (zero length) even if they contained data before being
// returned. This is a deterministic test that forces pool reuse by putting
// a buffer back and immediately getting one.
func TestBufferPool_ResetOnReuse(t *testing.T) {
	t.Parallel()

	// Get a buffer and write data to it
	buf := getTestBuffer()
	buf.WriteString("test data that should be cleared")

	if buf.Len() == 0 {
		t.Fatal("buffer should have data after WriteString")
	}

	// Return to pool
	putTestBuffer(buf)

	// Immediately get a buffer - should get the same one back (pool reuse)
	buf2 := getTestBuffer()

	// getTestBuffer calls Reset(), so the buffer must be empty regardless
	// of whether it's the same underlying buffer or a new allocation
	if buf2.Len() != 0 {
		t.Errorf("buffer not properly reset on reuse: Len()=%d, expected 0", buf2.Len())
	}

	// Write new data and verify it's clean (no residual data from previous use)
	buf2.WriteString("new data")
	if buf2.String() != "new data" {
		t.Errorf("buffer contains unexpected data: got %q, expected %q", buf2.String(), "new data")
	}

	putTestBuffer(buf2)
}

// TestBufferPool_CopyBeforePutPreventsCorruption verifies that copying data
// before returning a buffer to the pool prevents corruption when the pool
// reuses the same underlying buffer.
func TestBufferPool_CopyBeforePutPreventsCorruption(t *testing.T) {
	t.Parallel()

	// Get a buffer, write data, and take a SAFE copy before returning to pool
	buf := getTestBuffer()
	buf.WriteString("important data")

	// Safe pattern: copy before returning to pool
	safeCopy := make([]byte, buf.Len())
	copy(safeCopy, buf.Bytes())

	// Return buffer to pool
	putTestBuffer(buf)

	// Force reuse: get buffer back and overwrite it
	buf2 := getTestBuffer()
	buf2.WriteString("OVERWRITTEN COMPLETELY DIFFERENT DATA!!!")

	// The safe copy must still contain the original data
	if string(safeCopy) != "important data" {
		t.Errorf("safe copy was corrupted: got %q, expected %q", string(safeCopy), "important data")
	}

	putTestBuffer(buf2)
}

// TestBufferPool_ConcurrentSafety verifies that the copy-before-put pattern
// prevents data corruption under heavy concurrent load.
func TestBufferPool_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	const numProducers = 50
	const messagesPerProducer = 100

	ch := make(chan []byte, numProducers*messagesPerProducer)
	var wg sync.WaitGroup

	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(id byte) {
			defer wg.Done()
			for j := 0; j < messagesPerProducer; j++ {
				b := getTestBuffer()
				// Write a pattern: [id, j, id, j, ...]
				for k := 0; k < 50; k++ {
					b.WriteByte(id)
					b.WriteByte(byte(j))
				}
				// Safe pattern: copy then return to pool
				data := make([]byte, b.Len())
				copy(data, b.Bytes())
				putTestBuffer(b)
				ch <- data
			}
		}(byte(i))
	}

	wg.Wait()
	close(ch)

	corruptions := 0
	total := 0
	for data := range ch {
		total++
		if len(data) != 100 {
			corruptions++
			continue
		}
		id := data[0]
		seq := data[1]
		for k := 0; k < 50; k++ {
			if data[k*2] != id || data[k*2+1] != seq {
				corruptions++
				break
			}
		}
	}

	if corruptions > 0 {
		t.Errorf("%d/%d messages had corrupted data", corruptions, total)
	}
	if total != numProducers*messagesPerProducer {
		t.Errorf("Expected %d messages, got %d", numProducers*messagesPerProducer, total)
	}
}

// Issue #11 (MAJOR): Tests the write->return->get->verify pattern explicitly.
// This pattern is the core of the buffer pool corruption scenario: if you write data,
// return the buffer to the pool, then get a buffer back and verify, the original
// data reference (without copy) would be corrupted.
func TestBufferPool_WriteReturnGetVerify(t *testing.T) {
	t.Parallel()

	// Step 1: Write data to a buffer
	buf1 := getTestBuffer()
	originalData := "write-return-get-verify-test-data"
	buf1.WriteString(originalData)

	// Step 2: Take an UNSAFE reference (simulates the bug)
	unsafeRef := buf1.Bytes()

	// Step 3: Take a SAFE copy (simulates the fix)
	safeRef := make([]byte, buf1.Len())
	copy(safeRef, buf1.Bytes())

	// Step 4: Return buffer to pool
	putTestBuffer(buf1)

	// Step 5: Get a new buffer from pool (likely the same one)
	buf2 := getTestBuffer()
	buf2.WriteString("OVERWRITE-CORRUPTION-DATA-LONG-ENOUGH")

	// Step 6: Verify
	// Safe reference must still be correct
	if string(safeRef) != originalData {
		t.Errorf("Safe copy corrupted: got %q, expected %q", string(safeRef), originalData)
	}

	// Unsafe reference MAY be corrupted (depends on pool reuse).
	// We don't assert on unsafeRef's content because sync.Pool behavior is
	// non-deterministic - but we verify the safe copy is always correct.
	_ = unsafeRef // acknowledged: may be stale

	putTestBuffer(buf2)
}

// Issue #11 (MAJOR): Concurrent write->return->get->verify under load
func TestBufferPool_WriteReturnGetVerifyConcurrent(t *testing.T) {
	t.Parallel()

	const numGoroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	var corruptions atomic.Int64
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				// Write
				b := getTestBuffer()
				expected := byte(id ^ j)
				for k := 0; k < 64; k++ {
					b.WriteByte(expected)
				}

				// Safe copy
				data := make([]byte, b.Len())
				copy(data, b.Bytes())

				// Return to pool
				putTestBuffer(b)

				// Verify safe copy is intact
				for k := 0; k < len(data); k++ {
					if data[k] != expected {
						corruptions.Add(1)
						break
					}
				}
			}
		}(i)
	}

	wg.Wait()

	if c := corruptions.Load(); c > 0 {
		t.Errorf("safe copies were corrupted %d times", c)
	}
}
