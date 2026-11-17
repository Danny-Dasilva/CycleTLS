//go:build integration
// +build integration

package tlsfingerprint_test

import (
	"strings"
	"testing"
)

// TestSSE tests the /sse endpoint returns Server-Sent Events with TLS fingerprint data
func TestSSE(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/sse", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)

	// Verify SSE event format
	if !strings.Contains(resp.Body, "event: message") {
		t.Errorf("Expected SSE response to contain 'event: message', got: %s", resp.Body[:min(500, len(resp.Body))])
	}

	if !strings.Contains(resp.Body, "event: done") {
		t.Errorf("Expected SSE response to contain 'event: done', got: %s", resp.Body[:min(500, len(resp.Body))])
	}

	// Verify ja3_hash appears in data fields
	if !strings.Contains(resp.Body, "ja3_hash") {
		t.Errorf("Expected SSE data to contain 'ja3_hash', got: %s", resp.Body[:min(500, len(resp.Body))])
	}
}
