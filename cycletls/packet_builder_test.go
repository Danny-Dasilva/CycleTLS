package cycletls

import (
	"bytes"
	"testing"
)

// Issue #2: writeU16 integer overflow - values > 65535 should error
func TestWriteU16_Overflow(t *testing.T) {
	var b bytes.Buffer
	err := writeU16(&b, 70000) // exceeds uint16 max (65535)
	if err == nil {
		t.Fatal("writeU16 should return error for values > 65535")
	}
}

func TestWriteU16_MaxValid(t *testing.T) {
	var b bytes.Buffer
	err := writeU16(&b, 65535) // max uint16
	if err != nil {
		t.Fatalf("writeU16 should succeed for 65535, got %v", err)
	}
	if b.Len() != 2 {
		t.Fatalf("expected 2 bytes, got %d", b.Len())
	}
}

func TestWriteU16_Zero(t *testing.T) {
	var b bytes.Buffer
	err := writeU16(&b, 0)
	if err != nil {
		t.Fatalf("writeU16 should succeed for 0, got %v", err)
	}
	data := b.Bytes()
	if data[0] != 0 || data[1] != 0 {
		t.Fatalf("expected [0, 0], got %v", data)
	}
}

func TestWriteU16_Negative(t *testing.T) {
	var b bytes.Buffer
	err := writeU16(&b, -1) // negative values should error
	if err == nil {
		t.Fatal("writeU16 should return error for negative values")
	}
}

// Issue #2: writeU32 integer overflow
func TestWriteU32_Overflow(t *testing.T) {
	var b bytes.Buffer
	err := writeU32(&b, int(1<<32)) // exceeds uint32 max
	if err == nil {
		t.Fatal("writeU32 should return error for values > 4294967295")
	}
}

func TestWriteU32_MaxValid(t *testing.T) {
	var b bytes.Buffer
	err := writeU32(&b, int(1<<32-1)) // max uint32
	if err != nil {
		t.Fatalf("writeU32 should succeed for max uint32, got %v", err)
	}
	if b.Len() != 4 {
		t.Fatalf("expected 4 bytes, got %d", b.Len())
	}
}

func TestWriteU32_Negative(t *testing.T) {
	var b bytes.Buffer
	err := writeU32(&b, -1)
	if err == nil {
		t.Fatal("writeU32 should return error for negative values")
	}
}

// Issue #3: buildResponseFrame header map bounds check
func TestBuildResponseFrame_TooManyHeaders(t *testing.T) {
	headers := make(map[string][]string)
	// Create more than 65535 headers
	for i := 0; i < 65536; i++ {
		key := "Header-" + string(rune(i/256+'A')) + string(rune(i%256+'A'))
		headers[key] = []string{"value"}
	}
	_, err := buildResponseFrame("req-1", 200, "http://example.com", headers)
	if err == nil {
		t.Fatal("buildResponseFrame should return error when headers exceed 65535")
	}
}

func TestBuildResponseFrame_MaxHeaders(t *testing.T) {
	// A small map should work fine
	headers := map[string][]string{
		"Content-Type": {"text/html"},
	}
	_, err := buildResponseFrame("req-1", 200, "http://example.com", headers)
	if err != nil {
		t.Fatalf("buildResponseFrame should succeed with small headers, got %v", err)
	}
}

// Issue #5: buildWebSocketOpenFrame should handle json.Marshal error
func TestBuildWebSocketOpenFrame_Success(t *testing.T) {
	data, err := buildWebSocketOpenFrame("req-1", "proto", "ext")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty frame")
	}
}

// Issue #5: buildWebSocketCloseFrame should handle json.Marshal error
func TestBuildWebSocketCloseFrame_Success(t *testing.T) {
	data, err := buildWebSocketCloseFrame("req-1", 1000, "normal")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty frame")
	}
}

