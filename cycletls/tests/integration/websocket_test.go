//go:build integration
// +build integration

package cycletls_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"github.com/gorilla/websocket"
	utls "github.com/refraction-networking/utls"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Simple echo WebSocket server for testing
func startWebSocketServer(t *testing.T, done chan bool) string {
	// Create echo handler
	echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
		
		// Upgrade connection to WebSocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()
		
		// Echo loop
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if err := conn.WriteMessage(messageType, p); err != nil {
				break
			}
		}
	})
	
	// Start server
	server := &http.Server{
		Addr:    ":9123",
		Handler: echoHandler,
	}
	
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("WebSocket server error: %v", err)
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
	
	return "ws://localhost:9123"
}

func TestWebSocketClient(t *testing.T) {
	// Start WebSocket server
	done := make(chan bool)
	defer func() { done <- true }()
	serverURL := startWebSocketServer(t, done)
	
	// Create TLS config
	tlsConfig := &utls.Config{
		InsecureSkipVerify: true,
	}
	
	// Create headers
	headers := make(http.Header)
	headers.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	
	// Create WebSocket client
	wsClient := cycletls.NewWebSocketClient(tlsConfig, headers)
	
	// Connect to WebSocket server
	conn, resp, err := wsClient.Connect(serverURL)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket server: %v", err)
	}
	defer conn.Close()
	
	// Check response
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("WebSocket connection returned status %d, want %d", resp.StatusCode, http.StatusSwitchingProtocols)
	}
	
	// Send message
	testMessage := "Hello, WebSocket!"
	if err := conn.WriteMessage(websocket.TextMessage, []byte(testMessage)); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
	
	// Read response
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}
	
	// Check message type
	if messageType != websocket.TextMessage {
		t.Errorf("Received message type %d, want %d", messageType, websocket.TextMessage)
	}
	
	// Check message content
	if string(message) != testMessage {
		t.Errorf("Received message %q, want %q", string(message), testMessage)
	}
}

func TestWebSocketResponse(t *testing.T) {
	// Create WebSocket dialer
	dialer := websocket.DefaultDialer
	
	// Create WebSocket connection
	conn, _, err := dialer.Dial("ws://echo.websocket.org/", nil)
	if err != nil {
		t.Skipf("Cannot connect to echo.websocket.org: %v", err)
		return
	}
	
	wsResponse := &cycletls.WebSocketResponse{
		Conn: conn,
	}
	
	// Send message
	testMessage := "Hello, WebSocket!"
	if err := wsResponse.Send(websocket.TextMessage, []byte(testMessage)); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
	
	// Receive message
	messageType, message, err := wsResponse.Receive()
	if err != nil {
		t.Fatalf("Failed to receive message: %v", err)
	}
	
	// Check message type
	if messageType != websocket.TextMessage {
		t.Errorf("Received message type %d, want %d", messageType, websocket.TextMessage)
	}
	
	// Check message content
	if string(message) != testMessage {
		t.Errorf("Received message %q, want %q", string(message), testMessage)
	}
	
	// Close connection
	if err := wsResponse.Close(); err != nil {
		t.Errorf("Failed to close WebSocket connection: %v", err)
	}
}