//go:build integration
// +build integration

package cycletls_test

import (
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestDelayResponseOrder(t *testing.T) {
	var (
		ja3       = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0"
		userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36"
	)
	client := cycletls.Init(true) // Initialize with worker pool

	// Define the requests
	requests := []struct {
		URL     string
		Method  string
		Options cycletls.Options
	}{
		{
			URL:    "http://httpbin.org/delay/3",
			Method: "GET",
			Options: cycletls.Options{
				Ja3:       ja3,
				UserAgent: userAgent,
			},
		},
		{
			URL:    "http://httpbin.org/get",
			Method: "GET",
			Options: cycletls.Options{
				Ja3:       ja3,
				UserAgent: userAgent,
			},
		},
		{
			URL:    "http://httpbin.org/post",
			Method: "POST",
			Options: cycletls.Options{
				Ja3:       ja3,
				UserAgent: userAgent,
			},
		},
	}

	// Queue the requests
	for _, req := range requests {
		client.Queue(req.URL, req.Options, req.Method)
	}

	// Collect the order of responses
	responseOrder := make([]string, 0, len(requests))
	for i := 0; i < len(requests); i++ {
		response := <-client.RespChan
		responseOrder = append(responseOrder, response.FinalUrl)
	}

	// Close the client
	client.Close()

	// Assert that the last response is from "http://httpbin.org/delay/3"
	expectedLastURL := "http://httpbin.org/delay/3"
	lastResponseURL := responseOrder[len(responseOrder)-1]
	if lastResponseURL != expectedLastURL {
		t.Errorf("Expected last response URL to be %s, got %s", expectedLastURL, lastResponseURL)
	}
}
