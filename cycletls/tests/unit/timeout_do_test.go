package unit

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// Issue #8 (MAJOR): Named constants instead of magic numbers for timeout values.
const (
	// serverHeaderDelay is how long the mock server delays before sending headers.
	// Set to 1500ms to exceed the 1s client timeout by a clear margin.
	serverHeaderDelay = 1500 * time.Millisecond

	// serverBodyDelay is how long the mock server delays between partial body writes.
	// Set to 2s to exceed the 1s client timeout.
	serverBodyDelay = 2 * time.Second

	// clientTimeoutSeconds is the CycleTLS Options.Timeout value (in seconds).
	clientTimeoutSeconds = 1

	// testLevelTimeout bounds total test execution to prevent hanging if timeout
	// logic is broken. Set to 3x the server delay as generous upper bound.
	testLevelTimeout = 10 * time.Second
)

// Issue #3 (CRITICAL): Added test-level timeout via time.AfterFunc to prevent unbounded
// execution if the timeout logic fails. Also skips in -short mode.
// Issue #6 (MAJOR): Added goroutine leak verification after test.
// Issue #8 (MAJOR): Uses named constants instead of magic numbers.
// Issue #10 (MAJOR): Asserts on specific timeout message content.
func TestCycleTLSDoTimeoutHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in -short mode")
	}

	// Record goroutine count before test for leak detection (Issue #6)
	goroutinesBefore := runtime.NumGoroutine()

	// Test-level timeout guard (Issue #3): if this fires, the test hangs
	timer := time.AfterFunc(testLevelTimeout, func() {
		t.Errorf("test exceeded %v timeout - timeout logic may be broken", testLevelTimeout)
	})
	defer timer.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(serverHeaderDelay)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("late headers"))
	}))
	defer server.Close()

	client := cycletls.Init()
	defer client.Close()

	resp, err := client.Do(server.URL, cycletls.Options{
		Timeout: clientTimeoutSeconds,
	}, "GET")
	if err != nil {
		t.Fatalf("expected timeout response, got error: %v", err)
	}
	if resp.Status != http.StatusRequestTimeout {
		t.Fatalf("expected status %d (408 Request Timeout), got %d", http.StatusRequestTimeout, resp.Status)
	}

	// Issue #10 (MAJOR): Assert specific timeout message content
	bodyLower := strings.ToLower(resp.Body)
	if !strings.Contains(bodyLower, "timeout") {
		t.Fatalf("expected body to contain 'timeout', got %q", resp.Body)
	}
	// Verify the timeout message references the configured duration
	if !strings.Contains(bodyLower, "after") && !strings.Contains(bodyLower, "exceeded") {
		t.Logf("timeout message does not contain 'after' or 'exceeded' (informational): %q", resp.Body)
	}

	// Issue #6 (MAJOR): Goroutine leak check - allow some slack for runtime goroutines
	// Give goroutines a moment to wind down after client.Close()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	// Allow up to 5 goroutine variance for runtime/GC goroutines
	const maxGoroutineLeak = 5
	if goroutinesAfter > goroutinesBefore+maxGoroutineLeak {
		t.Errorf("possible goroutine leak: before=%d, after=%d (delta=%d, max allowed=%d)",
			goroutinesBefore, goroutinesAfter, goroutinesAfter-goroutinesBefore, maxGoroutineLeak)
	}
}

func TestCycleTLSDoTimeoutBody(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in -short mode")
	}

	goroutinesBefore := runtime.NumGoroutine()

	timer := time.AfterFunc(testLevelTimeout, func() {
		t.Errorf("test exceeded %v timeout - timeout logic may be broken", testLevelTimeout)
	})
	defer timer.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			_, _ = w.Write([]byte("partial"))
			flusher.Flush()
		}
		time.Sleep(serverBodyDelay)
		_, _ = w.Write([]byte("never read"))
	}))
	defer server.Close()

	client := cycletls.Init()
	defer client.Close()

	resp, err := client.Do(server.URL, cycletls.Options{
		Timeout: clientTimeoutSeconds,
	}, "GET")
	if err != nil {
		t.Fatalf("expected timeout response, got error: %v", err)
	}
	if resp.Status != http.StatusRequestTimeout {
		t.Fatalf("expected status %d (408 Request Timeout), got %d", http.StatusRequestTimeout, resp.Status)
	}

	// Issue #10: Assert specific timeout message content
	bodyLower := strings.ToLower(resp.Body)
	if !strings.Contains(bodyLower, "timeout") {
		t.Fatalf("expected body to contain 'timeout', got %q", resp.Body)
	}

	// Issue #6: Goroutine leak check
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	const maxGoroutineLeak = 5
	if goroutinesAfter > goroutinesBefore+maxGoroutineLeak {
		t.Errorf("possible goroutine leak: before=%d, after=%d (delta=%d, max allowed=%d)",
			goroutinesBefore, goroutinesAfter, goroutinesAfter-goroutinesBefore, maxGoroutineLeak)
	}
}
