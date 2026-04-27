package cycletls

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// ErrU16Overflow is returned when a value exceeds uint16 range.
var ErrU16Overflow = errors.New("packet: value exceeds uint16 range (0-65535)")

// ErrU32Overflow is returned when a value exceeds uint32 range.
var ErrU32Overflow = errors.New("packet: value exceeds uint32 range (0-4294967295)")

// ErrTooManyHeaders is returned when the header map exceeds the uint16 limit.
var ErrTooManyHeaders = errors.New("packet: header count exceeds uint16 limit (65535)")

// writeU16 writes a 16-bit unsigned integer in big-endian format.
// Returns an error if v is outside the uint16 range [0, 65535].
func writeU16(b *bytes.Buffer, v int) error {
	if v < 0 || v > math.MaxUint16 {
		return ErrU16Overflow
	}
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
	return nil
}

// writeStringWithLen writes a string with a 2-byte big-endian length prefix.
func writeStringWithLen(b *bytes.Buffer, s string) error {
	if err := writeU16(b, len(s)); err != nil {
		return fmt.Errorf("string length %d: %w", len(s), err)
	}
	b.WriteString(s)
	return nil
}

// writeRequestAndMethod writes the request ID and method name using length-prefixed format.
func writeRequestAndMethod(b *bytes.Buffer, requestID, method string) error {
	if err := writeStringWithLen(b, requestID); err != nil {
		return err
	}
	return writeStringWithLen(b, method)
}

// buildErrorFrame creates an error response packet with status code and message.
func buildErrorFrame(requestID string, statusCode int, message string) []byte {
	var b bytes.Buffer
	// Internal frames: requestID, method, statusCode, message are all bounded
	_ = writeRequestAndMethod(&b, requestID, "error")
	_ = writeU16(&b, statusCode)
	_ = writeStringWithLen(&b, message)
	return b.Bytes()
}

// buildEndFrame generates a frame signaling request completion.
func buildEndFrame(requestID string) []byte {
	var b bytes.Buffer
	_ = writeRequestAndMethod(&b, requestID, "end")
	return b.Bytes()
}

// writeU32 writes a 32-bit unsigned integer in big-endian format.
// Returns an error if v is outside the uint32 range [0, 4294967295].
func writeU32(b *bytes.Buffer, v int) error {
	if v < 0 || int64(v) > int64(math.MaxUint32) {
		return ErrU32Overflow
	}
	b.WriteByte(byte(v >> 24))
	b.WriteByte(byte(v >> 16))
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
	return nil
}

// buildDataFrame packages response body data with a 4-byte length header.
func buildDataFrame(requestID string, body []byte) []byte {
	var b bytes.Buffer
	_ = writeRequestAndMethod(&b, requestID, "data")
	_ = writeU32(&b, len(body))
	b.Write(body)
	return b.Bytes()
}

// buildResponseFrame constructs the main response packet containing status code, final URL, and HTTP headers.
// Returns an error if the header map exceeds 65535 entries (uint16 limit).
func buildResponseFrame(requestID string, statusCode int, finalURL string, headers map[string][]string) ([]byte, error) {
	if len(headers) > math.MaxUint16 {
		return nil, ErrTooManyHeaders
	}

	var b bytes.Buffer
	_ = writeRequestAndMethod(&b, requestID, "response")
	_ = writeU16(&b, statusCode)
	_ = writeStringWithLen(&b, finalURL)

	// headers
	_ = writeU16(&b, len(headers))
	for name, values := range headers {
		_ = writeStringWithLen(&b, name)
		_ = writeU16(&b, len(values))
		for _, v := range values {
			_ = writeStringWithLen(&b, v)
		}
	}

	return b.Bytes(), nil
}

// -----------------------------------------------------------------------------
// WebSocket Frame Builders
// -----------------------------------------------------------------------------

// buildWebSocketOpenFrame creates a ws_open frame with protocol and extensions.
// The payload is JSON encoded: {"type":"open","protocol":"...","extensions":"..."}
func buildWebSocketOpenFrame(requestID string, protocol, extensions string) ([]byte, error) {
	// Create JSON payload
	openMsg := map[string]interface{}{
		"type":       "open",
		"protocol":   protocol,
		"extensions": extensions,
	}
	payload, err := json.Marshal(openMsg)
	if err != nil {
		return nil, fmt.Errorf("json marshal ws_open: %w", err)
	}

	var b bytes.Buffer
	_ = writeRequestAndMethod(&b, requestID, "ws_open")
	_ = writeU32(&b, len(payload))
	b.Write(payload)
	return b.Bytes(), nil
}

// buildWebSocketMessageFrame creates a ws_message frame.
// messageType: 1 = text, 2 = binary
func buildWebSocketMessageFrame(requestID string, messageType int, data []byte) []byte {
	var b bytes.Buffer
	_ = writeRequestAndMethod(&b, requestID, "ws_message")
	b.WriteByte(byte(messageType))
	_ = writeU32(&b, len(data))
	b.Write(data)
	return b.Bytes()
}

// buildWebSocketCloseFrame creates a ws_close frame.
func buildWebSocketCloseFrame(requestID string, code int, reason string) ([]byte, error) {
	// Create JSON payload
	closeMsg := map[string]interface{}{
		"type":   "close",
		"code":   code,
		"reason": reason,
	}
	payload, err := json.Marshal(closeMsg)
	if err != nil {
		return nil, fmt.Errorf("json marshal ws_close: %w", err)
	}

	var b bytes.Buffer
	_ = writeRequestAndMethod(&b, requestID, "ws_close")
	_ = writeU32(&b, len(payload))
	b.Write(payload)
	return b.Bytes(), nil
}

// buildWebSocketErrorFrame creates a ws_error frame.
func buildWebSocketErrorFrame(requestID string, statusCode int, message string) []byte {
	var b bytes.Buffer
	_ = writeRequestAndMethod(&b, requestID, "ws_error")
	_ = writeU16(&b, statusCode)
	_ = writeStringWithLen(&b, message)
	return b.Bytes()
}
