package main

import (
	"log"

	// cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
	"./cycletls"
)

// Static variables
var (
	ja3       = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0"
	userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36"
)

// RequestConfig holds the configuration for each request.
type RequestConfig struct {
	URL     string
	Method  string
	Options cycletls.Options
}

func main() {
	client := cycletls.Init(true) // Initialize with worker pool

	// Define the requests
	requests := []RequestConfig{
		{
			URL:    "http://httpbin.org/delay/4",
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
				Body:      `{"field":"POST-VAL"}`,
				Ja3:       ja3,
				UserAgent: userAgent,
			},
		},
		{
			URL:    "http://httpbin.org/cookies",
			Method: "GET",
			Options: cycletls.Options{
				Ja3:       ja3,
				UserAgent: userAgent,
				Cookies: []cycletls.Cookie{
					{
						Name:  "example1",
						Value: "aaaaaaa",
					},
				},
			},
		},
	}

	// Queue the requests
	for _, req := range requests {
		client.Queue(req.URL, req.Options, req.Method)
	}

	// Asynchronously read responses as soon as they are available
	// They will return as soon as they are processed
	// e.g. Delay 3 will be returned last
	for i := 0; i < len(requests); i++ {
		response := <-client.RespChan
		log.Println("Response:", response)
	}

	// Close the client
	client.Close()
}
