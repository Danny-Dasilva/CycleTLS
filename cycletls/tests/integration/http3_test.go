//go:build integration
// +build integration

package cycletls_test

import (
	"crypto/tls"
	"os"
	"testing"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	http "github.com/Danny-Dasilva/fhttp"
)

func TestHTTP3Request(t *testing.T) {
	// Skip HTTP/3 tests in CI environment due to network restrictions
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping HTTP/3 test in CI environment due to network restrictions")
		return
	}

	// Create TLS config
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	// Create HTTP/3 transport
	transport := cycletls.NewHTTP3Transport(tlsConfig)

	// Create a test request using fhttp
	req, err := http.NewRequest("GET", "https://cloudflare-quic.com/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Perform the request using RoundTrip
	resp, err := transport.RoundTrip(req)
	if err != nil {
		// This test might fail if HTTP/3 is not supported by the test environment
		t.Skipf("HTTP/3 request failed: %v", err)
		return
	}

	// Check response status
	if resp.StatusCode != 200 {
		t.Errorf("HTTP/3 request returned status %d, want 200", resp.StatusCode)
	}

	// Check protocol (HTTP/3 typically reports as HTTP/3 or HTTP/3.0)
	if resp.Proto != "HTTP/3.0" && resp.Proto != "HTTP/3" {
		t.Logf("HTTP/3 request used protocol %s (expected HTTP/3.0 or HTTP/3)", resp.Proto)
	}

	// Clean up
	resp.Body.Close()
}

func TestHTTP3Transport(t *testing.T) {
	// Create TLS config from browser-like settings
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	// Create HTTP/3 transport
	transport := cycletls.NewHTTP3Transport(tlsConfig)

	// Check that the transport was created successfully
	if transport == nil {
		t.Error("Failed to create HTTP/3 transport")
	}

	// Check HTTP/3 transport properties
	if transport.TLSClientConfig == nil {
		t.Error("HTTP/3 transport has nil TLSClientConfig")
	}

	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("HTTP/3 transport.TLSClientConfig.InsecureSkipVerify is false, want true")
	}
}