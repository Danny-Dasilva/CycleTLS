// Package state provides shared global state for the cycletls package.
// This includes pooled buffers and debug logging utilities used across
// multiple components.
package state

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// DebugEnvVar names the environment variable that turns debug output on.
const DebugEnvVar = "CYCLETLS_DEBUG"

// DebugEnabled reports whether CYCLETLS_DEBUG asks for debug output. Any value
// other than empty, "0", "false", "off" and "no" turns it on.
func DebugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(DebugEnvVar))) {
	case "", "0", "false", "off", "no":
		return false
	}
	return true
}

// debugOutput is where DebugLogger writes: stdout when CYCLETLS_DEBUG is set,
// nowhere otherwise. The Go process usually runs as a child of a Node client.
// A parent that does not read the child's stdout lets the pipe fill, and the
// next debug line then blocks the request that wrote it.
func debugOutput() io.Writer {
	if DebugEnabled() {
		return os.Stdout
	}
	return io.Discard
}

// DebugLogger provides a logger for debug output with timestamp and file info.
// It is silent unless CYCLETLS_DEBUG is set when the process starts.
var DebugLogger = log.New(debugOutput(), "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)

// bufferPool provides reusable bytes.Buffer instances to reduce GC pressure
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// GetBuffer retrieves a buffer from the pool and resets it for reuse.
func GetBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// maxBufferSize is the maximum capacity of a buffer that will be returned to the pool.
// Buffers larger than this are discarded to prevent unbounded memory growth from
// occasional large responses.
const maxBufferSize = 256 * 1024 // 256KB

// PutBuffer returns a buffer to the pool for reuse.
// Issue #8 fix: Reset the buffer before returning to pool to prevent stale data
// from being accessible if a future caller reads before writing. Also discard
// oversized buffers to prevent unbounded memory growth.
func PutBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	// Discard oversized buffers to prevent memory bloat
	if buf.Cap() > maxBufferSize {
		return
	}
	buf.Reset()
	bufferPool.Put(buf)
}
