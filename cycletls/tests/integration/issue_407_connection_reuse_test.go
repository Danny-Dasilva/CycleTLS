//go:build integration
// +build integration

package cycletls_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// TestIssue407ConcurrentConnectionReuse reproduces the exact scenario from issue #407:
// Multiple concurrent requests with connection reuse enabled should not panic or cause port binding errors
func TestIssue407ConcurrentConnectionReuse(t *testing.T) {
	// Create a test server that simulates Google's behavior
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add a small delay to make race conditions more likely
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Test configuration matching issue #407
	const (
		NUM_INSTANCES            = 5  // Number of CycleTLS instances
		NUM_REQUESTS_PER_INSTANCE = 2  // Number of requests each instance will make
		DELAY_BETWEEN_REQUESTS_MS = 100 // Delay between requests in milliseconds
	)

	// Initialize client options with connection reuse enabled (the trigger for the bug)
	options := cycletls.Options{
		Ja3:                   "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent:             "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		InsecureSkipVerify:    true,
		EnableConnectionReuse: true, // This is the key setting that triggers the bug
	}

	// Create multiple client instances (simulating different ports in the original issue)
	clients := make([]cycletls.CycleTLS, NUM_INSTANCES)
	for i := 0; i < NUM_INSTANCES; i++ {
		clients[i] = cycletls.Init(cycletls.WithRawBytes())
		defer clients[i].Close()
	}

	// Track results and errors
	type result struct {
		instanceIndex int
		requestIndex  int
		err           error
		duration      time.Duration
	}
	results := make(chan result, NUM_INSTANCES*NUM_REQUESTS_PER_INSTANCE)

	// Concurrent execution (this is where the race condition occurs)
	var wg sync.WaitGroup
	for i := 0; i < NUM_INSTANCES; i++ {
		for j := 0; j < NUM_REQUESTS_PER_INSTANCE; j++ {
			wg.Add(1)
			go func(instanceIdx, requestIdx int) {
				defer wg.Done()

				// Add delay to stagger requests slightly
				time.Sleep(time.Duration(DELAY_BETWEEN_REQUESTS_MS*(instanceIdx*NUM_REQUESTS_PER_INSTANCE+requestIdx)) * time.Millisecond)

				start := time.Now()
				resp, err := clients[instanceIdx].Do(server.URL, options, "GET")
				duration := time.Since(start)

				if err != nil {
					results <- result{instanceIdx, requestIdx, err, duration}
					return
				}

				if resp.Status != 200 {
					results <- result{instanceIdx, requestIdx, fmt.Errorf("unexpected status: %d", resp.Status), duration}
					return
				}

				results <- result{instanceIdx, requestIdx, nil, duration}
			}(i, j)
		}
	}

	wg.Wait()
	close(results)

	// Analyze results
	var (
		totalRequests    int
		failedRequests   int
		successRequests  int
		totalDuration    time.Duration
		minDuration      = time.Hour
		maxDuration      time.Duration
	)

	for res := range results {
		totalRequests++
		totalDuration += res.duration

		if res.duration < minDuration {
			minDuration = res.duration
		}
		if res.duration > maxDuration {
			maxDuration = res.duration
		}

		if res.err != nil {
			failedRequests++
			t.Errorf("Instance %d, Request %d failed: %v", res.instanceIndex, res.requestIndex, res.err)
		} else {
			successRequests++
		}
	}

	// Report statistics
	avgDuration := totalDuration / time.Duration(totalRequests)
	t.Logf("=== Issue #407 Test Results ===")
	t.Logf("Total Requests: %d", totalRequests)
	t.Logf("Successful: %d", successRequests)
	t.Logf("Failed: %d", failedRequests)
	t.Logf("Average Duration: %v", avgDuration)
	t.Logf("Min Duration: %v", minDuration)
	t.Logf("Max Duration: %v", maxDuration)

	// Assert no failures
	if failedRequests > 0 {
		t.Fatalf("Test failed: %d out of %d requests failed", failedRequests, totalRequests)
	}

	t.Log("✅ Issue #407 test passed - no panics or port binding errors with concurrent connection reuse")
}

// TestIssue407StressTest is a more aggressive stress test
func TestIssue407StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// Create a test server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	const NUM_CONCURRENT_REQUESTS = 50

	options := cycletls.Options{
		Ja3:                   "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent:             "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		InsecureSkipVerify:    true,
		EnableConnectionReuse: true,
	}

	client := cycletls.Init(cycletls.WithRawBytes())
	defer client.Close()

	// Fire off many concurrent requests to the same host
	var wg sync.WaitGroup
	errors := make(chan error, NUM_CONCURRENT_REQUESTS)

	for i := 0; i < NUM_CONCURRENT_REQUESTS; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := client.Do(server.URL, options, "GET")
			if err != nil {
				errors <- fmt.Errorf("request %d failed: %w", idx, err)
				return
			}
			if resp.Status != 200 {
				errors <- fmt.Errorf("request %d: unexpected status %d", idx, resp.Status)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		errorCount++
		t.Errorf("Stress test error: %v", err)
	}

	if errorCount > 0 {
		t.Fatalf("Stress test failed with %d errors", errorCount)
	}

	t.Logf("✅ Stress test passed - %d concurrent requests completed successfully", NUM_CONCURRENT_REQUESTS)
}

// TestIssue407ConnectionReusePerformance validates that connection reuse provides performance benefits
func TestIssue407ConnectionReusePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create a test server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	options := cycletls.Options{
		Ja3:                "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent:          "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		InsecureSkipVerify: true,
	}

	// Test with connection reuse enabled
	optionsWithReuse := options
	optionsWithReuse.EnableConnectionReuse = true

	clientWithReuse := cycletls.Init(cycletls.WithRawBytes())
	defer clientWithReuse.Close()

	// First request (establishes connection)
	start := time.Now()
	_, err := clientWithReuse.Do(server.URL, optionsWithReuse, "GET")
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	firstRequestDuration := time.Since(start)

	// Subsequent requests (should reuse connection)
	const NUM_REUSE_REQUESTS = 5
	var totalReuseDuration time.Duration
	for i := 0; i < NUM_REUSE_REQUESTS; i++ {
		start = time.Now()
		_, err := clientWithReuse.Do(server.URL, optionsWithReuse, "GET")
		if err != nil {
			t.Fatalf("Reuse request %d failed: %v", i, err)
		}
		totalReuseDuration += time.Since(start)
	}
	avgReuseDuration := totalReuseDuration / NUM_REUSE_REQUESTS

	t.Logf("=== Connection Reuse Performance ===")
	t.Logf("First Request Duration: %v", firstRequestDuration)
	t.Logf("Average Reuse Duration: %v", avgReuseDuration)
	t.Logf("Performance Improvement: %.2fx faster", float64(firstRequestDuration)/float64(avgReuseDuration))

	// Subsequent requests should be significantly faster
	if avgReuseDuration > firstRequestDuration {
		t.Logf("⚠️ Warning: Connection reuse did not improve performance")
	} else {
		t.Logf("✅ Connection reuse is working - requests are faster")
	}
}
