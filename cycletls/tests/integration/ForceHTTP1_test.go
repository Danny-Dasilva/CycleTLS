//go:build integration
// +build integration

package cycletls_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// newProtoServer starts a local TLS server that offers both h2 and http/1.1
// and answers with the protocol the request arrived on. The ForceHTTP1 tests
// used tls.peet.ws for this, which answers 401 or 408 when CI runners share an
// address.
func newProtoServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.Proto)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func forceHTTP1Options(forceHTTP1 bool) cycletls.Options {
	return cycletls.Options{
		Body:       "",
		Ja3:        "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
		ForceHTTP1: forceHTTP1,
		// The local server uses httptest's self-signed certificate.
		InsecureSkipVerify: true,
	}
}

func TestForceHTTP1_h2(t *testing.T) {
	srv := newProtoServer(t)

	client := cycletls.Init()
	defer client.Close() // Ensure resources are cleaned up
	resp, err := client.Do(srv.URL, forceHTTP1Options(false), "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp.Status)
	}
	if resp.Body != "HTTP/2.0" {
		t.Fatalf("Expected HTTP/2.0, got %q", resp.Body)
	}
}

func TestForceHTTP1_h1(t *testing.T) {
	srv := newProtoServer(t)

	client := cycletls.Init()
	defer client.Close() // Ensure resources are cleaned up
	resp, err := client.Do(srv.URL, forceHTTP1Options(true), "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("Expected status 200, got %d", resp.Status)
	}
	if resp.Body != "HTTP/1.1" {
		t.Fatalf("Expected HTTP/1.1, got %q", resp.Body)
	}
}
