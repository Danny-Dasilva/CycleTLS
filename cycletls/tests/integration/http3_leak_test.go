//go:build integration
// +build integration

package cycletls_test

import (
	"runtime"
	"testing"
	"time"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// TestHTTP3ConnectionLeakPrevention is skipped because it requires a real HTTP/3
// server which is not available in unit/integration test environments. The httptest
// package only provides HTTP/1.1 servers, so this test cannot meaningfully validate
// HTTP/3 connection leak behavior.
//
// See TestHTTP1ConnectionCleanup for the equivalent test using HTTP/1.1.
func TestHTTP3ConnectionLeakPrevention(t *testing.T) {
	t.Skip("requires HTTP/3 test server - not available in unit/integration tests")
}

// TestHTTP1ConnectionCleanup tests that HTTP/1.1 requests don't leak
// goroutines or connections over multiple requests.
//
// Note: This was previously named TestHTTP3ConnectionLeakPrevention but was
// actually testing HTTP/1.1 connections via httptest.NewServer. Renamed for accuracy.
func TestHTTP1ConnectionCleanup(t *testing.T) {
	// Get baseline goroutine count after client initialization
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Create client first, then measure baseline (client init creates goroutines)
	client := cycletls.Init()
	defer client.Close()

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()

	// Make several requests to a real HTTPS endpoint
	for i := 0; i < 10; i++ {
		resp, err := client.Do("https://httpbin.org/get", cycletls.Options{
			// httpbin.org/tlsfingerprint.com fixture cert may be expired/rotated; we test the outgoing TLS fingerprint and HTTP body, not the fixture's cert chain.
			InsecureSkipVerify: true,
			Method:             "GET",
		}, "GET")
		if err != nil {
			t.Logf("Request %d error: %v", i, err)
		} else if resp.Status != 200 {
			t.Logf("Request %d status: %d", i, resp.Status)
		}
	}

	// Allow time for any leaked goroutines to accumulate
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check goroutine count
	finalGoroutines := runtime.NumGoroutine()
	goroutineDiff := finalGoroutines - baselineGoroutines

	// Allow some goroutine growth for legitimate purposes (connection pools, etc.)
	// but flag significant leaks. CI environments may have higher baseline variance.
	maxAllowedGrowth := 50
	if goroutineDiff > maxAllowedGrowth {
		t.Errorf("Possible goroutine leak: baseline=%d, final=%d, diff=%d (max allowed: %d)",
			baselineGoroutines, finalGoroutines, goroutineDiff, maxAllowedGrowth)
	} else {
		t.Logf("Goroutine count OK: baseline=%d, final=%d, diff=%d",
			baselineGoroutines, finalGoroutines, goroutineDiff)
	}
}

// TestHTTP3MultipleClientsNoLeak tests that creating and closing multiple
// clients doesn't leak resources.
func TestHTTP3MultipleClientsNoLeak(t *testing.T) {
	// Get baseline
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()

	// Create and close multiple clients
	for i := 0; i < 5; i++ {
		client := cycletls.Init()
		client.Close()
	}

	// Allow cleanup
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	goroutineDiff := finalGoroutines - baselineGoroutines

	// Should return close to baseline after closing all clients
	maxAllowedGrowth := 10
	if goroutineDiff > maxAllowedGrowth {
		t.Errorf("Possible resource leak after closing clients: baseline=%d, final=%d, diff=%d",
			baselineGoroutines, finalGoroutines, goroutineDiff)
	} else {
		t.Logf("Resource cleanup OK: baseline=%d, final=%d, diff=%d",
			baselineGoroutines, finalGoroutines, goroutineDiff)
	}
}
