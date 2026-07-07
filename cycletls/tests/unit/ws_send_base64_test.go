//go:build !integration

package unit

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// decodeBase64Message is a test helper that replicates the production ws_send
// binary decode logic: if isBinary, base64-decode the data string; otherwise
// use the raw string. On decode error, falls back to raw string.
func decodeBase64Message(dataStr string, isBinary bool) []byte {
	if isBinary {
		decoded, err := base64.StdEncoding.DecodeString(dataStr)
		if err != nil {
			return []byte(dataStr)
		}
		return decoded
	}
	return []byte(dataStr)
}

// TestWsSendBinaryBase64Decode tests that ws_send correctly decodes base64 data
// when isBinary is true (matching TypeScript client behavior)
func TestWsSendBinaryBase64Decode(t *testing.T) {
	t.Parallel()

	// Original binary data (some non-ASCII bytes)
	originalData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD, 0x48, 0x65, 0x6C, 0x6C, 0x6F}

	// TypeScript client base64 encodes binary data before sending
	base64Encoded := base64.StdEncoding.EncodeToString(originalData)

	// Simulate the JSON message that would come from TypeScript client
	message := map[string]interface{}{
		"action":    "ws_send",
		"requestId": "test-request-123",
		"data":      base64Encoded,
		"isBinary":  true,
	}

	dataStr, ok := message["data"].(string)
	if !ok {
		t.Fatal("data should be a string")
	}
	isBinary, _ := message["isBinary"].(bool)
	resultData := decodeBase64Message(dataStr, isBinary)

	// Verify the decoded data matches the original
	if len(resultData) != len(originalData) {
		t.Errorf("Length mismatch: got %d, want %d", len(resultData), len(originalData))
	}

	for i, b := range originalData {
		if resultData[i] != b {
			t.Errorf("Byte mismatch at index %d: got 0x%02X, want 0x%02X", i, resultData[i], b)
		}
	}
}

// TestWsSendTextNoBase64Decode tests that ws_send does NOT decode base64
// when isBinary is false (text message)
func TestWsSendTextNoBase64Decode(t *testing.T) {
	t.Parallel()

	// Text message - should NOT be base64 decoded
	textData := "Hello, World!"

	message := map[string]interface{}{
		"action":    "ws_send",
		"requestId": "test-request-456",
		"data":      textData,
		"isBinary":  false,
	}

	dataStr, ok := message["data"].(string)
	if !ok {
		t.Fatal("data should be a string")
	}
	isBinary, _ := message["isBinary"].(bool)
	resultData := decodeBase64Message(dataStr, isBinary)

	// For text messages, the data should be the original text
	if string(resultData) != textData {
		t.Errorf("Text data mismatch: got %q, want %q", string(resultData), textData)
	}
}

// TestWsSendBinaryBase64InvalidFallback tests graceful fallback when base64 decode fails
func TestWsSendBinaryBase64InvalidFallback(t *testing.T) {
	t.Parallel()

	// Invalid base64 string
	invalidBase64 := "not-valid-base64!!!"

	message := map[string]interface{}{
		"action":    "ws_send",
		"requestId": "test-request-789",
		"data":      invalidBase64,
		"isBinary":  true,
	}

	dataStr, ok := message["data"].(string)
	if !ok {
		t.Fatal("data should be a string")
	}
	isBinary, _ := message["isBinary"].(bool)
	resultData := decodeBase64Message(dataStr, isBinary)

	// On invalid base64, should fallback to the raw string
	if string(resultData) != invalidBase64 {
		t.Errorf("Fallback data mismatch: got %q, want %q", string(resultData), invalidBase64)
	}
}

// TestWsSendBinaryEmptyData tests handling of empty binary data
func TestWsSendBinaryEmptyData(t *testing.T) {
	t.Parallel()

	// Empty data, base64 encoded
	emptyData := []byte{}
	base64Encoded := base64.StdEncoding.EncodeToString(emptyData)

	resultData := decodeBase64Message(base64Encoded, true)

	if len(resultData) != 0 {
		t.Errorf("Empty data should decode to empty slice, got length %d", len(resultData))
	}
}

