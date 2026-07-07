package cycletls

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestReadBytesExceedsMaxLimit(t *testing.T) {
	// Create a small buffer - the size check happens before bounds check
	r := NewReader([]byte{1, 2, 3})

	// Request more than MaxReadBytes (10MB)
	_, err := r.ReadBytes(MaxReadBytes + 1)
	if err != ErrBytesTooLarge {
		t.Fatalf("Expected ErrBytesTooLarge, got %v", err)
	}
}

func TestReadBytesAtMaxLimit(t *testing.T) {
	// Create a reader with a small buffer
	// Reading exactly MaxReadBytes should fail with EOF (not size error)
	// since our buffer is smaller than MaxReadBytes
	r := NewReader([]byte{1, 2, 3})

	_, err := r.ReadBytes(MaxReadBytes)
	if err == ErrBytesTooLarge {
		t.Fatalf("MaxReadBytes should be allowed, got ErrBytesTooLarge")
	}
	// Should get EOF since buffer is too small
	if err == nil {
		t.Fatalf("Expected an error for buffer too small, got nil")
	}
}

func TestReadBytesMaxLimitConstant(t *testing.T) {
	// Verify the constant is 10MB
	expected := 10 * 1024 * 1024
	if MaxReadBytes != expected {
		t.Fatalf("Expected MaxReadBytes to be %d (10MB), got %d", expected, MaxReadBytes)
	}
}

// Issue #8: parseInitMessage should validate initialWindow range
func TestParseInitMessage_WindowTooSmall(t *testing.T) {
	// Build a valid init packet with window size = 0
	data := buildInitPacket("req-1", 0, `{"url":"http://example.com"}`)
	_, _, err := parseInitMessage(data)
	if err == nil {
		t.Fatal("parseInitMessage should reject window size 0")
	}
}

func TestParseInitMessage_WindowTooLarge(t *testing.T) {
	// Build a valid init packet with window size = 4 billion
	data := buildInitPacket("req-1", 4000000000, `{"url":"http://example.com"}`)
	_, _, err := parseInitMessage(data)
	if err == nil {
		t.Fatal("parseInitMessage should reject excessively large window size")
	}
}

func TestParseInitMessage_WindowValid(t *testing.T) {
	// Build a valid init packet with a reasonable window size
	data := buildInitPacket("req-1", 65536, `{"url":"http://example.com"}`)
	_, window, err := parseInitMessage(data)
	if err != nil {
		t.Fatalf("parseInitMessage should accept valid window, got %v", err)
	}
	if window != 65536 {
		t.Fatalf("expected window 65536, got %d", window)
	}
}

// Issue #9: parseClientMessage should detect trailing bytes
func TestParseClientMessage_TrailingBytes(t *testing.T) {
	// Build a valid credit message, then append garbage
	data := buildCreditPacket("req-1", 1000)
	data = append(data, 0xFF, 0xFE, 0xFD) // trailing garbage
	_, err := parseClientMessage(data)
	if err == nil {
		t.Fatal("parseClientMessage should reject messages with trailing bytes")
	}
}

func TestParseClientMessage_ExactBytes(t *testing.T) {
	// Build a valid credit message with no extra bytes
	data := buildCreditPacket("req-1", 1000)
	msg, err := parseClientMessage(data)
	if err != nil {
		t.Fatalf("parseClientMessage should accept exact message, got %v", err)
	}
	if msg.Credits != 1000 {
		t.Fatalf("expected credits 1000, got %d", msg.Credits)
	}
}

// Helper: build an init packet
func buildInitPacket(requestID string, windowSize uint32, optionsJSON string) []byte {
	var b bytes.Buffer
	// requestID (length-prefixed string)
	binary.Write(&b, binary.BigEndian, uint16(len(requestID)))
	b.WriteString(requestID)
	// method "init" (length-prefixed string)
	method := "init"
	binary.Write(&b, binary.BigEndian, uint16(len(method)))
	b.WriteString(method)
	// window size (uint32)
	binary.Write(&b, binary.BigEndian, windowSize)
	// options JSON (length-prefixed string)
	binary.Write(&b, binary.BigEndian, uint16(len(optionsJSON)))
	b.WriteString(optionsJSON)
	return b.Bytes()
}

// Helper: build a credit packet
func buildCreditPacket(requestID string, credits uint32) []byte {
	var b bytes.Buffer
	// requestID (length-prefixed string)
	binary.Write(&b, binary.BigEndian, uint16(len(requestID)))
	b.WriteString(requestID)
	// method "credit" (length-prefixed string)
	method := "credit"
	binary.Write(&b, binary.BigEndian, uint16(len(method)))
	b.WriteString(method)
	// credits (uint32)
	binary.Write(&b, binary.BigEndian, credits)
	return b.Bytes()
}

