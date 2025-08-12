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
	resp, err := client.Do("https://httpbin.org/ip", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		Proxy:     "socks5://127.0.0.1:9050",
	}, "GET")
	if err != nil {
		t.Fatalf("Request Failed: %s", err.Error())
	}
	if resp.Status != 200 {
		t.Fatalf("Expected %d Got %d for Status", 200, resp.Status)
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
	resp, err := client.Do("https://httpbin.org/ip", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		Proxy:     "socks4://127.0.0.1:9050",
	}, "GET")
	if err != nil {
		t.Fatalf("Request Failed: %s", err.Error())
	}
	if resp.Status != 200 {
		t.Fatalf("Expected %d Got %d for Status", 200, resp.Status)
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
	resp, err := client.Do("https://httpbin.org/ip", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		Proxy:     "socks5h://127.0.0.1:9050",
	}, "GET")
	if err != nil {
		t.Fatalf("Request Failed: %s", err.Error())
	}
	if resp.Status != 200 {
		t.Fatalf("Expected %d Got %d for Status", 200, resp.Status)
	}
	log.Print("Body: " + resp.Body)
}
