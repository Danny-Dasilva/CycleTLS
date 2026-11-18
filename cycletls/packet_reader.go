package cycletls

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// MaxStringLen defines the maximum allowed length for a string.
// This protects against malformed or malicious packets.
// Set to max uint16 value (65535) since the protocol uses 2-byte length encoding.
// Request options that do not fit are sent in an "init2" packet, whose options
// field has a 4-byte length (see parseInitMessage and MaxInitOptionsLen).
const MaxStringLen = 65535

// ErrStringTooLarge is returned when a string length exceeds MaxStringLen.
var ErrStringTooLarge = errors.New("packet: string length exceeds limit")

// Init packet method names. Both carry the same fields; they differ only in
// the width of the length prefix of the options JSON.
const (
	// initMethod is the original init packet: options JSON with a uint16 length.
	initMethod = "init"
	// initMethodLong is the large init packet: options JSON with a uint32 length.
	initMethodLong = "init2"
)

// MaxInitOptionsLen is the largest options JSON accepted in an "init2" packet
// (32 MiB). The options JSON holds the whole request, body included, so this
// bounds the request body: about 32 MiB of text, or about 24 MiB of binary
// data, which travels base64 encoded.
const MaxInitOptionsLen = 32 * 1024 * 1024

// ErrInitOptionsTooLarge is returned when the options JSON of an "init2"
// packet is longer than MaxInitOptionsLen.
var ErrInitOptionsTooLarge = errors.New("packet: init options exceed 32MB limit")

// ErrNegativeLength is returned when a negative length is encountered.
var ErrNegativeLength = errors.New("packet: negative length")

// MaxReadBytes defines the maximum allowed size for ReadBytes (10 MB).
// This prevents unbounded memory allocation from malformed packets.
const MaxReadBytes = 10 * 1024 * 1024

// MinInitialWindow is the minimum allowed initial window size (1 KB).
const MinInitialWindow uint32 = 1024

// MaxInitialWindow is the maximum allowed initial window size (100 MB).
const MaxInitialWindow uint32 = 100 * 1024 * 1024

// ErrBytesTooLarge is returned when a ReadBytes length exceeds MaxReadBytes.
var ErrBytesTooLarge = errors.New("packet: read bytes length exceeds 10MB limit")

// ErrWindowTooSmall is returned when the initial window size is below the minimum.
var ErrWindowTooSmall = errors.New("packet: initial window size too small (minimum 1KB)")

// ErrWindowTooLarge is returned when the initial window size exceeds the maximum.
var ErrWindowTooLarge = errors.New("packet: initial window size too large (maximum 100MB)")

// ErrTrailingBytes is returned when a parsed message has unconsumed trailing bytes.
var ErrTrailingBytes = errors.New("packet: unexpected trailing bytes")

// Reader allows sequential reading of typed values from a binary buffer.
//
// IMPORTANT:
//   - Reader is NOT thread-safe.
//   - It must be used by ONE goroutine at a time.
//   - Any concurrent use will cause data races.
//
// Reader never modifies the underlying buffer, but maintains a mutable internal index.
type Reader struct {
	data []byte
	pos  int
}

// NewReader creates a Reader positioned at the beginning of the buffer.
//
// The passed buffer is NOT copied:
//   - it must remain valid throughout the Reader's lifetime
//   - it must not be modified during reading
func NewReader(data []byte) *Reader {
	return &Reader{data: data}
}

// Remaining returns the number of bytes still readable.
func (r *Reader) Remaining() int {
	return len(r.data) - r.pos
}

// ReadU16 reads a 16-bit unsigned integer in big-endian format.
func (r *Reader) ReadU16() (uint16, error) {
	const size = 2

	if (r.pos + size) > len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}

	v := binary.BigEndian.Uint16(r.data[r.pos : r.pos+size])
	r.pos += size
	return v, nil
}