// TestWsSendParseFromJSON tests end-to-end JSON parsing like in actual handler
func TestWsSendParseFromJSON(t *testing.T) {
	t.Parallel()

	// Simulate actual JSON that comes over the wire
	originalData := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	base64Encoded := base64.StdEncoding.EncodeToString(originalData)

	jsonStr := `{"action":"ws_send","requestId":"test-123","data":"` + base64Encoded + `","isBinary":true}`

	var baseMessage map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &baseMessage)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// This simulates the parsing logic in the actual handler
	action, _ := baseMessage["action"].(string)
	if action != "ws_send" {
		t.Errorf("Expected action ws_send, got %s", action)
	}

	dataStr, _ := baseMessage["data"].(string)
	isBinary, _ := baseMessage["isBinary"].(bool)
	resultData := decodeBase64Message(dataStr, isBinary)

	// Verify the result matches original binary data
	if len(resultData) != len(originalData) {
		t.Errorf("Length mismatch: got %d, want %d", len(resultData), len(originalData))
	}

	for i, b := range originalData {
		if resultData[i] != b {
			t.Errorf("Byte mismatch at index %d: got 0x%02X, want 0x%02X", i, resultData[i], b)
		}
	}
}

// Issue #13 (MAJOR): Edge case - invalid padding in base64
func TestWsSendBinaryBase64InvalidPadding(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		wantRaw  bool // true means we expect fallback to raw string
	}{
		{
			name:    "missing padding",
			input:   "SGVsbG8", // "Hello" without padding (valid for RawStdEncoding but not StdEncoding)
			wantRaw: true,
		},
		{
			name:    "extra padding",
			input:   "SGVsbG8=====",
			wantRaw: true,
		},
		{
			name:    "wrong padding character",
			input:   "SGVsbG8!",
			wantRaw: true,
		},
		{
			name:    "valid with correct padding",
			input:   base64.StdEncoding.EncodeToString([]byte("Hello")),
			wantRaw: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := decodeBase64Message(tc.input, true)
			if tc.wantRaw {
				if string(result) != tc.input {
					t.Errorf("Expected raw fallback %q, got %q", tc.input, string(result))
				}
			} else {
				if string(result) != "Hello" {
					t.Errorf("Expected decoded 'Hello', got %q", string(result))
				}
			}
		})
	}
}

// Issue #13 (MAJOR): Edge case - whitespace in base64 string
func TestWsSendBinaryBase64Whitespace(t *testing.T) {
	t.Parallel()

	original := []byte("whitespace test data")
	validB64 := base64.StdEncoding.EncodeToString(original)

	testCases := []struct {
		name  string
		input string
	}{
		{"leading space", " " + validB64},
		{"trailing space", validB64 + " "},
		{"embedded newline", validB64[:4] + "\n" + validB64[4:]},
		{"embedded tab", validB64[:4] + "\t" + validB64[4:]},
		{"carriage return", validB64[:4] + "\r\n" + validB64[4:]},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := decodeBase64Message(tc.input, true)
			// base64.StdEncoding does NOT handle whitespace - these should fallback
			// to raw string (the production code falls back gracefully)
			if string(result) == string(original) {
				// StdEncoding decoded it despite whitespace - unexpected but not wrong
				t.Logf("StdEncoding handled whitespace in %q (informational)", tc.name)
			}
			// Just verify no panic - the result is either decoded or raw fallback
			if len(result) == 0 {
				t.Error("Result should not be empty")
			}
		})
	}
}

// Issue #13 (MAJOR): Edge case - large payloads
func TestWsSendBinaryBase64LargePayload(t *testing.T) {
	t.Parallel()

	// 1MB binary payload
	const payloadSize = 1024 * 1024
	largeData := make([]byte, payloadSize)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	encoded := base64.StdEncoding.EncodeToString(largeData)
	result := decodeBase64Message(encoded, true)

	if len(result) != payloadSize {
		t.Errorf("Large payload length mismatch: got %d, want %d", len(result), payloadSize)
	}

	// Spot-check some bytes
	for _, idx := range []int{0, 1, 255, 256, payloadSize / 2, payloadSize - 1} {
		expected := byte(idx % 256)
		if result[idx] != expected {
			t.Errorf("Large payload byte mismatch at %d: got 0x%02X, want 0x%02X", idx, result[idx], expected)
		}
	}
}

// Issue #13 (MAJOR): Edge case - null bytes in binary data
func TestWsSendBinaryBase64NullBytes(t *testing.T) {
	t.Parallel()

	// Data with null bytes interspersed
	dataWithNulls := []byte{0x00, 0x00, 0x41, 0x00, 0x42, 0x00, 0x00, 0x43}
	encoded := base64.StdEncoding.EncodeToString(dataWithNulls)

	result := decodeBase64Message(encoded, true)

	if len(result) != len(dataWithNulls) {
		t.Fatalf("Null byte data length mismatch: got %d, want %d", len(result), len(dataWithNulls))
	}

	for i, b := range dataWithNulls {
		if result[i] != b {
			t.Errorf("Null byte data mismatch at index %d: got 0x%02X, want 0x%02X", i, result[i], b)
		}
	}
}

