package cycletls

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// Issue #2: Protocol Writer with bounds-checked writes
// =============================================================================

// ErrU16Overflow is defined in packet_builder.go

// ErrNegativeValue is returned when a negative value is passed to an unsigned write.
var ErrNegativeValue = errors.New("packet: negative value for unsigned field")

// ErrStringTooLong is returned when a string exceeds MaxStringLen for safe writes.
var ErrStringTooLong = errors.New("packet: string exceeds maximum length for protocol encoding")

// Writer builds binary packets with bounds-checked writes.
type Writer struct {
	buf []byte
}

// NewWriter creates a new Writer.
func NewWriter() *Writer {
	return &Writer{}
}

// WriteU16Safe writes a uint16 with range validation.
func (w *Writer) WriteU16Safe(v int) error {
	if v < 0 {
		return ErrNegativeValue
	}
	if v > 65535 {
		return ErrU16Overflow
	}
	w.buf = append(w.buf, byte(v>>8), byte(v))
	return nil
}

// WriteU32Safe writes a uint32 with range validation.
func (w *Writer) WriteU32Safe(v int) error {
	if v < 0 {
		return ErrNegativeValue
	}
	if int64(v) > int64(math.MaxUint32) {
		return ErrU32Overflow
	}
	w.buf = append(w.buf, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	return nil
}

// WriteStringSafe writes a length-prefixed string with bounds checking.
func (w *Writer) WriteStringSafe(s string) error {
	if len(s) > MaxStringLen {
		return ErrStringTooLong
	}
	l := len(s)
	w.buf = append(w.buf, byte(l>>8), byte(l))
	w.buf = append(w.buf, []byte(s)...)
	return nil
}

// Bytes returns the built packet.
func (w *Writer) Bytes() []byte {
	return w.buf
}

// =============================================================================
// Issue #5: Debug Logging Sensitive Header Redaction
// =============================================================================

// sensitiveHeaders lists header names that should be redacted in logs.
// All comparisons are case-insensitive.
var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"cookie":              true,
	"set-cookie":          true,
	"proxy-authorization": true,
	"x-api-key":           true,
	"x-auth-token":        true,
}

// RedactSensitiveHeaders returns a copy of headers with sensitive values redacted.
// This should be used before logging any headers to prevent leaking credentials.
func RedactSensitiveHeaders(headers map[string]string) map[string]string {
	redacted := make(map[string]string, len(headers))
	for k, v := range headers {
		if sensitiveHeaders[strings.ToLower(k)] {
			redacted[k] = "[REDACTED]"
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

// RedactSensitiveHeaderSlice returns a copy of multi-valued headers with sensitive values redacted.
func RedactSensitiveHeaderSlice(headers map[string][]string) map[string][]string {
	redacted := make(map[string][]string, len(headers))
	for k, v := range headers {
		if sensitiveHeaders[strings.ToLower(k)] {
			redacted[k] = []string{"[REDACTED]"}
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

// =============================================================================
// Issue #7: Rate Limiting for Local WebSocket
// =============================================================================

// RateLimiter implements a simple token bucket rate limiter.
type RateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new token bucket rate limiter.
// rate is tokens per second, burst is the maximum burst size.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: rate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed under the rate limit.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.lastRefill = now

	// Refill tokens
	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

// ConcurrentRequestTracker limits the number of concurrent in-flight requests.
type ConcurrentRequestTracker struct {
	current int64
	max     int64
}

// NewConcurrentRequestTracker creates a tracker with the given maximum.
func NewConcurrentRequestTracker(max int) *ConcurrentRequestTracker {
	return &ConcurrentRequestTracker{
		max: int64(max),
	}
}

// TryAcquire attempts to acquire a slot. Returns false if at capacity.
func (t *ConcurrentRequestTracker) TryAcquire() bool {
	for {
		cur := atomic.LoadInt64(&t.current)
		if cur >= t.max {
			return false
		}
		if atomic.CompareAndSwapInt64(&t.current, cur, cur+1) {
			return true
		}
	}
}

// Release releases a slot.
func (t *ConcurrentRequestTracker) Release() {
	atomic.AddInt64(&t.current, -1)
}

// Current returns the current number of in-flight requests.
func (t *ConcurrentRequestTracker) Current() int64 {
	return atomic.LoadInt64(&t.current)
}

// =============================================================================
// Issue #10: TLS Certificate Pinning Support (Interface)
// =============================================================================

// CertPinConfig configures TLS certificate pinning.
// When pins are set, the TLS handshake will verify that at least one
// of the certificate's SHA256 fingerprints matches a pin.
type CertPinConfig struct {
	// SHA256Pins is a list of SHA256 fingerprints (hex-encoded, 64 chars each)
	// of certificates to trust. If non-empty, connections will fail if none
	// of the server's certificate chain matches.
	SHA256Pins []string
}

// Validate checks that all pins are well-formed SHA256 hex strings.
func (c *CertPinConfig) Validate() error {
	for _, pin := range c.SHA256Pins {
		if len(pin) != 64 {
			return fmt.Errorf("invalid SHA256 pin length: got %d, want 64 hex chars", len(pin))
		}
		if _, err := hex.DecodeString(pin); err != nil {
			return fmt.Errorf("invalid SHA256 pin hex encoding: %w", err)
		}
	}
	return nil
}

// HasPins returns true if any pins are configured.
func (c *CertPinConfig) HasPins() bool {
	return c != nil && len(c.SHA256Pins) > 0
}

// =============================================================================
// Issue #3: Separate proxy InsecureSkipVerify from target
// =============================================================================

// getProxyInsecureSkipVerify returns the InsecureSkipVerify setting for proxy
// connections. This defaults to true for backward compatibility, since proxies
// commonly use self-signed certificates. The target's InsecureSkipVerify
// setting is handled separately by the roundTripper.
func getProxyInsecureSkipVerify(browser Browser) bool {
	// If ProxyInsecureSkipVerify is explicitly set, use it
	if browser.ProxyInsecureSkipVerify != nil {
		return *browser.ProxyInsecureSkipVerify
	}
	// Default to true for backward compatibility
	return true
}

// =============================================================================
// Issue #4: WebSocket Authentication Token
// =============================================================================

// DefaultWSRateLimitPerSec is the default rate limit for WebSocket messages per second.
const DefaultWSRateLimitPerSec = 1000

// DefaultWSBurst is the default burst size for WebSocket rate limiting.
const DefaultWSBurst = 100

// DefaultMaxConcurrentRequests is the default maximum number of concurrent requests.
const DefaultMaxConcurrentRequests = 1000
