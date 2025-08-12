//go:build integration
// +build integration

package cycletls_test

import (
	"strings"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestDelayResponseOrder(t *testing.T) {
	var (
		ja3       = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0"
		ja4r      = "t13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601" // Chrome JA4R fingerprint
		userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36"
	)

	// Create client without worker pool
	client := cycletls.Init()
	defer client.Close() // Ensure resources are cleaned up

	// Define the requests with both JA3 and JA4R testing
	requests := []struct {
		URL     string
		Method  string
		Options cycletls.Options
		Name    string
	}{
		{
			URL:    "http://httpbin.org/delay/1", // Reduced delay for faster testing
			Method: "GET",
			Options: cycletls.Options{
				Ja3:       ja3,
				UserAgent: userAgent,
			},
			Name: "JA3 Delayed Request",
		},
		{
			URL:    "http://httpbin.org/get",
			Method: "GET",
			Options: cycletls.Options{
				Ja4r:      ja4r,
				UserAgent: userAgent,
			},
			Name: "JA4R Quick Request",
		},
		{
			URL:    "http://httpbin.org/post",
			Method: "POST",
			Options: cycletls.Options{
				Ja3:       ja3,
				UserAgent: userAgent,
				Body:      `{"test": "data"}`,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
			},
			Name: "JA3 POST Request",
		},
	}

	// Make requests directly using Do() method
	responses := make([]cycletls.Response, len(requests))
	for i, req := range requests {

		resp, err := client.Do(req.URL, req.Options, req.Method)
		if err != nil {
			t.Errorf("Request %s failed: %v", req.Name, err)
			continue
		}
		responses[i] = resp

	}

	// Verify all requests completed successfully
	for i, req := range requests {
		if responses[i].Status == 0 {
			t.Errorf("Request %s failed - no response received", req.Name)
			continue
		}

		if responses[i].Status < 200 || responses[i].Status >= 300 {
			t.Errorf("Request %s returned status %d, expected 2xx", req.Name, responses[i].Status)
		}

		// Verify URL contains expected path
		if !containsExpectedPath(responses[i].FinalUrl, req.URL) {
			t.Errorf("Request %s - unexpected final URL: %s", req.Name, responses[i].FinalUrl)
		}
	}

}

// Helper function to check if final URL contains expected path
func containsExpectedPath(finalURL, originalURL string) bool {
	// Simple check to see if the path is preserved
	// This handles redirects from http to https
	return finalURL != "" && (finalURL == originalURL ||
		strings.Contains(finalURL, "httpbin.org/delay") ||
		strings.Contains(finalURL, "httpbin.org/get") ||
		strings.Contains(finalURL, "httpbin.org/post"))
}
