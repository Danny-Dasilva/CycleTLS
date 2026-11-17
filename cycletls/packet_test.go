package cycletls

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

// ============================================================================
// Packet Builder Tests
// ============================================================================

func TestBuildErrorFrame(t *testing.T) {
	requestID := "test-request-123"
	statusCode := 404
	message := "Not Found"

	frame := buildErrorFrame(requestID, statusCode, message)

	// Parse the frame
	r := NewReader(frame)

	// Read request ID
	id, err := r.ReadString()
	if err != nil {
		t.Fatalf("Failed to read request ID: %v", err)
	}
	if id != requestID {
		t.Fatalf("Expected request ID %s, got %s", requestID, id)
	}

	// Read method
	method, err := r.ReadString()
	if err != nil {
		t.Fatalf("Failed to read method: %v", err)
	}
	if method != "error" {
		t.Fatalf("Expected method 'error', got %s", method)
	}

	// Read status code (2 bytes big-endian)
	code, err := r.ReadU16()
	if err != nil {
		t.Fatalf("Failed to read status code: %v", err)
	}
	if int(code) != statusCode {
		t.Fatalf("Expected status code %d, got %d", statusCode, code)
	}

	// Read message
	msg, err := r.ReadString()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}
	if msg != message {
		t.Fatalf("Expected message %s, got %s", message, msg)
	}
}

func TestBuildEndFrame(t *testing.T) {
	requestID := "end-request-456"

	frame := buildEndFrame(requestID)

	r := NewReader(frame)

	id, err := r.ReadString()
	if err != nil {
		t.Fatalf("Failed to read request ID: %v", err)
	}
	if id != requestID {
		t.Fatalf("Expected request ID %s, got %s", requestID, id)
	}

	method, err := r.ReadString()
	if err != nil {
		t.Fatalf("Failed to read method: %v", err)
	}
	if method != "end" {
		t.Fatalf("Expected method 'end', got %s", method)
	}

	// Should be at end of frame
	if r.Remaining() != 0 {
		t.Fatalf("Expected no remaining bytes, got %d", r.Remaining())
	}
}

func TestBuildDataFrame(t *testing.T) {
	requestID := "data-request-789"
	body := []byte("Hello, World! This is test data.")

	frame := buildDataFrame(requestID, body)

	r := NewReader(frame)

	id, err := r.ReadString()
	if err != nil {
		t.Fatalf("Failed to read request ID: %v", err)
	}
	if id != requestID {
		t.Fatalf("Expected request ID %s, got %s", requestID, id)
	}

	method, err := r.ReadString()
	if err != nil {
		t.Fatalf("Failed to read method: %v", err)
	}
	if method != "data" {
		t.Fatalf("Expected method 'data', got %s", method)
	}

	// Read body length (4 bytes big-endian)
	length, err := r.ReadU32()
	if err != nil {
		t.Fatalf("Failed to read body length: %v", err)
	}
	if int(length) != len(body) {
		t.Fatalf("Expected body length %d, got %d", len(body), length)
	}

	// Read body
	readBody, err := r.ReadBytes(int(length))
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}
	if !bytes.Equal(readBody, body) {
		t.Fatalf("Expected body %v, got %v", body, readBody)
	}
}

