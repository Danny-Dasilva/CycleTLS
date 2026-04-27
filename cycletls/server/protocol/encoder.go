// Package protocol provides binary frame encoding for CycleTLS WebSocket communication.
// The protocol uses big-endian byte ordering for all multi-byte integers.
package protocol

import (
	"bytes"
	"fmt"
	"math"
	"sync"
)

// Frame types used in the binary protocol
const (
	FrameTypeError     = "error"
	FrameTypeResponse  = "response"
	FrameTypeData      = "data"
	FrameTypeEnd       = "end"
	FrameTypeWSOpen    = "ws_open"
	FrameTypeWSMessage = "ws_message"
	FrameTypeWSClose   = "ws_close"
	FrameTypeWSError   = "ws_error"
)

// bufferPool provides reusable buffers for frame encoding
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// getBuffer retrieves a buffer from the pool
func getBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// putBuffer returns a buffer to the pool
func putBuffer(buf *bytes.Buffer) {
	if buf.Cap() > 64*1024 { // Don't pool very large buffers
		return
	}
	bufferPool.Put(buf)
}

// Helper functions for encoding.
// writeUint16 writes a 16-bit value. Callers must ensure v is in range [0, 65535].
// For untrusted input, use safeWriteUint16 instead.
func writeUint16(b *bytes.Buffer, v int) {
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
}

// writeUint32 writes a 32-bit value. Callers must ensure v is in range [0, 4294967295].
// For untrusted input, use safeWriteUint32 instead.
func writeUint32(b *bytes.Buffer, v int) {
	b.WriteByte(byte(v >> 24))
	b.WriteByte(byte(v >> 16))
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
}

// safeWriteUint16 writes a 16-bit value with bounds checking.
// Returns an error if v is negative or exceeds math.MaxUint16 (65535).
func safeWriteUint16(b *bytes.Buffer, v int) error {
	if v < 0 || v > math.MaxUint16 {
		return fmt.Errorf("protocol: value %d overflows uint16 (range 0-%d)", v, math.MaxUint16)
	}
	writeUint16(b, v)
	return nil
}

// safeWriteUint32 writes a 32-bit value with bounds checking.
// Returns an error if v is negative or exceeds math.MaxUint32 (4294967295).
func safeWriteUint32(b *bytes.Buffer, v int) error {
	if v < 0 || int64(v) > int64(math.MaxUint32) {
		return fmt.Errorf("protocol: value %d overflows uint32 (range 0-%d)", v, uint64(math.MaxUint32))
	}
	writeUint32(b, v)
	return nil
}

func writeRequestID(b *bytes.Buffer, requestID string) {
	writeUint16(b, len(requestID))
	b.WriteString(requestID)
}

func writeFrameType(b *bytes.Buffer, frameType string) {
	b.WriteByte(0)
	b.WriteByte(byte(len(frameType)))
	b.WriteString(frameType)
}

// EncodeError creates an error frame as bytes.
// Useful for cases where the caller needs to handle the bytes directly.
func EncodeError(requestID string, statusCode int, message string) []byte {
	b := getBuffer()
	defer putBuffer(b)

	writeRequestID(b, requestID)
	writeFrameType(b, FrameTypeError)
	writeUint16(b, statusCode)
	writeUint16(b, len(message))
	b.WriteString(message)

	// Return a copy since buffer is pooled
	result := make([]byte, b.Len())
	copy(result, b.Bytes())
	return result
}

// EncodeResponse creates a response header frame as bytes without sending.
func EncodeResponse(requestID string, statusCode int, url string, headers map[string][]string) []byte {
	b := getBuffer()
	defer putBuffer(b)

	writeRequestID(b, requestID)
	writeFrameType(b, FrameTypeResponse)
	writeUint16(b, statusCode)

	// Write URL
	writeUint16(b, len(url))
	b.WriteString(url)

	// Write headers
	writeUint16(b, len(headers))
	for name, values := range headers {
		writeUint16(b, len(name))
		b.WriteString(name)
		writeUint16(b, len(values))
		for _, value := range values {
			writeUint16(b, len(value))
			b.WriteString(value)
		}
	}

	// Return a copy since buffer is pooled
	result := make([]byte, b.Len())
	copy(result, b.Bytes())
	return result
}

// EncodeData creates a data chunk frame as bytes without sending.
func EncodeData(requestID string, data []byte) []byte {
	b := getBuffer()
	defer putBuffer(b)

	writeRequestID(b, requestID)
	writeFrameType(b, FrameTypeData)
	writeUint32(b, len(data))
	b.Write(data)

	// Return a copy since buffer is pooled
	result := make([]byte, b.Len())
	copy(result, b.Bytes())
	return result
}

// EncodeEnd creates an end-of-response frame as bytes without sending.
func EncodeEnd(requestID string) []byte {
	b := getBuffer()
	defer putBuffer(b)

	writeRequestID(b, requestID)
	writeFrameType(b, FrameTypeEnd)

	// Return a copy since buffer is pooled
	result := make([]byte, b.Len())
	copy(result, b.Bytes())
	return result
}