// M1: an over-long length-prefixed field must surface an error and emit NO frame,
// instead of silently truncating (which desyncs the reader for every later frame).
func TestBuildResponseFrame_LengthOverflow(t *testing.T) {
	big := string(bytes.Repeat([]byte("x"), 65536)) // >= 65536 overflows a uint16 length

	cases := []struct {
		name    string
		url     string
		headers map[string][]string
	}{
		{"header value", "http://example.com", map[string][]string{"X-Big": {big}}},
		{"header name", "http://example.com", map[string][]string{big: {"v"}}},
		{"final url", big, map[string][]string{"X": {"v"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frame, err := buildResponseFrame("req-1", 200, tc.url, tc.headers)
			if err == nil {
				t.Fatalf("expected error for over-long %s (>= 65536 bytes)", tc.name)
			}
			if frame != nil {
				t.Fatalf("expected nil frame on overflow for %s, got %d bytes (truncated frame would desync reader)", tc.name, len(frame))
			}
		})
	}
}

// M1 guard: the byte layout of a normal response frame must not change. If this
// fails the wire format diverged from what the TypeScript reader expects.
func TestBuildResponseFrame_WireFormatStable(t *testing.T) {
	frame, err := buildResponseFrame("r", 200, "u", map[string][]string{"A": {"b"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []byte{
		0x00, 0x01, 'r', // requestID "r"
		0x00, 0x08, 'r', 'e', 's', 'p', 'o', 'n', 's', 'e', // method "response"
		0x00, 0xC8, // status 200
		0x00, 0x01, 'u', // finalURL "u"
		0x00, 0x01, // header count 1
		0x00, 0x01, 'A', // header name "A"
		0x00, 0x01, // value count 1
		0x00, 0x01, 'b', // value "b"
	}
	if !bytes.Equal(frame, expected) {
		t.Fatalf("wire format changed:\n got  %v\n want %v", frame, expected)
	}
}

// M1 round-trip: a normal frame with multiple headers/values decodes back to the
// exact same fields, proving the length prefixes stay in sync.
func TestBuildResponseFrame_RoundTrip(t *testing.T) {
	reqID := "req-42"
	status := 404
	finalURL := "https://example.com/path?q=1"
	headers := map[string][]string{
		"Content-Type": {"text/html; charset=utf-8"},
		"Set-Cookie":   {"a=1", "b=2"},
	}

	frame, err := buildResponseFrame(reqID, status, finalURL, headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	off := 0
	readStr := func() string {
		n := int(frame[off])<<8 | int(frame[off+1])
		off += 2
		s := string(frame[off : off+n])
		off += n
		return s
	}
	readU16 := func() int {
		v := int(frame[off])<<8 | int(frame[off+1])
		off += 2
		return v
	}

	if got := readStr(); got != reqID {
		t.Fatalf("requestID = %q, want %q", got, reqID)
	}
	if got := readStr(); got != "response" {
		t.Fatalf("method = %q, want %q", got, "response")
	}
	if got := readU16(); got != status {
		t.Fatalf("status = %d, want %d", got, status)
	}
	if got := readStr(); got != finalURL {
		t.Fatalf("finalURL = %q, want %q", got, finalURL)
	}

	decoded := make(map[string][]string)
	hc := readU16()
	for i := 0; i < hc; i++ {
		name := readStr()
		vc := readU16()
		vals := make([]string, vc)
		for j := 0; j < vc; j++ {
			vals[j] = readStr()
		}
		decoded[name] = vals
	}
	if off != len(frame) {
		t.Fatalf("decoded %d of %d bytes: length prefixes desynced", off, len(frame))
	}
	if len(decoded) != len(headers) {
		t.Fatalf("decoded %d headers, want %d", len(decoded), len(headers))
	}
	for name, want := range headers {
		got := decoded[name]
		if len(got) != len(want) {
			t.Fatalf("header %q: %d values, want %d", name, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("header %q[%d] = %q, want %q", name, i, got[i], want[i])
			}
		}
	}
}

// M1: buildErrorFrame cannot return an error, so an over-long message must be
// clamped to keep its length prefix consistent rather than writing nothing.
func TestBuildErrorFrame_ClampsOverlongMessage(t *testing.T) {
	msg := string(bytes.Repeat([]byte("e"), 70000)) // > 65535
	frame := buildErrorFrame("req-1", 500, msg)

	off := 0
	readStr := func() string {
		n := int(frame[off])<<8 | int(frame[off+1])
		off += 2
		s := string(frame[off : off+n])
		off += n
		return s
	}
	readU16 := func() int {
		v := int(frame[off])<<8 | int(frame[off+1])
		off += 2
		return v
	}
	if got := readStr(); got != "req-1" {
		t.Fatalf("requestID = %q", got)
	}
	if got := readStr(); got != "error" {
		t.Fatalf("method = %q", got)
	}
	if got := readU16(); got != 500 {
		t.Fatalf("status = %d", got)
	}
	message := readStr()
	if len(message) != 65535 {
		t.Fatalf("message length = %d, want clamped to 65535", len(message))
	}
	if off != len(frame) {
		t.Fatalf("decoded %d of %d bytes: clamp left the frame desynced", off, len(frame))
	}
}
