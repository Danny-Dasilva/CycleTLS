package cycletls_test

import (
	"testing"
	"time"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// doHTTPBinRequestWithRetry retries httpbin.org-style external requests on
// 408/502/503/504 — these are upstream-flake codes from rate-limited public
// fixtures, not failures in cycletls itself. Returns the last response so the
// caller can still see the final status (and can choose to t.Skipf when the
// flake persists across all retries).
//
// Backoff is intentionally short (500ms, 1s, 2s) so the total retry budget
// stays under ~5s even when all attempts are needed; httpbin's 408s clear
// quickly on retry and we don't want to exhaust the test deadline in CI.
//
// Mirrors doProxyRequestWithRetry from proxy_test.go but accepts an explicit
// URL/method so it can be reused across the binary, image, and form tests.
func doHTTPBinRequestWithRetry(t *testing.T, client cycletls.CycleTLS, url string, opts cycletls.Options, method string) cycletls.Response {
	t.Helper()
	const attempts = 3
	backoffs := []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second}
	var resp cycletls.Response
	var err error
	for i := 0; i < attempts; i++ {
		resp, err = client.Do(url, opts, method)
		if err != nil {
			t.Logf("attempt %d/%d: request error: %v", i+1, attempts, err)
		} else if resp.Status >= 200 && resp.Status < 400 {
			return resp
		} else if resp.Status == 408 || resp.Status == 502 || resp.Status == 503 || resp.Status == 504 {
			t.Logf("attempt %d/%d: upstream flake status %d, retrying", i+1, attempts, resp.Status)
		} else {
			// Non-flake error (e.g. 4xx other than 408) — return immediately
			return resp
		}
		if i < len(backoffs) {
			time.Sleep(backoffs[i])
		}
	}
	return resp
}

// isUpstreamFlake returns true if the status code is one we treat as an
// httpbin upstream flake (rate limit, gateway timeout, etc.).
func isUpstreamFlake(status int) bool {
	return status == 408 || status == 502 || status == 503 || status == 504
}
