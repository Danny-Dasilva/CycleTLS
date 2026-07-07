//go:build integration
// +build integration

package tlsfingerprint_test

import (
	"testing"
	"time"
)

func TestDelay(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()

	// Measure time before request
	start := time.Now()

	resp, err := client.Do(TestServerURL+"/delay/1", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// Measure elapsed time
	elapsed := time.Since(start)

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var delayResp DelayResponse
	parseJSONResponse(t, resp.Body, &delayResp)

	// Verify delay field in response
	if delayResp.Delay != 1 {
		t.Errorf("Expected delay=1 in response, got %d", delayResp.Delay)
	}

	// Verify the request actually took at least 1 second
	if elapsed < time.Second {
		t.Errorf("Expected request to take at least 1 second, took %v", elapsed)
	}
}
