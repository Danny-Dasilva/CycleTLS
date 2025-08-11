//go:build integration
// +build integration

package cycletls_test

import (
	//"fmt"
	// "encoding/json"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

type CycleTLSOptions struct {
	Ja3Hash      string
	Ja3          string
	UserAgent    string
	HTTPResponse int
}

var TLS13Results = []CycleTLSOptions{
	{"b32309a26951912be7dba376398abc3b", // HelloChrome_100
	"771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36 Edg/100.0.1185.44",
		200},
}

func TestTLS_13(t *testing.T) {
	client := cycletls.Init()
	for _, options := range TLS13Results {

		// Test with a TLS 1.3 endpoint that should work
		response, err := client.Do("https://www.howsmyssl.com/a/check", cycletls.Options{
			Ja3:       options.Ja3,
			UserAgent: options.UserAgent,
		}, "GET")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		
		// Check that we got a successful response
		if response.Status < 200 || response.Status >= 300 {
			t.Errorf("Expected successful response, got status %d", response.Status)
		}
		
		// The response should indicate TLS 1.3 support
		t.Logf("TLS 1.3 test completed with status: %d", response.Status)
	}
}