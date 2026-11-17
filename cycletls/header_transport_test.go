package cycletls

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestOptionsUnmarshalJSONKeepsStringHeadersEditable(t *testing.T) {
	var options Options
	if err := json.Unmarshal([]byte(`{"headers": {"accept": "text/plain"}}`), &options); err != nil {
		t.Fatalf("UnmarshalJSON returned error: %v", err)
	}
	if _, ok := options.HeaderValues["accept"]; ok {
		t.Fatalf("string header was copied into HeaderValues: %#v", options.HeaderValues)
	}

	// A caller that decodes Options and then edits Headers must see the edit sent.
	options.Headers["accept"] = "application/json"
	want := []string{"application/json"}
	if got := options.requestHeaders()["accept"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("requestHeaders = %#v, want %#v", got, want)
	}
}

// captureRawRequest serves one HTTP/1.1 request on a loopback listener and
// returns the request line and header lines exactly as they arrived.
func captureRawRequest(t *testing.T, contentType string, send func(url string)) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	rawRequests := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			rawRequests <- ""
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

		reader := bufio.NewReader(conn)
		var raw strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			raw.WriteString(line)
			if line == "\r\n" {
				break
			}
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: " + contentType + "\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"))
		rawRequests <- raw.String()
	}()

	send("http://" + listener.Addr().String() + "/")

	select {
	case raw := <-rawRequests:
		return raw
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the request")
		return ""
	}
}

// headerLines returns the values of every header line whose name matches
// name in any casing, in wire order.
func headerLines(rawRequest, name string) []string {
	var values []string
	for _, line := range strings.Split(rawRequest, "\r\n") {
		key, value, ok := strings.Cut(line, ": ")
		if ok && strings.EqualFold(key, name) {
			values = append(values, value)
		}
	}
	return values
}

func hasExactLine(rawRequest, line string) bool {
	for _, got := range strings.Split(rawRequest, "\r\n") {
		if got == line {
			return true
		}
	}
	return false
}

// processRequest is the path every request from the Node client takes. The
// transport writes its own Host, User-Agent and Cookie, so a caller-supplied
// copy in another casing must not show up as a second line.
func TestProcessRequestSendsTransportHeadersOnce(t *testing.T) {
	rawRequest := captureRawRequest(t, "text/plain", func(url string) {
		var request cycleTLSRequest
		payload := fmt.Sprintf(`{"requestId": "header-test", "options": {
			"url": %q,
			"method": "GET",
			"userAgent": "CycleTLS-Test",
			"forceHTTP1": true,
			"cookies": [{"name": "b", "value": "2"}],
			"headers": {
				"user-agent": "from-headers",
				"host": "fronted.example",
				"cookie": "a=1",
				"x-CUSTOM-case": "one",
				"x-MULTI-value": ["one", "two"]
			}
		}}`, url)
		if err := json.Unmarshal([]byte(payload), &request); err != nil {
			t.Fatalf("UnmarshalJSON returned error: %v", err)
		}

		prepared := processRequest(request)
		resp, err := prepared.client.Do(prepared.req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
	})

	for name, want := range map[string][]string{
		"User-Agent":    {"CycleTLS-Test"},
		"Host":          {"fronted.example"},
		"Cookie":        {"a=1; b=2"},
		"x-custom-case": {"one"},
		"x-multi-value": {"one", "two"},
	} {
		if got := headerLines(rawRequest, name); !reflect.DeepEqual(got, want) {
			t.Errorf("%s lines = %#v, want %#v\nraw request:\n%s", name, got, want, rawRequest)
		}
	}
	for _, line := range []string{"x-CUSTOM-case: one", "x-MULTI-value: one", "x-MULTI-value: two"} {
		if !hasExactLine(rawRequest, line) {
			t.Errorf("missing header line %q\nraw request:\n%s", line, rawRequest)
		}
	}
}

func TestSSERequestKeepsCallerAcceptAndCasing(t *testing.T) {
	rawRequest := captureRawRequest(t, "text/event-stream", func(url string) {
		prepared := dispatchSSERequest(cycleTLSRequest{
			RequestID: "sse-header-test",
			Options: Options{
				URL:        url,
				UserAgent:  "CycleTLS-Test",
				ForceHTTP1: true,
				Headers: map[string]string{
					"accept":        "application/x-custom",
					"x-CUSTOM-case": "one",
				},
			},
		})
		resp, err := prepared.sseClient.Connect(context.Background(), url)
		if err != nil {
			t.Fatalf("SSE connect failed: %v", err)
		}
		resp.Close()
	})

	if got, want := headerLines(rawRequest, "Accept"), []string{"application/x-custom"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Accept lines = %#v, want %#v\nraw request:\n%s", got, want, rawRequest)
	}
	if !hasExactLine(rawRequest, "x-CUSTOM-case: one") {
		t.Errorf("missing header line %q\nraw request:\n%s", "x-CUSTOM-case: one", rawRequest)
	}
}

// gorilla/websocket rejects or rewrites handshake headers by canonical key,
// so the WebSocket path must hand it canonical keys.
func TestWebSocketRequestUsesCanonicalHeaderKeys(t *testing.T) {
	prepared := dispatchWebSocketRequest(cycleTLSRequest{
		RequestID: "ws-header-test",
		Options: Options{
			URL:          "ws://127.0.0.1:1/",
			Headers:      map[string]string{"host": "fronted.example"},
			HeaderValues: map[string][]string{"x-multi-value": {"one", "two"}},
		},
	})

	headers := prepared.wsClient.Headers
	if got, want := headers["Host"], []string{"fronted.example"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Host = %#v, want %#v (headers: %#v)", got, want, headers)
	}
	if got, want := headers["X-Multi-Value"], []string{"one", "two"}; !reflect.DeepEqual(got, want) {
		t.Errorf("X-Multi-Value = %#v, want %#v (headers: %#v)", got, want, headers)
	}
}
