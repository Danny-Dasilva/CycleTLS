package cycletls

import (
	"net"
	"testing"

	http "github.com/Danny-Dasilva/fhttp"
)

// Test that getTransport no longer panics when dialTLS returns (conn, nil)
// due to a cached connection/transport already being present for the address.
func TestGetTransport_NoPanicWhenCachedConnPresent(t *testing.T) {
	// Create a new roundTripper
	rtIface := newRoundTripper(Browser{})

	// Type assert to concrete type to access internals
	rt, ok := rtIface.(*roundTripper)
	if !ok {
		t.Fatalf("expected *roundTripper, got %T", rtIface)
	}

	// Build a simple HTTPS request
	req, err := http.NewRequest("GET", "https://example.com/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Determine the addr key used by roundTripper
	addr := rt.getDialTLSAddr(req)

	// Simulate a previously established (cached) TLS connection and transport
	c1, _ := net.Pipe()
	defer c1.Close()
	rt.cachedConnections[addr] = c1
	rt.cachedTransports[addr] = &http.Transport{}

	// Ensure no panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("did not expect panic: %v", r)
		}
	}()

	if err := rt.getTransport(req, addr); err != nil {
		t.Fatalf("getTransport returned error: %v", err)
	}
}
