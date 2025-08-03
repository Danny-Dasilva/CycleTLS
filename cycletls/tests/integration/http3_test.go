//go:build integration
// +build integration

package cycletls_test

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestHTTP3Request(t *testing.T) {
	// Create TLS config
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	// Create HTTP/3 transport
	transport := cycletls.NewHTTP3Transport(tlsConfig)

	// Create a request
	ctx := context.Background()
	req, err := transport.RoundTripper.RoundTrip(context.TODO(), "GET", "https://cloudflare-quic.com/", nil, nil, nil)
	if err != nil {
		// This test might fail if HTTP/3 is not supported by the test environment
		t.Skipf("HTTP/3 request failed: %v", err)
		return
	}

	// Check response status
	if req.StatusCode != 200 {
		t.Errorf("HTTP/3 request returned status %d, want 200", req.StatusCode)
	}

	// Check protocol
	if req.Proto != "HTTP/3.0" {
		t.Errorf("HTTP/3 request used protocol %s, want HTTP/3.0", req.Proto)
	}

	// Clean up
	req.Body.Close()
}

func TestHTTP3Transport(t *testing.T) {
	// Create browser configuration
	browser := cycletls.Browser{
		UserAgent:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		InsecureSkipVerify: true,
		ForceHTTP3:         true,
	}

	// Create HTTP/3 transport
	transport := cycletls.NewHTTP3Transport(browser)

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