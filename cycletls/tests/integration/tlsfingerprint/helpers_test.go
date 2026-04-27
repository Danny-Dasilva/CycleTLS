//go:build integration
// +build integration

package tlsfingerprint_test

import (
	"encoding/json"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// TestServerURL is the target server for all tests
const TestServerURL = "https://tlsfingerprint.com"

// Common JA3 fingerprint for all tests (Chrome 120)
var defaultJA3 = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0"

// TLSFingerprintResponse represents common TLS fields in all responses
type TLSFingerprintResponse struct {
	JA3           string `json:"ja3"`
	JA3Hash       string `json:"ja3_hash"`
	JA4           string `json:"ja4"`
	JA4R          string `json:"ja4_r"`
	Akamai        string `json:"akamai"`
	AkamaiHash    string `json:"akamai_hash"`
	PeetPrint     string `json:"peetprint"`
	PeetPrintHash string `json:"peetprint_hash"`
	HTTPVersion   string `json:"http_version"`
}

// EchoResponse represents httpbin-style echo response
type EchoResponse struct {
	TLSFingerprintResponse
	Args    map[string]interface{} `json:"args"`
	Headers map[string]string      `json:"headers"`
	Origin  string                 `json:"origin"`
	URL     string                 `json:"url"`
	Method  string                 `json:"method"`
	Data    string                 `json:"data"`
	Form    map[string]interface{} `json:"form"`
	Files   map[string]interface{} `json:"files"`
	JSON    interface{}            `json:"json"`
}

// CompressionResponse represents gzip/deflate/brotli response
type CompressionResponse struct {
	TLSFingerprintResponse
	Gzipped  bool `json:"gzipped"`
	Deflated bool `json:"deflated"`
	Brotli   bool `json:"brotli"`
}

// CookiesResponse represents /cookies endpoint response
type CookiesResponse struct {
	TLSFingerprintResponse
	Cookies map[string]string `json:"cookies"`
}

// RedirectResponse represents redirect endpoint response
type RedirectResponse struct {
	TLSFingerprintResponse
	RedirectCount int    `json:"redirect_count"`
	Location      string `json:"location"`
}

// StatusResponse represents /status endpoint response
type StatusResponse struct {
	TLSFingerprintResponse
	StatusCode int `json:"status_code"`
}

// DelayResponse represents /delay endpoint response
type DelayResponse struct {
	EchoResponse
	Delay int `json:"delay"`
}

// getDefaultOptions returns standard test options
func getDefaultOptions() cycletls.Options {
	return cycletls.Options{
		// httpbin.org/tlsfingerprint.com fixture cert may be expired/rotated; we test the outgoing TLS fingerprint and HTTP body, not the fixture's cert chain.
		InsecureSkipVerify: true,
		Ja3:                defaultJA3,
		UserAgent:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
}

// assertTLSFieldsPresent validates TLS fingerprint fields exist in response
func assertTLSFieldsPresent(t *testing.T, body string) {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	required := []string{"ja3", "ja3_hash", "ja4", "peetprint", "peetprint_hash"}
	for _, field := range required {
		if val, ok := resp[field]; !ok || val == "" {
			t.Errorf("Missing or empty required TLS field: %s", field)
		}
	}
}

// assertStatusCode validates HTTP status code
func assertStatusCode(t *testing.T, expected, actual int) {
	t.Helper()
	if actual != expected {
		t.Fatalf("Expected status %d, got %d", expected, actual)
	}
}

// parseJSONResponse unmarshals response body into target struct
func parseJSONResponse(t *testing.T, body string, target interface{}) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), target); err != nil {
		t.Fatalf("Failed to parse JSON response: %v\nBody: %s", err, body)
	}
}

// newClient creates a new CycleTLS client for tests
func newClient() cycletls.CycleTLS {
	return cycletls.Init()
}
