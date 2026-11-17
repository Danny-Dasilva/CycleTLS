//go:build integration
// +build integration

package tlsfingerprint_test

import (
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// TestBytes tests the /bytes/{n} endpoint returns exactly n random bytes
func TestBytes(t *testing.T) {
	client := cycletls.Init(cycletls.WithRawBytes())
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/bytes/100", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)

	if len(resp.BodyBytes) != 100 {
		t.Errorf("Expected exactly 100 bytes, got %d bytes", len(resp.BodyBytes))
	}
}

// TestBase64 tests the /base64/{value} endpoint decodes base64 correctly
func TestBase64(t *testing.T) {
	client := cycletls.Init(cycletls.WithRawBytes())
	defer client.Close()

	opts := getDefaultOptions()
	// SGVsbG8gV29ybGQ= is "Hello World" in base64
	resp, err := client.Do(TestServerURL+"/base64/SGVsbG8gV29ybGQ=", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)

	expected := "Hello World"
	if string(resp.BodyBytes) != expected {
		t.Errorf("Expected decoded base64 to be %q, got %q", expected, string(resp.BodyBytes))
	}
}
