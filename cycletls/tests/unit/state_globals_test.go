//go:build !integration

package unit

import (
	"sync"
	"testing"

	"github.com/Danny-Dasilva/CycleTLS/cycletls/state"
)

func TestGetBuffer_ReturnsValidBuffer(t *testing.T) {
	buf := state.GetBuffer()
	if buf == nil {
		t.Fatal("GetBuffer() returned nil")
	}

	testData := []byte("test data")
	n, err := buf.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write to buffer: %v", err)
	}
	if n != len(testData) {
		t.Fatalf("Expected to write %d bytes, wrote %d", len(testData), n)
	}

	state.PutBuffer(buf)
}

func TestGetBuffer_ReturnsEmptyBuffer(t *testing.T) {
	buf := state.GetBuffer()
	buf.WriteString("some existing data")
	state.PutBuffer(buf)

	buf2 := state.GetBuffer()
	if buf2.Len() != 0 {
		t.Errorf("GetBuffer() returned buffer with Len() = %d, expected 0", buf2.Len())
	}
	state.PutBuffer(buf2)
}

func TestPutBuffer_AcceptsBufferWithoutPanic(t *testing.T) {
	buf := state.GetBuffer()
	buf.WriteString("test content")
	state.PutBuffer(buf)
}

func TestBufferPool_ConcurrentAccess(t *testing.T) {
	const numGoroutines = 20
	const numIterations = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				buf := state.GetBuffer()
				if buf == nil {
					t.Errorf("GetBuffer returned nil")
					return
				}
				buf.WriteString("concurrent test data")
				state.PutBuffer(buf)
			}
		}(i)
	}

	wg.Wait()
}

func TestDebugLogger_Initialized(t *testing.T) {
	if state.DebugLogger == nil {
		t.Fatal("DebugLogger is nil")
	}

	prefix := state.DebugLogger.Prefix()
	if prefix != "DEBUG: " {
		t.Errorf("DebugLogger prefix = %q, expected %q", prefix, "DEBUG: ")
	}
}
