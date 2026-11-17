package cycletls

import (
	"math"
	"strings"
	"testing"
)

// =============================================================================
// Issue #2: Binary Protocol Buffer Overflow Tests
// =============================================================================

func TestPacketBufferBoundsCheck_ReadU8(t *testing.T) {
	// Empty buffer - readU8 should not panic
	r := NewReader([]byte{})
	_, err := r.ReadU16() // Try reading from empty buffer
	if err == nil {
		t.Error("Expected error reading from empty buffer, got nil")
	}
}

func TestPacketBufferBoundsCheck_ReadU16Overflow(t *testing.T) {
	// Buffer too small for U16
	r := NewReader([]byte{0x01})
	_, err := r.ReadU16()
	if err == nil {
		t.Error("Expected error reading U16 from 1-byte buffer, got nil")
	}
}

func TestPacketBufferBoundsCheck_ReadU32Overflow(t *testing.T) {
	// Buffer too small for U32
	r := NewReader([]byte{0x01, 0x02, 0x03})
	_, err := r.ReadU32()
	if err == nil {
		t.Error("Expected error reading U32 from 3-byte buffer, got nil")
	}
}

func TestPacketBufferBoundsCheck_ReadStringLengthOverflow(t *testing.T) {
	// Length field says 1000 bytes but buffer only has 5
	data := []byte{0x03, 0xE8} // length = 1000
	data = append(data, []byte("hi")...)
	r := NewReader(data)
	_, err := r.ReadString()
	if err == nil {
		t.Error("Expected error reading string with overflowed length, got nil")
	}
}

func TestPacketBufferBoundsCheck_ReadBytesNegativeLength(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02, 0x03})
	_, err := r.ReadBytes(-1)
	if err == nil {
		t.Error("Expected error reading bytes with negative length")
	}
	if err != ErrNegativeLength {
		t.Errorf("Expected ErrNegativeLength, got %v", err)
	}
}

func TestPacketBufferBoundsCheck_ReadBytesExceedsMax(t *testing.T) {
	r := NewReader([]byte{0x01, 0x02, 0x03})
	_, err := r.ReadBytes(MaxReadBytes + 1)
	if err == nil {
		t.Error("Expected error reading bytes exceeding max")
	}
	if err != ErrBytesTooLarge {
		t.Errorf("Expected ErrBytesTooLarge, got %v", err)
	}
}

func TestPacketBufferBoundsCheck_WriteU16RangeValidation(t *testing.T) {
	// Test that writeU16 in the protocol writer validates range
	// The protocol uses 2-byte length encoding, so values > 65535 should be rejected
	w := NewWriter()
	err := w.WriteU16Safe(65536)
	if err == nil {
		t.Error("Expected error writing U16 value > 65535")
	}
}

func TestPacketBufferBoundsCheck_WriteU32RangeValidation(t *testing.T) {
	// Test that writeU32 validates for negative values (when cast from int)
	w := NewWriter()
	err := w.WriteU32Safe(-1)
	if err == nil {
		t.Error("Expected error writing negative U32 value")
	}
}

func TestPacketBufferBoundsCheck_WriteU32Overflow(t *testing.T) {
	// WriteU32Safe must reject negatives and values above math.MaxUint32,
	// and accept the boundary value exactly.
	w := NewWriter()

	if err := w.WriteU32Safe(-1); err == nil {
		t.Error("Expected error writing negative U32 value")
	}

	if err := w.WriteU32Safe(int(math.MaxUint32) + 1); err == nil {
		t.Error("Expected error writing U32 value > math.MaxUint32")
	}

	if err := w.WriteU32Safe(math.MaxUint32); err != nil {
		t.Errorf("Expected no error writing U32 value at boundary, got %v", err)
	}
}

// =============================================================================
// Issue #5: Debug Logging Sensitive Header Redaction
// =============================================================================

func TestRedactSensitiveHeaders(t *testing.T) {
	headers := map[string]string{
		"Authorization":       "Bearer secret-token-12345",
		"Cookie":              "session=abc123; csrf=xyz789",
		"Content-Type":        "application/json",
		"X-Custom-Header":     "safe-value",
		"Proxy-Authorization": "Basic dXNlcjpwYXNz",
		"Set-Cookie":          "id=abc; Path=/; HttpOnly",
	}

	redacted := RedactSensitiveHeaders(headers)

	// Sensitive headers should be redacted
	if redacted["Authorization"] != "[REDACTED]" {
		t.Errorf("Authorization not redacted: %s", redacted["Authorization"])
	}
	if redacted["Cookie"] != "[REDACTED]" {
		t.Errorf("Cookie not redacted: %s", redacted["Cookie"])
	}
	if redacted["Proxy-Authorization"] != "[REDACTED]" {
		t.Errorf("Proxy-Authorization not redacted: %s", redacted["Proxy-Authorization"])
	}
	if redacted["Set-Cookie"] != "[REDACTED]" {
		t.Errorf("Set-Cookie not redacted: %s", redacted["Set-Cookie"])
	}

	// Non-sensitive headers should pass through
	if redacted["Content-Type"] != "application/json" {
		t.Errorf("Content-Type incorrectly redacted: %s", redacted["Content-Type"])
	}
	if redacted["X-Custom-Header"] != "safe-value" {
		t.Errorf("X-Custom-Header incorrectly redacted: %s", redacted["X-Custom-Header"])
	}
}

