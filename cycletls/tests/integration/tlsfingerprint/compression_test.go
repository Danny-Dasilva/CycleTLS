//go:build integration
// +build integration

package tlsfingerprint_test

import (
	"testing"
)

// TestGzip verifies gzip compression endpoint
func TestGzip(t *testing.T) {
	client := newClient()
	defer client.Close()

	options := getDefaultOptions()
	options.Headers = map[string]string{
		"Accept-Encoding": "gzip, deflate, br",
	}

	resp, err := client.Do(TestServerURL+"/gzip", options, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var compressionResp CompressionResponse
	parseJSONResponse(t, resp.Body, &compressionResp)

	if !compressionResp.Gzipped {
		t.Error("Expected Gzipped to be true")
	}
}

// TestDeflate verifies deflate compression endpoint
func TestDeflate(t *testing.T) {
	client := newClient()
	defer client.Close()

	options := getDefaultOptions()
	options.Headers = map[string]string{
		"Accept-Encoding": "gzip, deflate, br",
	}

	resp, err := client.Do(TestServerURL+"/deflate", options, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var compressionResp CompressionResponse
	parseJSONResponse(t, resp.Body, &compressionResp)

	if !compressionResp.Deflated {
		t.Error("Expected Deflated to be true")
	}
}

// TestBrotli verifies brotli compression endpoint
func TestBrotli(t *testing.T) {
	client := newClient()
	defer client.Close()

	options := getDefaultOptions()
	options.Headers = map[string]string{
		"Accept-Encoding": "gzip, deflate, br",
	}

	resp, err := client.Do(TestServerURL+"/brotli", options, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var compressionResp CompressionResponse
	parseJSONResponse(t, resp.Body, &compressionResp)

	if !compressionResp.Brotli {
		t.Error("Expected Brotli to be true")
	}
}
