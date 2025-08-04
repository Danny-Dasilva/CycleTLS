//go:build integration
// +build integration

package cycletls_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestConnectionReuse(t *testing.T) {
	// Track both server-side connections and request tracking
	connectionTracker := make(map[string]int) // Track requests by RemoteAddr
	connectionMutex := sync.Mutex{}
	
	// Track actual connection lifecycle - use atomic or protected variables
	var connectionCount int
	var handshakeCount int
	
	// Create unstarted server so we can configure ConnState before starting
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectionMutex.Lock()
		connectionTracker[r.RemoteAddr]++
		connectionMutex.Unlock()
		
		if strings.Contains(r.URL.Path, "/connection-stats") {
			connectionMutex.Lock()
			stats := fmt.Sprintf("unique_connections:%d,total_requests:%d,handshakes:%d", 
				len(connectionTracker), 
				getTotalRequests(connectionTracker),
				handshakeCount)
			connectionMutex.Unlock()
			w.Write([]byte(stats))
			return
		}
		
		// Any other path just returns OK with connection info
		w.Write([]byte(fmt.Sprintf("OK from %s", r.RemoteAddr)))
	}))
	
	// Configure connection state tracking before starting
	server.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		connectionMutex.Lock()
		defer connectionMutex.Unlock()
		
		switch state {
		case http.StateNew:
			connectionCount++
			handshakeCount++
		case http.StateClosed:
		case http.StateIdle:
		case http.StateActive:
		}
	}
	
	// Start TLS server
	server.StartTLS()
	defer server.Close()
	
	// Extract server URL
	serverURL := server.URL
	
	// Initialize client options with connection reuse settings
	options := cycletls.Options{
		Ja3:                "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent:          "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		InsecureSkipVerify: true, // Required for test server's self-signed certificate
		EnableConnectionReuse: true, // Enable connection reuse for the test
	}
	
	// Make multiple requests using the same client instance to test connection reuse
	client := cycletls.Init(false) // Don't use worker pool to focus on connection reuse
	defer client.Close() // Ensure resources are cleaned up
	
	// Make first request
	resp1, err := client.Do(serverURL+"/first", options, "GET")
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	if resp1.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp1.Status)
	}
	
	// Make second request to the same server (should reuse connection)
	resp2, err := client.Do(serverURL+"/second", options, "GET")
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	if resp2.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp2.Status)
	}
	
	// Make third request to the same server (should reuse connection)
	resp3, err := client.Do(serverURL+"/third", options, "GET")
	if err != nil {
		t.Fatalf("Third request failed: %v", err)
	}
	if resp3.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp3.Status)
	}
	
	// Get connection statistics
	respStats, err := client.Do(serverURL+"/connection-stats", options, "GET")
	if err != nil {
		t.Fatalf("Connection stats request failed: %v", err)
	}
	

	
	// Parse the stats - format: unique_connections:X,total_requests:Y,handshakes:Z
	stats := strings.Split(respStats.Body, ",")
	if len(stats) != 3 {
		t.Fatalf("Unexpected stats format: %s", respStats.Body)
	}
	
	totalRequests := extractNumber(stats[1])
	handshakes := extractNumber(stats[2])
	

	
	// For proper connection reuse, we should have:
	// - 4 total requests (3 regular + 1 stats request)
	// - Only 1 handshake for all requests to the same host (connection reuse working)
	if totalRequests != 4 {
		t.Errorf("Expected 4 total requests, got %d", totalRequests)
	}
	
	// New behavior: CycleTLS now reuses connections across requests
	// This means connection reuse is working (single connection for all requests to same host)
	
	// We test that:
	// 1. All requests share the same connection (new behavior)
	// 2. The transport configuration is working (we get responses)
	// 3. The connection tracking is working correctly
	// 4. Connection reuse provides better performance
	
	expectedHandshakes := 1 // Only one handshake needed with connection reuse
	if handshakes != expectedHandshakes {
		t.Errorf("Expected %d handshake (connection reuse enabled), got %d", expectedHandshakes, handshakes)
	}
}

// Helper function to get total requests from connection tracker
func getTotalRequests(tracker map[string]int) int {
	total := 0
	for _, count := range tracker {
		total += count
	}
	return total
}

// Helper function to extract number from "key:value" format
func extractNumber(keyValue string) int {
	parts := strings.Split(keyValue, ":")
	if len(parts) != 2 {
		return 0
	}
	num, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}
	return num
}