func TestBuildResponseFrame(t *testing.T) {
	requestID := "response-request-001"
	statusCode := 200
	finalURL := "https://example.com/final"
	headers := map[string][]string{
		"Content-Type": {"application/json"},
		"Set-Cookie":   {"session=abc123", "user=test"},
	}

	frame, err := buildResponseFrame(requestID, statusCode, finalURL, headers)
	if err != nil {
		t.Fatalf("buildResponseFrame returned error: %v", err)
	}

	r := NewReader(frame)

	// Read request ID
	id, err := r.ReadString()
	if err != nil {
		t.Fatalf("Failed to read request ID: %v", err)
	}
	if id != requestID {
		t.Fatalf("Expected request ID %s, got %s", requestID, id)
	}

	// Read method
	method, err := r.ReadString()
	if err != nil {
		t.Fatalf("Failed to read method: %v", err)
	}
	if method != "response" {
		t.Fatalf("Expected method 'response', got %s", method)
	}

	// Read status code
	code, err := r.ReadU16()
	if err != nil {
		t.Fatalf("Failed to read status code: %v", err)
	}
	if int(code) != statusCode {
		t.Fatalf("Expected status code %d, got %d", statusCode, code)
	}

	// Read final URL
	url, err := r.ReadString()
	if err != nil {
		t.Fatalf("Failed to read final URL: %v", err)
	}
	if url != finalURL {
		t.Fatalf("Expected final URL %s, got %s", finalURL, url)
	}

	// Read header count
	headerCount, err := r.ReadU16()
	if err != nil {
		t.Fatalf("Failed to read header count: %v", err)
	}
	if int(headerCount) != len(headers) {
		t.Fatalf("Expected %d headers, got %d", len(headers), headerCount)
	}
}

// ============================================================================
// Packet Reader Tests
// ============================================================================

func TestReaderReadU16(t *testing.T) {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 0x1234)

	r := NewReader(data)
	val, err := r.ReadU16()
	if err != nil {
		t.Fatalf("ReadU16 failed: %v", err)
	}
	if val != 0x1234 {
		t.Fatalf("Expected 0x1234, got 0x%x", val)
	}

	// Reading again should fail
	_, err = r.ReadU16()
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("Expected ErrUnexpectedEOF, got %v", err)
	}
}

func TestReaderReadU32(t *testing.T) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, 0x12345678)

	r := NewReader(data)
	val, err := r.ReadU32()
	if err != nil {
		t.Fatalf("ReadU32 failed: %v", err)
	}
	if val != 0x12345678 {
		t.Fatalf("Expected 0x12345678, got 0x%x", val)
	}
}

func TestReaderReadString(t *testing.T) {
	testStr := "Hello, World!"
	data := make([]byte, 2+len(testStr))
	binary.BigEndian.PutUint16(data, uint16(len(testStr)))
	copy(data[2:], testStr)

	r := NewReader(data)
	str, err := r.ReadString()
	if err != nil {
		t.Fatalf("ReadString failed: %v", err)
	}
	if str != testStr {
		t.Fatalf("Expected %s, got %s", testStr, str)
	}
}

func TestReaderReadStringTooLarge(t *testing.T) {
	maxUint16 := ^uint16(0)
	if MaxStringLen >= int(maxUint16) {
		t.Skip("MaxStringLen equals uint16 max; overflow case unreachable with 2-byte length")
	}

	// Create data with length > MaxStringLen
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, maxUint16)

	r := NewReader(data)
	_, err := r.ReadString()
	if err != ErrStringTooLarge {
		t.Fatalf("Expected ErrStringTooLarge, got %v", err)
	}
}

func TestReaderReadEmptyString(t *testing.T) {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, 0) // length = 0

	r := NewReader(data)
	str, err := r.ReadString()
	if err != nil {
		t.Fatalf("ReadString failed: %v", err)
	}
	if str != "" {
		t.Fatalf("Expected empty string, got %s", str)
	}
}

func TestReaderReadBytes(t *testing.T) {
	original := []byte{1, 2, 3, 4, 5}
	r := NewReader(original)

	// Read 3 bytes
	result, err := r.ReadBytes(3)
	if err != nil {
		t.Fatalf("ReadBytes failed: %v", err)
	}
	if !bytes.Equal(result, []byte{1, 2, 3}) {
		t.Fatalf("Expected [1,2,3], got %v", result)
	}

	// Remaining should be 2
	if r.Remaining() != 2 {
		t.Fatalf("Expected 2 remaining, got %d", r.Remaining())
	}

	// Read rest
	result, err = r.ReadBytes(2)
	if err != nil {
		t.Fatalf("ReadBytes failed: %v", err)
	}
	if !bytes.Equal(result, []byte{4, 5}) {
		t.Fatalf("Expected [4,5], got %v", result)
	}
}

func TestReaderReadBytesNegative(t *testing.T) {
	r := NewReader([]byte{1, 2, 3})
	_, err := r.ReadBytes(-1)
	if err != ErrNegativeLength {
		t.Fatalf("Expected ErrNegativeLength, got %v", err)
	}
}