// ReadU32 reads a 32-bit unsigned integer in big-endian format.
func (r *Reader) ReadU32() (uint32, error) {
	const size = 4

	if (r.pos + size) > len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}

	v := binary.BigEndian.Uint32(r.data[r.pos : r.pos+size])
	r.pos += size
	return v, nil
}

// ReadString reads a string encoded as:
//
//	uint16 length (big-endian)
//	followed by <length> bytes
//
// The returned string is COPIED and does not reference the internal buffer.
//
// Possible errors:
//   - io.ErrUnexpectedEOF
//   - ErrStringTooLarge
func (r *Reader) ReadString() (string, error) {
	length, err := r.ReadU16()
	if err != nil {
		return "", err
	}

	if length > MaxStringLen {
		return "", ErrStringTooLarge
	}

	n := int(length)

	if n == 0 {
		return "", nil
	}

	if (r.pos + n) > len(r.data) {
		return "", io.ErrUnexpectedEOF
	}

	s := string(r.data[r.pos : r.pos+n])
	r.pos += n
	return s, nil
}

// readLongBytes reads a field encoded as:
//
//	uint32 length (big-endian)
//	followed by <length> bytes
//
// and returns a slice that ALIASES the Reader's buffer (no copy). max bounds
// the declared length; a larger one fails with tooLarge before anything is read.
//
// Possible errors:
//   - io.ErrUnexpectedEOF
//   - tooLarge
func (r *Reader) readLongBytes(max int, tooLarge error) ([]byte, error) {
	length, err := r.ReadU32()
	if err != nil {
		return nil, err
	}

	if uint64(length) > uint64(max) {
		return nil, tooLarge
	}

	n := int(length)

	if n > r.Remaining() {
		return nil, io.ErrUnexpectedEOF
	}

	out := r.data[r.pos : r.pos+n]
	r.pos += n
	return out, nil
}

// ReadBytes reads n bytes and returns a COPY of the buffer.
//
// This method avoids unnecessary conversions to string.
func (r *Reader) ReadBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrNegativeLength
	}

	if n == 0 {
		return []byte{}, nil
	}

	if n > MaxReadBytes {
		return nil, ErrBytesTooLarge
	}

	if (r.pos + n) > len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}

	out := make([]byte, n)
	copy(out, r.data[r.pos:r.pos+n])
	r.pos += n
	return out, nil
}

// -----------------------------------------------------------------------------
// Message parsing
// -----------------------------------------------------------------------------

// parseInitMessage parses the first packet of a request. Two layouts are
// accepted, told apart by the method name:
//
//	"init":  requestID (u16 len + bytes), "init"  (u16 len + bytes),
//	         initialWindow (u32), options JSON (u16 len + bytes)
//	"init2": requestID (u16 len + bytes), "init2" (u16 len + bytes),
//	         initialWindow (u32), options JSON (u32 len + bytes)
//
// "init" caps the options JSON, and with it the request body, at 65535 bytes.
// "init2" raises the cap to MaxInitOptionsLen. Clients older than 3.0.1 only
// send "init", so it has to keep parsing.
//
// Returns the request, initial window size, and any error.
func parseInitMessage(data []byte) (cycleTLSRequest, uint32, error) {
	r := NewReader(data)

	requestID, err := r.ReadString()
	if err != nil {
		return cycleTLSRequest{}, 0, err
	}

	method, err := r.ReadString()
	if err != nil {
		return cycleTLSRequest{}, 0, err
	}
	if method != initMethod && method != initMethodLong {
		return cycleTLSRequest{}, 0, fmt.Errorf("unexpected method %q", method)
	}

	initialWindow, err := r.ReadU32()
	if err != nil {
		return cycleTLSRequest{}, 0, err
	}

	// Validate initial window size range
	if initialWindow < MinInitialWindow {
		return cycleTLSRequest{}, 0, ErrWindowTooSmall
	}
	if initialWindow > MaxInitialWindow {
		return cycleTLSRequest{}, 0, ErrWindowTooLarge
	}

	// "init" carries the options JSON with a uint16 length, "init2" with a uint32.
	var optionsJSON []byte
	if method == initMethodLong {
		optionsJSON, err = r.readLongBytes(MaxInitOptionsLen, ErrInitOptionsTooLarge)
	} else {
		var s string
		s, err = r.ReadString()
		optionsJSON = []byte(s)
	}
	if err != nil {
		return cycleTLSRequest{}, 0, err
	}

	// Check for trailing bytes
	if r.Remaining() > 0 {
		return cycleTLSRequest{}, 0, ErrTrailingBytes
	}

	var opts Options
	if err := json.Unmarshal(optionsJSON, &opts); err != nil {
		return cycleTLSRequest{}, 0, err
	}

	return cycleTLSRequest{
		RequestID: requestID,
		Options:   opts,
	}, initialWindow, nil
}

