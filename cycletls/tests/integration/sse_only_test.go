//go:build integration
// +build integration

package cycletls_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestSSEOnly(t *testing.T) {
	// Start the server
	go func() {
		err := http.ListenAndServe(":3334", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Origin", "*")

			// SSE event format
			event := "message"
			data := "testing"

			// Start SSE loop
			for i := 0; i < 3; i++ {
				// Send SSE event
				_, err := w.Write([]byte("event: " + event + "\n"))
				if err != nil {
					return
				}
				_, err = w.Write([]byte("data: " + data + "\n\n"))
				if err != nil {
					return
				}

				// Flush the response writer
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}

				// Delay before sending the next event
				time.Sleep(1 * time.Second)
			}
		}))
		if err != nil {
			t.Error(err)
		}
	}()

	// Wait for server to start
	time.Sleep(time.Second * 3)

	// Create browser configuration with defaults
	browser := cycletls.Browser{
		UserAgent:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		JA3:                "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		InsecureSkipVerify: true,
	}

	// Connect to SSE endpoint
	response, err := browser.SSEConnect(context.Background(), "http://127.0.0.1:3334/events")
	if err != nil {
		t.Error("SSE connection failed:", err)
		return
	}

	if response == nil {
		t.Error("SSE response is nil")
		return
	}
	defer response.Close()

	// Read events with timeout
	eventCount := 0
	maxEvents := 3
	timeout := time.After(10 * time.Second)

	for eventCount < maxEvents {
		select {
		case <-timeout:
			return
		default:
			event, err := response.NextEvent()
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Error("SSE read error:", err)
				return
			}

			if event == nil {
				continue
			}

			eventCount++

			if event.Data != "testing" {
				t.Errorf("expected 'testing', got: %s", event.Data)
			}
		}
	}

}
