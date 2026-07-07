//go:build integration
// +build integration

package tlsfingerprint_test

import (
	"testing"
)

func TestHeaders(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	opts.Headers = map[string]string{
		"X-Custom-Header": "test-value",
		"Accept":          "application/json",
	}

	resp, err := client.Do(TestServerURL+"/headers", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var headersResp struct {
		TLSFingerprintResponse
		Headers map[string]string `json:"headers"`
	}
	parseJSONResponse(t, resp.Body, &headersResp)

	// Verify headers map exists
	if headersResp.Headers == nil {
		t.Fatal("Expected headers in response, got nil")
	}

	// Verify custom header is echoed
	if val, ok := headersResp.Headers["X-Custom-Header"]; !ok {
		t.Error("Expected 'X-Custom-Header' in response headers")
	} else if val != "test-value" {
		t.Errorf("Expected headers['X-Custom-Header'] = 'test-value', got '%s'", val)
	}

	// Verify User-Agent is present (from getDefaultOptions)
	if _, ok := headersResp.Headers["User-Agent"]; !ok {
		t.Error("Expected 'User-Agent' in response headers")
	}
}

func TestIP(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()

	resp, err := client.Do(TestServerURL+"/ip", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var ipResp struct {
		TLSFingerprintResponse
		Origin string `json:"origin"`
	}
	parseJSONResponse(t, resp.Body, &ipResp)

	// Verify origin is non-empty
	if ipResp.Origin == "" {
		t.Error("Expected non-empty 'origin' field in response")
	}
}

func TestUserAgent(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	expectedUA := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	resp, err := client.Do(TestServerURL+"/user-agent", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var uaResp struct {
		TLSFingerprintResponse
		UserAgent string `json:"user-agent"`
	}
	parseJSONResponse(t, resp.Body, &uaResp)

	// Verify user-agent matches what we sent
	if uaResp.UserAgent == "" {
		t.Error("Expected non-empty 'user-agent' field in response")
	}

	if uaResp.UserAgent != expectedUA {
		t.Errorf("Expected user-agent='%s', got '%s'", expectedUA, uaResp.UserAgent)
	}
}
