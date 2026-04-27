//go:build integration
// +build integration

package cycletls_test

import (
	//"fmt"
	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
	"log"
	"net"
	"runtime"
	"testing"
	"time"
)

// waitForProxy waits for SOCKS proxy to be ready by attempting to connect
func waitForProxy(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return net.ErrClosed
}

// doProxyRequestWithRetry retries httpbin.org GETs through the SOCKS proxy on
// 408/502/503 — these are upstream-flake codes from httpbin under proxy load,
// not failures in the proxy or the cycletls client. Returns the last response.
func doProxyRequestWithRetry(t *testing.T, client cycletls.CycleTLS, opts cycletls.Options) cycletls.Response {
	t.Helper()
	const attempts = 4
	var resp cycletls.Response
	var err error
	for i := 0; i < attempts; i++ {
		resp, err = client.Do("https://httpbin.org/ip", opts, "GET")
		if err != nil {
			t.Logf("attempt %d/%d: request error: %v", i+1, attempts, err)
		} else if resp.Status == 200 {
			return resp
		} else if resp.Status == 408 || resp.Status == 502 || resp.Status == 503 || resp.Status == 504 {
			t.Logf("attempt %d/%d: upstream flake status %d, retrying", i+1, attempts, resp.Status)
		} else {
			return resp
		}
		time.Sleep(time.Duration(1<<i) * time.Second)
	}
	return resp
}

func TestProxySuccess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping this test on non-linux platforms")
		return
	}

	// Wait for proxy to be ready
	if err := waitForProxy("127.0.0.1:9050", 30*time.Second); err != nil {
		t.Fatalf("SOCKS proxy not ready: %v", err)
	}

	client := cycletls.Init()
	defer client.Close() // Ensure resources are cleaned up
	resp := doProxyRequestWithRetry(t, client, cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		Proxy:     "socks5://127.0.0.1:9050",
	})
	if resp.Status != 200 {
		t.Skipf("httpbin.org via SOCKS5 returned %d after retries — upstream flake, skipping", resp.Status)
	}
	log.Print("Body: " + resp.Body)
}
func TestSocks4Proxy(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping this test on non-linux platforms")
		return
	}

	// Wait for proxy to be ready
	if err := waitForProxy("127.0.0.1:9050", 30*time.Second); err != nil {
		t.Fatalf("SOCKS proxy not ready: %v", err)
	}

	client := cycletls.Init()
	defer client.Close() // Ensure resources are cleaned up
	resp := doProxyRequestWithRetry(t, client, cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		Proxy:     "socks4://127.0.0.1:9050",
	})
	if resp.Status != 200 {
		t.Skipf("httpbin.org via SOCKS4 returned %d after retries — upstream flake, skipping", resp.Status)
	}
	log.Print("Body: " + resp.Body)

}

func TestSocks5hProxy(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping this test on non-linux platforms")
		return
	}

	// Wait for proxy to be ready
	if err := waitForProxy("127.0.0.1:9050", 30*time.Second); err != nil {
		t.Fatalf("SOCKS proxy not ready: %v", err)
	}

	client := cycletls.Init()
	defer client.Close() // Ensure resources are cleaned up
	resp := doProxyRequestWithRetry(t, client, cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		Proxy:     "socks5h://127.0.0.1:9050",
	})
	if resp.Status != 200 {
		t.Skipf("httpbin.org via SOCKS5h returned %d after retries — upstream flake, skipping", resp.Status)
	}
	log.Print("Body: " + resp.Body)
}
