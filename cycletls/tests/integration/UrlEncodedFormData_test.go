//go:build integration
// +build integration

package cycletls_test

import (
	"encoding/json"
	"net/url"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

type FormResponse struct {
	Args    map[string]string `json:"args"`
	Data    string            `json:"data"`
	Form    map[string]string `json:"form"`
	Headers map[string]string `json:"headers"`
	Json    interface{}       `json:"json"`
	Origin  string            `json:"origin"`
	URL     string            `json:"url"`
}

func TestUrlEncodedFormDataUpload(t *testing.T) {
	client := cycletls.Init()
	defer client.Close() // Ensure resources are cleaned up

	// Prepare form data
	form := url.Values{}
	form.Add("key1", "value1")
	form.Add("key2", "value2")

	response := doHTTPBinRequestWithRetry(t, client, "http://httpbin.org/post", cycletls.Options{
		// httpbin.org/tlsfingerprint.com fixture cert may be expired/rotated; we test the outgoing TLS fingerprint and HTTP body, not the fixture's cert chain.
		InsecureSkipVerify: true,
		Body:               form.Encode(),
		Headers: map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		},
	}, "POST")
	if isUpstreamFlake(response.Status) {
		t.Skipf("httpbin upstream flake after retries: status %d", response.Status)
	}
	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}

	// Parse the JSON response
	var respData FormResponse
	if err := json.Unmarshal([]byte(response.Body), &respData); err != nil {
		t.Fatal("Unmarshal Error: ", err)
	}

	// Validate the 'form' part of the response
	expectedForm := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	for key, expectedValue := range expectedForm {
		if value, ok := respData.Form[key]; !ok || value != expectedValue {
			t.Fatalf("Expected form field %s to be %s, got %s", key, expectedValue, value)
		}
	}
}
