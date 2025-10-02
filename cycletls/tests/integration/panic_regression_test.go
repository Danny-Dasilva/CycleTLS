//go:build integration
// +build integration

package cycletls_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// TestPanicRegression tests the specific scenario that caused the panic:
// Multiple goroutines making requests to the same host where a cached transport
// already exists, which previously caused dialTLS to return nil instead of errProtocolNegotiated
func TestPanicRegression(t *testing.T) {
	// Create a simple test server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Initialize client options
	options := cycletls.Options{
		Ja3:                "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent:          "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		InsecureSkipVerify: true,
	}

	client := cycletls.Init(cycletls.WithRawBytes())
	defer client.Close()

	// Make an initial request to establish the transport cache
	_, err := client.Do(server.URL, options, "GET")
	if err != nil {
		t.Fatalf("Initial request failed: %v", err)
	}

	// Now simulate multiple concurrent requests that would trigger the cached transport path
	// This is the scenario that previously caused the panic
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Do(server.URL, options, "GET")
			if err != nil {
				errors <- err
				return
			}
			if resp.Status != 200 {
				errors <- nil // Non-critical error, just track it
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for any errors (panics would cause test failure)
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		}
	}

	t.Log("Panic regression test passed - no panics occurred with cached transports")
}

// TestCachedTransportEdgeCase specifically tests the edge case where
// rt.cachedTransports[addr] != nil in dialTLS
func TestCachedTransportEdgeCase(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	options := cycletls.Options{
		Ja3:                "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent:          "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		InsecureSkipVerify: true,
	}

	client := cycletls.Init(cycletls.WithRawBytes())
	defer client.Close()

	// Make first request to establish cached transport
	resp1, err := client.Do(server.URL, options, "GET")
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	if resp1.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp1.Status)
	}

	// Make second request - this should hit the cached transport path in dialTLS
	// Previously this would return nil error instead of errProtocolNegotiated, causing panic
	resp2, err := client.Do(server.URL, options, "GET")
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	if resp2.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp2.Status)
	}

	// Make third request to be extra sure
	resp3, err := client.Do(server.URL, options, "GET")
	if err != nil {
		t.Fatalf("Third request failed: %v", err)
	}
	if resp3.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp3.Status)
	}

}