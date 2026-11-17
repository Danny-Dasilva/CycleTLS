//go:build integration
// +build integration

package tlsfingerprint_test

import (
	"net/url"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestRedirect(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/redirect/3", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	// After following 3 redirects, we end up at /get which returns EchoResponse
	var echoResp EchoResponse
	parseJSONResponse(t, resp.Body, &echoResp)

	// Verify we got a valid response after redirect chain
	if echoResp.Method != "GET" {
		t.Errorf("Expected method=GET, got %s", echoResp.Method)
	}
}

func TestRedirectTo(t *testing.T) {
	client := newClient()
	defer client.Close()

	// Use custom options for this test to ensure a fresh connection.
	// Disable connection reuse to avoid stale HTTP/2 streams from previous tests.
	opts := cycletls.Options{
		Ja3:                   defaultJA3,
		UserAgent:             "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Timeout:               30,    // Increased timeout for CI environments
		EnableConnectionReuse: false, // Disable connection reuse for test isolation
		InsecureSkipVerify:    true,  // Test server uses self-signed cert
	}

	// URL-encode the redirect target to ensure proper handling
	redirectTarget := url.QueryEscape(TestServerURL + "/get")
	resp, err := client.Do(TestServerURL+"/redirect-to?url="+redirectTarget, opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var echoResp EchoResponse
	parseJSONResponse(t, resp.Body, &echoResp)

	// Verify url field is present in response
	if echoResp.URL == "" {
		t.Error("Expected url field in response")
	}
}

func TestStatus(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/status/201", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 201, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var statusResp StatusResponse
	parseJSONResponse(t, resp.Body, &statusResp)

	// Verify status_code in response body matches
	if statusResp.StatusCode != 201 {
		t.Errorf("Expected status_code=201 in body, got %d", statusResp.StatusCode)
	}
}
