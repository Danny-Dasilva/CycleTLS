//go:build integration
// +build integration

package cycletls_test

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestHTTP1RequestHeadersPreserveCasingOnWire(t *testing.T) {
	rawRequest := captureRawHTTP1Request(t, cycletls.Options{
		Headers: map[string]string{
			"x-CUSTOM-case": "one",
		},
		UserAgent:  "CycleTLS-Test",
		ForceHTTP1: true,
	})

	assertHeaderLine(t, rawRequest, "x-CUSTOM-case: one")
	assertNoHeaderLine(t, rawRequest, "X-Custom-Case:")
}

func TestHTTP1RequestHeadersSendMultipleValuesOnWire(t *testing.T) {
	rawRequest := captureRawHTTP1Request(t, cycletls.Options{
		HeaderValues: map[string][]string{
			"x-MULTI-value": []string{"one", "two"},
		},
		UserAgent:  "CycleTLS-Test",
		ForceHTTP1: true,
	})

	assertHeaderLine(t, rawRequest, "x-MULTI-value: one")
	assertHeaderLine(t, rawRequest, "x-MULTI-value: two")
	if got := countHeaderLines(rawRequest, "x-MULTI-value:"); got != 2 {
		t.Fatalf("x-MULTI-value line count = %d, want 2\nraw request:\n%s", got, rawRequest)
	}
	assertNoHeaderLine(t, rawRequest, "X-Multi-Value:")
}

func captureRawHTTP1Request(t *testing.T, options cycletls.Options) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	rawRequests := make(chan string, 1)
	serverErrors := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErrors <- err
			return
		}
		defer conn.Close()

		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			serverErrors <- err
			return
		}

		reader := bufio.NewReader(conn)
		var raw strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				serverErrors <- err
				return
			}
			raw.WriteString(line)
			if line == "\r\n" {
				break
			}
		}

		rawRequests <- raw.String()
		_, err = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nOK"))
		if err != nil {
			serverErrors <- err
		}
	}()

	client := cycletls.Init()
	defer client.Close()

	resp, err := client.Do(fmt.Sprintf("http://%s/", listener.Addr().String()), options, "GET")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200, body: %q", resp.Status, resp.Body)
	}

	select {
	case rawRequest := <-rawRequests:
		return rawRequest
	case err := <-serverErrors:
		t.Fatalf("server failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for raw request")
	}

	return ""
}

func assertHeaderLine(t *testing.T, rawRequest string, line string) {
	t.Helper()
	if !strings.Contains(rawRequest, "\r\n"+line+"\r\n") {
		t.Fatalf("missing header line %q\nraw request:\n%s", line, rawRequest)
	}
}

func assertNoHeaderLine(t *testing.T, rawRequest string, linePrefix string) {
	t.Helper()
	if countHeaderLines(rawRequest, linePrefix) != 0 {
		t.Fatalf("unexpected header line prefix %q\nraw request:\n%s", linePrefix, rawRequest)
	}
}

func countHeaderLines(rawRequest string, linePrefix string) int {
	count := 0
	for _, line := range strings.Split(rawRequest, "\r\n") {
		if strings.HasPrefix(line, linePrefix) {
			count++
		}
	}
	return count
}
