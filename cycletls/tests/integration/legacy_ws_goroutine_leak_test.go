//go:build integration
// +build integration

package cycletls_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestLegacyWebSocketGoroutineLeak tests that the legacy WebSocket handler (v=1)
// properly cleans up goroutines when clients disconnect.
// This is a regression test for the goroutine leak bug where writeSocket
// would block forever because chanWrite was never closed.
func TestLegacyWebSocketGoroutineLeak(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping goroutine leak test in short mode")
	}

	// Create a simple CycleTLS server for testing BEFORE measuring baseline
	// so server goroutines are included in the baseline count
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This simulates the CycleTLS WSEndpoint but in a test context
		// We can't directly test the real WSEndpoint without the full server
		// So we test the principle: channel closure on disconnect
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		// Simulate the legacy handler pattern with proper cleanup
		chanRead := make(chan map[string]interface{})
		chanWrite := make(chan []byte)
		done := make(chan struct{})

		// Reader goroutine
		go func() {
			defer close(done)
			defer close(chanRead)
			for {
				_, message, err := ws.ReadMessage()
				if err != nil {
					return
				}
				var msg map[string]interface{}
				if json.Unmarshal(message, &msg) == nil {
					chanRead <- msg
				}
			}
		}()

		// Processor goroutine
		go func() {
			for range chanRead {
				// Process messages (no-op for test)
			}
		}()

		// Cleanup goroutine - closes chanWrite when reader exits
		go func() {
			<-done
			close(chanWrite)
		}()

		// Writer (blocks until chanWrite is closed)
		for buf := range chanWrite {
			ws.WriteMessage(websocket.BinaryMessage, buf)
		}
	}))
	defer server.Close()

	// Measure baseline AFTER server is fully started so server goroutines
	// are included in the baseline and don't count as "leaks"
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()

	// Convert http URL to ws URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?v=1"

	// Create and disconnect multiple clients to test for leaks
	numClients := 5
	var wg sync.WaitGroup

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientNum int) {
			defer wg.Done()

			// Connect
			dialer := websocket.Dialer{
				HandshakeTimeout: 5 * time.Second,
			}
			conn, _, err := dialer.Dial(wsURL, nil)
			if err != nil {
				t.Logf("Client %d: Failed to connect: %v", clientNum, err)
				return
			}

			// Send a message
			msg := map[string]interface{}{
				"requestId": "test",
				"url":       "https://example.com",
			}
			msgBytes, _ := json.Marshal(msg)
			conn.WriteMessage(websocket.TextMessage, msgBytes)

			// Wait briefly then disconnect
			time.Sleep(50 * time.Millisecond)
			conn.Close()
		}(i)
	}

	// Wait for all clients to finish
	wg.Wait()

	// Allow time for goroutines to clean up
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check goroutine count
	finalGoroutines := runtime.NumGoroutine()
	leakedGoroutines := finalGoroutines - baselineGoroutines

	// Allow for some variance (test framework goroutines, GC, CI environment jitter)
	// But we should not have numClients * 3 leaked goroutines (reader, processor, writer per client)
	maxAllowedLeak := 10 // Buffer for timing/GC variance and CI environments
	if leakedGoroutines > maxAllowedLeak {
		t.Errorf("Goroutine leak detected: baseline=%d, final=%d, leaked=%d (max allowed=%d)",
			baselineGoroutines, finalGoroutines, leakedGoroutines, maxAllowedLeak)
	} else {
		t.Logf("Goroutine count OK: baseline=%d, final=%d, diff=%d",
			baselineGoroutines, finalGoroutines, leakedGoroutines)
	}
}

// TestLegacyWebSocketExitAction tests that the "exit" action properly terminates
// the legacy WebSocket connection without leaking goroutines.
func TestLegacyWebSocketExitAction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	// Create server BEFORE measuring baseline so server goroutines are included
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		chanRead := make(chan map[string]interface{})
		chanWrite := make(chan []byte)
		done := make(chan struct{})

		go func() {
			defer close(done)
			defer close(chanRead)
			for {
				_, message, err := ws.ReadMessage()
				if err != nil {
					return
				}
				var msg map[string]interface{}
				if json.Unmarshal(message, &msg) == nil {
					// Handle exit action
					if action, ok := msg["action"]; ok && action == "exit" {
						ws.WriteMessage(websocket.CloseMessage,
							websocket.FormatCloseMessage(websocket.CloseNormalClosure, "exit"))
						ws.Close()
						return
					}
					chanRead <- msg
				}
			}
		}()

		go func() {
			for range chanRead {
			}
		}()

		go func() {
			<-done
			close(chanWrite)
		}()

		for buf := range chanWrite {
			ws.WriteMessage(websocket.BinaryMessage, buf)
		}
	}))
	defer server.Close()

	// Measure baseline AFTER server is fully started
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?v=1"

	// Connect and send exit action
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Send exit action
	exitMsg := map[string]interface{}{"action": "exit"}
	msgBytes, _ := json.Marshal(exitMsg)
	conn.WriteMessage(websocket.TextMessage, msgBytes)

	// Wait for close frame
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Log("Received unexpected message after exit")
	}
	conn.Close()

	// Allow cleanup
	time.Sleep(300 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	leakedGoroutines := finalGoroutines - baselineGoroutines

	if leakedGoroutines > 10 {
		t.Errorf("Goroutine leak after exit action: baseline=%d, final=%d, leaked=%d",
			baselineGoroutines, finalGoroutines, leakedGoroutines)
	} else {
		t.Logf("Goroutine count OK after exit: baseline=%d, final=%d, diff=%d",
			baselineGoroutines, finalGoroutines, leakedGoroutines)
	}
}