// parseCreditMessage parses a "credit" packet from the client.
// Returns the request ID, credit amount, and any error.
func parseCreditMessage(data []byte) (string, uint32, error) {
	r := NewReader(data)

	requestID, err := r.ReadString()
	if err != nil {
		return "", 0, err
	}

	method, err := r.ReadString()
	if err != nil {
		return "", 0, err
	}
	if method != "credit" {
		return "", 0, fmt.Errorf("unexpected method %q", method)
	}

	credits, err := r.ReadU32()
	if err != nil {
		return "", 0, err
	}

	// Check for trailing bytes
	if r.Remaining() > 0 {
		return "", 0, ErrTrailingBytes
	}

	return requestID, credits, nil
}

// -----------------------------------------------------------------------------
// WebSocket command parsing (V2 protocol)
// -----------------------------------------------------------------------------

// ClientMessage represents a parsed message from the TypeScript client.
type ClientMessage struct {
	RequestID string
	Method    string
	// For credit messages
	Credits uint32
	// For ws_send messages
	MessageType int // 1 = text, 2 = binary
	Data        []byte
	// For ws_close messages
	CloseCode   int
	CloseReason string
}

// parseClientMessage parses any client message and returns a ClientMessage struct.
// Supported methods: "credit", "ws_send", "ws_close"
func parseClientMessage(data []byte) (ClientMessage, error) {
	r := NewReader(data)

	requestID, err := r.ReadString()
	if err != nil {
		return ClientMessage{}, err
	}

	method, err := r.ReadString()
	if err != nil {
		return ClientMessage{}, err
	}

	msg := ClientMessage{
		RequestID: requestID,
		Method:    method,
	}

	switch method {
	case "credit":
		credits, err := r.ReadU32()
		if err != nil {
			return ClientMessage{}, err
		}
		msg.Credits = credits

	case "ws_send":
		// Format: messageType (1 byte) + data length (4 bytes) + data
		msgType, err := r.ReadBytes(1)
		if err != nil {
			return ClientMessage{}, err
		}
		msg.MessageType = int(msgType[0])

		dataLen, err := r.ReadU32()
		if err != nil {
			return ClientMessage{}, err
		}

		msgData, err := r.ReadBytes(int(dataLen))
		if err != nil {
			return ClientMessage{}, err
		}
		msg.Data = msgData

	case "ws_close":
		// Format: code (4 bytes) + reason (length-prefixed string)
		code, err := r.ReadU32()
		if err != nil {
			return ClientMessage{}, err
		}
		msg.CloseCode = int(code)

		reason, err := r.ReadString()
		if err != nil {
			return ClientMessage{}, err
		}
		msg.CloseReason = reason

	default:
		return ClientMessage{}, fmt.Errorf("unknown method %q", method)
	}

	// Check for trailing bytes
	if r.Remaining() > 0 {
		return ClientMessage{}, ErrTrailingBytes
	}

	return msg, nil
}
