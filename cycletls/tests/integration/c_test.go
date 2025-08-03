//go:build integration
// +build integration

package cycletls_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestConnectionReuse(t *testing.T) {
	// Create a test server that counts TLS handshakes
	handshakeCount := 0
	
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/handshake-count") {
			fmt.Fprintf(w, "%d", handshakeCount)
			return
		}
		
		// Any other path just returns OK
		w.Write([]byte("OK"))
	}))
	defer server.Close()
	
	// Extract server URL
	serverURL := server.URL
	
	// Initialize client options
	options := cycletls.Options{
		Ja3:                "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent:          "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		InsecureSkipVerify: true, // Required for test server's self-signed certificate
	}
	
	// Initialize the server connection monitoring
	// We need to access the TLS connection state after each request
	server.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		if state == http.StateNew {
			handshakeCount++
		}
	}
	
	// Make first request
	client := cycletls.Init(false) // Don't use worker pool to focus on connection reuse
	resp1, err := client.Do(serverURL+"/first", options, "GET")
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	if resp1.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp1.Status)
	}
	
	// Make second request to the same server
	resp2, err := client.Do(serverURL+"/second", options, "GET")
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	if resp2.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp2.Status)
	}
	
	// Check the handshake count from the server
	resp3, err := client.Do(serverURL+"/handshake-count", options, "GET")
	if err != nil {
		t.Fatalf("Handshake count request failed: %v", err)
	}
	
	// If connection reuse is working, we should only have 1 handshake despite 3 requests
	// But give some margin (max 2) since the counter request might cause a new connection
	if resp3.Body != "1" && resp3.Body != "2" {
		t.Errorf("Expected 1-2 handshakes for connection reuse, got %s", resp3.Body)
	}
}