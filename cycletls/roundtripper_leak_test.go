//go:build !integration

package cycletls

import (
	"fmt"
	"net"
	"testing"
	"time"

	http "github.com/Danny-Dasilva/fhttp"
	utls "github.com/refraction-networking/utls"
)

// seedAddressMutexAndCache creates a per-address mutex plus matching cached
// connection and transport entries for addr, mirroring what getTransport +
// cacheTransportAndConnection produce for a live address.
func seedAddressMutexAndCache(rt *roundTripper, addr string, lastUsed time.Time) {
	_ = rt.getAddressMutex(addr) // registers rt.addressMutexes[addr]
	rt.cacheMu.Lock()
	rt.cachedConnections[addr] = &cachedConn{conn: newMockConn(), lastUsed: lastUsed}
	rt.cachedTransports[addr] = &cachedTransport{transport: &http.Transport{}, lastUsed: lastUsed}
	rt.cacheMu.Unlock()
}

// TestAddressMutexes_PrunedByCloseIdleConnections verifies that closing all
// connections removes every per-address mutex, so addressMutexes cannot leak an
// entry per distinct host for the process lifetime (M2).
func TestAddressMutexes_PrunedByCloseIdleConnections(t *testing.T) {
	rt := createTestRoundTripper()

	const n = 20
	for i := 0; i < n; i++ {
		seedAddressMutexAndCache(rt, fmt.Sprintf("host-%d.example:443", i), time.Now())
	}

	if got := len(rt.addressMutexes); got != n {
		t.Fatalf("setup: expected %d address mutexes, got %d", n, got)
	}

	// Close-all evicts every connection/transport; prune must drop every mutex.
	rt.CloseIdleConnections()

	if got := len(rt.addressMutexes); got != 0 {
		t.Fatalf("expected addressMutexes fully pruned after CloseIdleConnections, got %d retained", got)
	}
	if got := len(rt.cachedConnections); got != 0 {
		t.Fatalf("expected all connections closed, got %d retained", got)
	}
}

// TestAddressMutexes_PrunedForEvictedAddressesOnly verifies the selective-close
// path keeps the mutex for the retained address and prunes the rest.
func TestAddressMutexes_PrunedForEvictedAddressesOnly(t *testing.T) {
	rt := createTestRoundTripper()

	keep := "keep.example:443"
	seedAddressMutexAndCache(rt, keep, time.Now())
	for i := 0; i < 10; i++ {
		seedAddressMutexAndCache(rt, fmt.Sprintf("drop-%d.example:443", i), time.Now())
	}

	// Keep only `keep`, close and prune the rest.
	rt.CloseIdleConnections(keep)

	if _, ok := rt.addressMutexes[keep]; !ok {
		t.Fatalf("expected mutex for retained address %q to be kept", keep)
	}
	if got := len(rt.addressMutexes); got != 1 {
		t.Fatalf("expected exactly 1 retained address mutex, got %d", got)
	}
}

// TestAddressMutexes_PrunedByCleanupCache verifies the periodic cleanup path
// prunes mutexes for age-evicted addresses.
func TestAddressMutexes_PrunedByCleanupCache(t *testing.T) {
	rt := createTestRoundTripper()

	old := time.Now().Add(-2 * connectionMaxAge) // guaranteed past the age cutoff
	const n = 15
	for i := 0; i < n; i++ {
		seedAddressMutexAndCache(rt, fmt.Sprintf("stale-%d.example:443", i), old)
	}

	rt.cleanupCache()

	if got := len(rt.cachedConnections); got != 0 {
		t.Fatalf("expected stale connections evicted, got %d retained", got)
	}
	if got := len(rt.addressMutexes); got != 0 {
		t.Fatalf("expected addressMutexes pruned after cleanupCache, got %d retained", got)
	}
}

// TestCacheInsert_EnforcesBoundWithoutTicker verifies the transport and
// connection caches stay within their maximum size when many distinct addresses
// are inserted rapidly, WITHOUT waiting for the 5-minute cleanup ticker (m3).
func TestCacheInsert_EnforcesBoundWithoutTicker(t *testing.T) {
	rt := createTestRoundTripper()

	total := maxCachedConnections + 50
	pipes := make([]net.Conn, 0, total)
	defer func() {
		for _, c := range pipes {
			_ = c.Close()
		}
	}()

	for i := 0; i < total; i++ {
		addr := fmt.Sprintf("bound-%d.example:443", i)
		// net.Pipe gives an in-memory conn; utls wraps it without handshaking.
		// cacheTransportAndConnection reads ConnectionState().NegotiatedProtocol
		// (empty pre-handshake -> HTTP/1.x branch) and caches the entry.
		local, remote := net.Pipe()
		pipes = append(pipes, remote)
		uconn := utls.UClient(local, &utls.Config{ServerName: "x", InsecureSkipVerify: true}, utls.HelloChrome_Auto)
		if err := rt.cacheTransportAndConnection(addr, uconn, time.Now()); err != nil {
			t.Fatalf("cacheTransportAndConnection returned error: %v", err)
		}
	}

	rt.cacheMu.RLock()
	nConns := len(rt.cachedConnections)
	nTransports := len(rt.cachedTransports)
	rt.cacheMu.RUnlock()

	if nConns > maxCachedConnections {
		t.Errorf("cachedConnections exceeded bound: got %d, want <= %d", nConns, maxCachedConnections)
	}
	if nTransports > maxCachedTransports {
		t.Errorf("cachedTransports exceeded bound: got %d, want <= %d", nTransports, maxCachedTransports)
	}
}