func TestRedactSensitiveHeaders_CaseInsensitive(t *testing.T) {
	headers := map[string]string{
		"authorization":       "Bearer token",
		"COOKIE":              "session=abc",
		"proxy-authorization": "Basic auth",
	}

	redacted := RedactSensitiveHeaders(headers)

	for _, key := range []string{"authorization", "COOKIE", "proxy-authorization"} {
		if redacted[key] != "[REDACTED]" {
			t.Errorf("Header %q not redacted (case insensitive): %s", key, redacted[key])
		}
	}
}

// =============================================================================
// Issue #6: MaxReadBytes limit validation
// =============================================================================

func TestMaxReadBytesLimit(t *testing.T) {
	// Verify MaxReadBytes is set to a reasonable value
	if MaxReadBytes > 10*1024*1024 {
		t.Errorf("MaxReadBytes too large: %d (should be <= 10MB)", MaxReadBytes)
	}
	if MaxReadBytes < 1024*1024 {
		t.Errorf("MaxReadBytes too small: %d (should be >= 1MB)", MaxReadBytes)
	}
}

// =============================================================================
// Issue #7: Rate Limiting on Local WebSocket
// =============================================================================

func TestRateLimiter_AllowsBurst(t *testing.T) {
	limiter := NewRateLimiter(100, 10) // 100 req/s, burst of 10

	// First 10 requests should all be allowed (burst)
	for i := 0; i < 10; i++ {
		if !limiter.Allow() {
			t.Errorf("Request %d should have been allowed within burst", i)
		}
	}
}

func TestRateLimiter_RejectsOverBurst(t *testing.T) {
	limiter := NewRateLimiter(1, 5) // 1 req/s, burst of 5

	// Exhaust the burst
	for i := 0; i < 5; i++ {
		limiter.Allow()
	}

	// Next request should be rejected (no time passed for token refill)
	if limiter.Allow() {
		t.Error("Request should have been rejected after burst exhaustion")
	}
}

func TestMaxConcurrentRequests(t *testing.T) {
	maxConcurrent := 1000
	tracker := NewConcurrentRequestTracker(maxConcurrent)

	// Should allow up to max
	for i := 0; i < maxConcurrent; i++ {
		if !tracker.TryAcquire() {
			t.Errorf("Request %d should have been allowed (under max)", i)
		}
	}

	// Should reject over max
	if tracker.TryAcquire() {
		t.Error("Request should have been rejected (at max)")
	}

	// After releasing one, should allow again
	tracker.Release()
	if !tracker.TryAcquire() {
		t.Error("Request should have been allowed after release")
	}
}

// =============================================================================
// Issue #10: TLS Certificate Pinning Support Interface
// =============================================================================

func TestCertPinConfig_Validation(t *testing.T) {
	// Valid SHA256 pin (64 hex chars)
	validPin := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	config := &CertPinConfig{
		SHA256Pins: []string{validPin},
	}
	err := config.Validate()
	if err != nil {
		t.Errorf("Valid pin rejected: %v", err)
	}

	// Invalid pin length
	config = &CertPinConfig{
		SHA256Pins: []string{"tooshort"},
	}
	err = config.Validate()
	if err == nil {
		t.Error("Invalid pin length should have been rejected")
	}

	// Invalid hex characters
	config = &CertPinConfig{
		SHA256Pins: []string{"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
	}
	err = config.Validate()
	if err == nil {
		t.Error("Invalid hex characters should have been rejected")
	}
}

// =============================================================================
// Issue #3: Proxy InsecureSkipVerify separation
// =============================================================================

func TestProxyInsecureSkipVerify_DefaultTrue(t *testing.T) {
	// Default behavior: proxy InsecureSkipVerify should be true
	// even when target InsecureSkipVerify is false
	browser := Browser{
		InsecureSkipVerify: false,
	}

	// The ProxyInsecureSkipVerify should default to true for backward compat
	proxyInsecureSkipVerify := getProxyInsecureSkipVerify(browser)
	if !proxyInsecureSkipVerify {
		t.Error("Proxy InsecureSkipVerify should default to true for backward compatibility")
	}
}

func TestProxyInsecureSkipVerify_SeparateFromTarget(t *testing.T) {
	// When target InsecureSkipVerify is false, proxy should still be true
	browser := Browser{
		InsecureSkipVerify: false,
	}

	proxyInsecureSkipVerify := getProxyInsecureSkipVerify(browser)
	if !proxyInsecureSkipVerify {
		t.Error("Proxy InsecureSkipVerify should be independent of target setting")
	}
}

// =============================================================================
// Writer safety tests (Issue #2)
// =============================================================================

func TestWriterStringSafeValidation(t *testing.T) {
	w := NewWriter()

	// String within limits should work
	err := w.WriteStringSafe("hello")
	if err != nil {
		t.Errorf("Short string should be allowed: %v", err)
	}

	// String exceeding max length should fail
	longStr := strings.Repeat("x", MaxStringLen+1)
	err = w.WriteStringSafe(longStr)
	if err == nil {
		t.Error("String exceeding MaxStringLen should be rejected")
	}
}
