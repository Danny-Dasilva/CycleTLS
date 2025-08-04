//go:build integration
// +build integration

package cycletls_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	fhttp "github.com/Danny-Dasilva/fhttp"
)

// Simple SSE server for testing
func startSSEServer(t *testing.T, done chan bool) string {
	// Create SSE handler
	sseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		
		// Send events
		for i := 0; i < 3; i++ {
			// Send event type
			fmt.Fprintf(w, "event: message\n")
			
			// Send event ID
			fmt.Fprintf(w, "id: %d\n", i+1)
			
			// Send event data
			fmt.Fprintf(w, "data: Event %d\n\n", i+1)
			
			// Flush the response writer
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			
			// Wait before sending the next event
			time.Sleep(100 * time.Millisecond)
		}
	})
	
	// Start server
	server := &http.Server{
		Addr:    ":9124",
		Handler: sseHandler,
	}
	
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("SSE server error: %v", err)
		}
	}()
	
	// Wait for server startup
	time.Sleep(100 * time.Millisecond)
	
	// Setup shutdown when test is done
	go func() {
		<-done
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()
	
	return "http://localhost:9124"
}

func TestSSEClient(t *testing.T) {
	// Start SSE server
	done := make(chan bool)
	defer func() { done <- true }()
	serverURL := startSSEServer(t, done)
	
	// Create HTTP client using fhttp
	httpClient := &fhttp.Client{
		Timeout: 30 * time.Second,
	}
	
	// Create headers using fhttp
	headers := make(fhttp.Header)
	headers.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	
	// Create SSE client
	sseClient := cycletls.NewSSEClient(httpClient, headers)
	
	// Connect to SSE server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	sseResp, err := sseClient.Connect(ctx, serverURL)
	if err != nil {
		t.Fatalf("Failed to connect to SSE server: %v", err)
	}
	defer sseResp.Close()
	
	// Read events
	eventCount := 0
	for {
		event, err := sseResp.NextEvent()
		if err != nil {
			break
		}
		
		// Check event
		eventCount++
		expectedData := fmt.Sprintf("Event %d", eventCount)
		if event.Data != expectedData {
			t.Errorf("Event %d data is %q, want %q", eventCount, event.Data, expectedData)
		}
		
		if event.ID != fmt.Sprintf("%d", eventCount) {
			t.Errorf("Event %d ID is %q, want %q", eventCount, event.ID, fmt.Sprintf("%d", eventCount))
		}
		
		if event.Event != "message" {
			t.Errorf("Event %d type is %q, want %q", eventCount, event.Event, "message")
		}
		
		if eventCount >= 3 {
			break
		}
	}
	
	// Check event count
	if eventCount != 3 {
		t.Errorf("Received %d events, want %d", eventCount, 3)
	}
}

// TestSSEResponse is commented out due to struct field access issues
// func TestSSEResponse(t *testing.T) {
// 	// Create an HTTP response for testing
// 	resp := &http.Response{
// 		StatusCode: http.StatusOK,
// 		Header: http.Header{
// 			"Content-Type": []string{"text/event-stream"},
// 		},
// 		Body: http.NoBody,
// 	}
// 	
// 	// Create an SSE client
// 	sseClient := &cycletls.SSEClient{
// 		HTTPClient: &fhttp.Client{},
// 		Headers:    make(fhttp.Header),
// 	}
// 	
// 	// Create an SSE response
// 	sseResp := &cycletls.SSEResponse{
// 		Response: resp,
// 		client:   sseClient,
// 	}
// 	
// 	// Check SSE response properties
// 	if sseResp.Response.StatusCode != http.StatusOK {
// 		t.Errorf("SSE response status code is %d, want %d", sseResp.Response.StatusCode, http.StatusOK)
// 	}
// 	
// 	if sseResp.Response.Header.Get("Content-Type") != "text/event-stream" {
// 		t.Errorf("SSE response content type is %q, want %q", sseResp.Response.Header.Get("Content-Type"), "text/event-stream")
// 	}
// 	
// 	// Close connection
// 	if err := sseResp.Close(); err != nil {
// 		t.Errorf("Failed to close SSE connection: %v", err)
// 	}
// }

func TestSSE(t *testing.T) {
	// Start the server
	go func() {
		err := http.ListenAndServe(":3333", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
					log.Println("Error writing SSE event:", err)
					return
				}
				_, err = w.Write([]byte("data: " + data + "\n\n"))
				if err != nil {
					log.Println("Error writing SSE data:", err)
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
	
	// Create browser configuration
	browser := cycletls.Browser{
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	}
	
	// Connect to SSE endpoint
	response, err := browser.SSEConnect(context.Background(), "http://127.0.0.1:3333/events")
	if err != nil {
		t.Error(err)
		return
	}
	defer response.Close()
	
	if response == nil {
		t.Error("not is sseClient")
		return
	}
	
	// Read events with timeout protection
	eventCount := 0
	maxEvents := 3
	timeout := time.After(10 * time.Second)
	
	for eventCount < maxEvents {
		select {
		case <-timeout:
			t.Fatal("Test timeout: didn't receive expected events in time")
			return
		default:
			event, err := response.NextEvent()
			if err != nil {
				if err == io.EOF {
					t.Log("SSE stream ended")
					break
				}
				t.Error("SSE read error:", err)
				return
			}
			
			// Check if event is nil (can happen when no event data is available)
			if event == nil {
				continue
			}
			
			eventCount++
			if event.Data != "testing" {
				t.Error("expected 'testing', got:", event.Data)
			}
		}
	}
}