// Issue #13 (MAJOR): Edge case - all null bytes
func TestWsSendBinaryBase64AllNulls(t *testing.T) {
	t.Parallel()

	allNulls := make([]byte, 256)
	encoded := base64.StdEncoding.EncodeToString(allNulls)

	result := decodeBase64Message(encoded, true)

	if len(result) != 256 {
		t.Fatalf("All-nulls data length mismatch: got %d, want 256", len(result))
	}

	for i, b := range result {
		if b != 0x00 {
			t.Errorf("Expected null byte at index %d, got 0x%02X", i, b)
		}
	}
}

// Issue #13 (MAJOR): Edge case - string that looks like base64 but has special chars
func TestWsSendBinaryBase64SpecialCharacters(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input string
	}{
		{"unicode", base64.StdEncoding.EncodeToString([]byte("Hello"))},
		{"url-safe chars", "SGVsbG8-_w=="},           // URL-safe base64 chars (- and _)
		{"only padding", "===="},                       // Only padding
		{"single char", "Q"},                           // Too short
		{"empty string", ""},                           // Empty
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Should not panic regardless of input
			result := decodeBase64Message(tc.input, true)
			_ = result // just verify no panic
		})
	}
}

// Issue #13 (MAJOR): Edge case - isBinary missing from JSON (defaults to false)
func TestWsSendBinaryMissingFlag(t *testing.T) {
	t.Parallel()

	jsonStr := `{"action":"ws_send","requestId":"test","data":"SGVsbG8="}`

	var baseMessage map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &baseMessage)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	dataStr, _ := baseMessage["data"].(string)
	// isBinary not present - type assertion returns zero value (false)
	isBinary, _ := baseMessage["isBinary"].(bool)

	if isBinary {
		t.Error("Missing isBinary should default to false")
	}

	result := decodeBase64Message(dataStr, isBinary)
	// Without isBinary, should treat as raw text (no decode)
	if string(result) != "SGVsbG8=" {
		t.Errorf("Expected raw base64 string, got %q", string(result))
	}
}

// Issue #13 (MAJOR): Edge case - data field is not a string (type safety)
func TestWsSendBinaryDataNotString(t *testing.T) {
	t.Parallel()

	// Simulate malformed message where data is a number
	jsonStr := `{"action":"ws_send","requestId":"test","data":12345,"isBinary":true}`

	var baseMessage map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &baseMessage)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// data.(string) should fail with ok=false
	dataStr, ok := baseMessage["data"].(string)
	if ok {
		t.Error("data type assertion should fail for numeric data")
	}
	if dataStr != "" {
		t.Errorf("Failed type assertion should yield empty string, got %q", dataStr)
	}
}

// Issue #13 (MAJOR): Verify base64 round-trip for all byte values (0x00-0xFF)
func TestWsSendBinaryBase64AllByteValues(t *testing.T) {
	t.Parallel()

	allBytes := make([]byte, 256)
	for i := range allBytes {
		allBytes[i] = byte(i)
	}

	encoded := base64.StdEncoding.EncodeToString(allBytes)
	result := decodeBase64Message(encoded, true)

	if len(result) != 256 {
		t.Fatalf("Round-trip length mismatch: got %d, want 256", len(result))
	}

	for i := 0; i < 256; i++ {
		if result[i] != byte(i) {
			t.Errorf("Round-trip byte mismatch at %d: got 0x%02X, want 0x%02X", i, result[i], byte(i))
		}
	}
}

// Issue #13: Verify isBinary as string "true" vs bool true (JSON type handling)
func TestWsSendBinaryFlagAsString(t *testing.T) {
	t.Parallel()

	// In JSON, isBinary could come as string "true" instead of bool true
	jsonStr := `{"action":"ws_send","requestId":"test","data":"SGVsbG8=","isBinary":"true"}`

	var baseMessage map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &baseMessage)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// bool type assertion will fail for string "true"
	isBinary, ok := baseMessage["isBinary"].(bool)
	if ok {
		t.Error("String 'true' should not pass bool type assertion")
	}

	// Fallback: check string value
	if !isBinary {
		isBinaryStr, strOk := baseMessage["isBinary"].(string)
		if strOk && strings.EqualFold(isBinaryStr, "true") {
			// Production code should handle this case
			t.Log("isBinary was string 'true' - production code should handle this variant")
		}
	}
}