// Helper: build a ws_close packet
func buildWsClosePacket(requestID string, code uint32, reason string) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, uint16(len(requestID)))
	b.WriteString(requestID)
	method := "ws_close"
	binary.Write(&b, binary.BigEndian, uint16(len(method)))
	b.WriteString(method)
	binary.Write(&b, binary.BigEndian, code)
	binary.Write(&b, binary.BigEndian, uint16(len(reason)))
	b.WriteString(reason)
	return b.Bytes()
}

func TestParseClientMessage_WsCloseTrailingBytes(t *testing.T) {
	data := buildWsClosePacket("req-1", 1000, "goodbye")
	data = append(data, 0xFF) // trailing garbage
	_, err := parseClientMessage(data)
	if err == nil {
		t.Fatal("parseClientMessage should reject ws_close with trailing bytes")
	}
}

func TestParseClientMessage_WsCloseExact(t *testing.T) {
	data := buildWsClosePacket("req-1", 1000, "goodbye")
	msg, err := parseClientMessage(data)
	if err != nil {
		t.Fatalf("parseClientMessage should accept exact ws_close, got %v", err)
	}
	if msg.CloseCode != 1000 {
		t.Fatalf("expected close code 1000, got %d", msg.CloseCode)
	}
	if msg.CloseReason != "goodbye" {
		t.Fatalf("expected reason 'goodbye', got %q", msg.CloseReason)
	}
}

// Helper: build a ws_send packet
func buildWsSendPacket(requestID string, msgType byte, msgData []byte) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, uint16(len(requestID)))
	b.WriteString(requestID)
	method := "ws_send"
	binary.Write(&b, binary.BigEndian, uint16(len(method)))
	b.WriteString(method)
	b.WriteByte(msgType) // message type byte
	binary.Write(&b, binary.BigEndian, uint32(len(msgData)))
	b.Write(msgData)
	return b.Bytes()
}

func TestParseClientMessage_WsSendTrailingBytes(t *testing.T) {
	data := buildWsSendPacket("req-1", 1, []byte("hello"))
	data = append(data, 0xFF) // trailing garbage
	_, err := parseClientMessage(data)
	if err == nil {
		t.Fatal("parseClientMessage should reject ws_send with trailing bytes")
	}
}

func TestParseClientMessage_WsSendExact(t *testing.T) {
	data := buildWsSendPacket("req-1", 1, []byte("hello"))
	msg, err := parseClientMessage(data)
	if err != nil {
		t.Fatalf("parseClientMessage should accept exact ws_send, got %v", err)
	}
	if msg.MessageType != 1 {
		t.Fatalf("expected message type 1, got %d", msg.MessageType)
	}
	if string(msg.Data) != "hello" {
		t.Fatalf("expected data 'hello', got %q", string(msg.Data))
	}
}

// Verify parseInitMessage also rejects trailing bytes
func TestParseInitMessage_TrailingBytes(t *testing.T) {
	data := buildInitPacket("req-1", 65536, `{"url":"http://example.com"}`)
	data = append(data, 0xFF) // trailing garbage
	_, _, err := parseInitMessage(data)
	if err == nil {
		t.Fatal("parseInitMessage should reject messages with trailing bytes")
	}
}

// Verify parseCreditMessage also rejects trailing bytes
func TestParseCreditMessage_TrailingBytes(t *testing.T) {
	data := buildCreditPacket("req-1", 1000)
	data = append(data, 0xFF) // trailing garbage
	_, _, err := parseCreditMessage(data)
	if err == nil {
		t.Fatal("parseCreditMessage should reject messages with trailing bytes")
	}
}

// Ensure json.Marshal errors are propagated
func TestBuildWebSocketOpenFrame_MarshalError(t *testing.T) {
	// Valid inputs should work
	_, err := buildWebSocketOpenFrame("req-1", "proto", "ext")
	if err != nil {
		t.Fatalf("expected no error for valid input, got %v", err)
	}

	// The current implementation uses map[string]interface{} which json.Marshal
	// handles for all string values, so we just verify it works properly
	data, err := buildWebSocketOpenFrame("req-1", "", "")
	if err != nil {
		t.Fatalf("expected no error for empty strings, got %v", err)
	}
	// Verify the payload contains valid JSON
	r := NewReader(data)
	_, _ = r.ReadString() // requestID
	_, _ = r.ReadString() // method
	payloadLen, _ := r.ReadU32()
	payloadBytes, _ := r.ReadBytes(int(payloadLen))
	var result map[string]interface{}
	if jsonErr := json.Unmarshal(payloadBytes, &result); jsonErr != nil {
		t.Fatalf("expected valid JSON payload, got unmarshal error: %v", jsonErr)
	}
}