func TestReaderReadBytesZero(t *testing.T) {
	r := NewReader([]byte{1, 2, 3})
	result, err := r.ReadBytes(0)
	if err != nil {
		t.Fatalf("ReadBytes(0) failed: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("Expected empty slice, got %v", result)
	}
}

func TestReaderRemaining(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	r := NewReader(data)

	if r.Remaining() != 5 {
		t.Fatalf("Expected 5 remaining, got %d", r.Remaining())
	}

	r.ReadBytes(2)
	if r.Remaining() != 3 {
		t.Fatalf("Expected 3 remaining, got %d", r.Remaining())
	}
}

func TestParseInitMessage(t *testing.T) {
	// Build an init message manually
	var buf bytes.Buffer

	requestID := "init-test-123"
	method := "init"
	initialWindow := uint32(65536)
	options := `{"url":"https://example.com","method":"GET"}`

	// Request ID
	buf.WriteByte(byte(len(requestID) >> 8))
	buf.WriteByte(byte(len(requestID)))
	buf.WriteString(requestID)

	// Method
	buf.WriteByte(byte(len(method) >> 8))
	buf.WriteByte(byte(len(method)))
	buf.WriteString(method)

	// Initial window (4 bytes)
	buf.WriteByte(byte(initialWindow >> 24))
	buf.WriteByte(byte(initialWindow >> 16))
	buf.WriteByte(byte(initialWindow >> 8))
	buf.WriteByte(byte(initialWindow))

	// Options JSON
	buf.WriteByte(byte(len(options) >> 8))
	buf.WriteByte(byte(len(options)))
	buf.WriteString(options)

	req, window, err := parseInitMessage(buf.Bytes())
	if err != nil {
		t.Fatalf("parseInitMessage failed: %v", err)
	}

	if req.RequestID != requestID {
		t.Fatalf("Expected request ID %s, got %s", requestID, req.RequestID)
	}
	if window != initialWindow {
		t.Fatalf("Expected window %d, got %d", initialWindow, window)
	}
	if req.Options.URL != "https://example.com" {
		t.Fatalf("Expected URL https://example.com, got %s", req.Options.URL)
	}
}

func TestParseCreditMessage(t *testing.T) {
	// Build a credit message manually
	var buf bytes.Buffer

	requestID := "credit-test-456"
	method := "credit"
	credits := uint32(32768)

	// Request ID
	buf.WriteByte(byte(len(requestID) >> 8))
	buf.WriteByte(byte(len(requestID)))
	buf.WriteString(requestID)

	// Method
	buf.WriteByte(byte(len(method) >> 8))
	buf.WriteByte(byte(len(method)))
	buf.WriteString(method)

	// Credits (4 bytes)
	buf.WriteByte(byte(credits >> 24))
	buf.WriteByte(byte(credits >> 16))
	buf.WriteByte(byte(credits >> 8))
	buf.WriteByte(byte(credits))

	id, creditAmount, err := parseCreditMessage(buf.Bytes())
	if err != nil {
		t.Fatalf("parseCreditMessage failed: %v", err)
	}

	if id != requestID {
		t.Fatalf("Expected request ID %s, got %s", requestID, id)
	}
	if creditAmount != credits {
		t.Fatalf("Expected credits %d, got %d", credits, creditAmount)
	}
}

func TestParseCreditMessageWrongMethod(t *testing.T) {
	var buf bytes.Buffer

	requestID := "wrong-method-test"
	method := "wrong"

	buf.WriteByte(byte(len(requestID) >> 8))
	buf.WriteByte(byte(len(requestID)))
	buf.WriteString(requestID)

	buf.WriteByte(byte(len(method) >> 8))
	buf.WriteByte(byte(len(method)))
	buf.WriteString(method)

	// Credits
	buf.Write([]byte{0, 0, 0, 100})

	_, _, err := parseCreditMessage(buf.Bytes())
	if err == nil {
		t.Fatal("Expected error for wrong method")
	}
}
