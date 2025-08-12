//go:build integration
// +build integration

package cycletls_test

import (
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestDeflateDecoding(t *testing.T) {
	client := cycletls.Init()
	defer client.Close() // Ensure resources are cleaned up
	resp, err := client.Do("https://httpbin.org/deflate", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Headers:   map[string]string{"Accept-Encoding": "gzip, deflate, br"}, // Axios-style Accept-Encoding
	}, "GET")
	if err != nil {
		t.Fatalf("Deflate request failed: %v", err)
	}

	// Verify response status
	if resp.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp.Status)
	}

	// Parse JSON response - should be automatically decompressed
	jsonBody := resp.JSONBody()
	if jsonBody == nil {
		t.Fatalf("Failed to parse JSON response body: %s", resp.Body)
	}

	// Verify the deflated field indicates the response was deflate-compressed
	deflated, ok := jsonBody["deflated"].(bool)
	if !ok || !deflated {
		t.Fatalf("Expected deflated=true in response, got: %v", jsonBody)
	}

}
func TestBrotliDecoding(t *testing.T) {
	client := cycletls.Init()
	defer client.Close() // Ensure resources are cleaned up
	resp, err := client.Do("https://httpbin.org/brotli", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Headers:   map[string]string{"Accept-Encoding": "gzip, deflate, br"}, // Axios-style Accept-Encoding
	}, "GET")
	if err != nil {
		t.Fatalf("Brotli request failed: %v", err)
	}

	// Verify response status
	if resp.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp.Status)
	}

	// Parse JSON response - should be automatically decompressed
	jsonBody := resp.JSONBody()
	if jsonBody == nil {
		t.Fatalf("Failed to parse JSON response body: %s", resp.Body)
	}

	// Verify the brotli field indicates the response was brotli-compressed
	brotliCompressed, ok := jsonBody["brotli"].(bool)
	if !ok || !brotliCompressed {
		t.Fatalf("Expected brotli=true in response, got: %v", jsonBody)
	}

}

func TestGZIPDecoding(t *testing.T) {
	client := cycletls.Init()
	defer client.Close() // Ensure resources are cleaned up
	resp, err := client.Do("https://httpbin.org/gzip", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Headers:   map[string]string{"Accept-Encoding": "gzip, deflate, br"}, // Axios-style Accept-Encoding
	}, "GET")
	if err != nil {
		t.Fatalf("GZIP request failed: %v", err)
	}

	// Verify response status
	if resp.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp.Status)
	}

	// Parse JSON response - should be automatically decompressed
	jsonBody := resp.JSONBody()
	if jsonBody == nil {
		t.Fatalf("Failed to parse JSON response body: %s", resp.Body)
	}

	// Verify the gzipped field indicates the response was gzip-compressed
	gzipCompressed, ok := jsonBody["gzipped"].(bool)
	if !ok || !gzipCompressed {
		t.Fatalf("Expected gzipped=true in response, got: %v", jsonBody)
	}

}
