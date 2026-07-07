//go:build !integration

package cycletls

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	http "github.com/Danny-Dasilva/fhttp"
)

// Test that getTransport no longer panics when dialTLS returns (conn, nil)
// due to a cached connection/transport already being present for the address.
func TestGetTransport_NoPanicWhenCachedConnPresent(t *testing.T) {
	// Create a new roundTripper
	rtIface := newRoundTripper(Browser{})

	// Type assert to concrete type to access internals
	rt, ok := rtIface.(*roundTripper)
	if !ok {
		t.Fatalf("expected *roundTripper, got %T", rtIface)
	}

	// Build a simple HTTPS request
	req, err := http.NewRequest("GET", "https://example.com/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Determine the addr key used by roundTripper
	addr := rt.getDialTLSAddr(req)

	// Simulate a previously established (cached) TLS connection and transport
	c1, _ := net.Pipe()
	defer c1.Close()
	now := time.Now()
	rt.cachedConnections[addr] = &cachedConn{
		conn:     c1,
		lastUsed: now,
	}
	rt.cachedTransports[addr] = &cachedTransport{
		transport: &http.Transport{},
		lastUsed:  now,
	}

	// Ensure no panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("did not expect panic: %v", r)
		}
	}()

	if err := rt.getTransport(req, addr); err != nil {
		t.Fatalf("getTransport returned error: %v", err)
	}
}

// barrierDialer is a proxy.ContextDialer that lets a test observe how many
// DialContext calls are in flight simultaneously. Each call registers its
// arrival on `started`, then blocks until `release` is closed. It returns an
// error instead of a real conn, so dialTLS returns before attempting a TLS
// handshake (no network needed).
type barrierDialer struct {
	started chan struct{}
	release chan struct{}

	mu      sync.Mutex
	current int
	maxSeen int
}

func (d *barrierDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d.mu.Lock()
	d.current++
	if d.current > d.maxSeen {
		d.maxSeen = d.current
	}
	d.mu.Unlock()

	// Signal arrival, then wait for the test to release all dials at once.
	d.started <- struct{}{}
	<-d.release

	d.mu.Lock()
	d.current--
	d.mu.Unlock()
	return nil, fmt.Errorf("barrierDialer: no real connection")
}

// TestDialTLS_DoesNotGloballySerialize proves that dialTLS does NOT hold a
// global mutex across the dial: many goroutines dialing DISTINCT addresses must
// be able to sit inside DialContext concurrently. Before the fix, dialTLS held
// rt.Lock() across DialContext+Handshake, so only ONE goroutine could enter the
// dial at a time and the barrier would never reach N (the test would time out).
func TestDialTLS_DoesNotGloballySerialize(t *testing.T) {
	const n = 8

	d := &barrierDialer{
		started: make(chan struct{}, n),
		release: make(chan struct{}),
	}

	rt := createTestRoundTripper()
	rt.dialer = d

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			addr := fmt.Sprintf("host-%d.example:443", i)
			// We only care that DialContext is reached concurrently; the dial
			// itself returns an error, so dialTLS returns without handshaking.
			_, _ = rt.dialTLS(context.Background(), "tcp", addr)
		}(i)
	}

	// Wait until all n dials are simultaneously parked inside DialContext.
	// If dialTLS serialized dials with a global lock, only one goroutine would
	// arrive and this would time out.
	deadline := time.After(5 * time.Second)
	for arrived := 0; arrived < n; arrived++ {
		select {
		case <-d.started:
		case <-deadline:
			close(d.release) // unblock any parked dials before failing
			t.Fatalf("timed out waiting for concurrent dials: only %d/%d reached DialContext (global serialization?)", arrived, n)
		}
	}

	// All n reached the dial concurrently — release them.
	close(d.release)
	wg.Wait()

	d.mu.Lock()
	maxSeen := d.maxSeen
	d.mu.Unlock()
	if maxSeen != n {
		t.Fatalf("expected %d concurrent dials, observed max %d", n, maxSeen)
	}
}

// TestCleanupStartStop_NoRaceNoDoubleClose spawns cleanup-goroutine starts
// concurrently with many StopCacheCleanup / CloseIdleConnections callers and
// asserts there is neither a double-close panic nor a data race on cleanupStop.
// Run under `go test -race`.
func TestCleanupStartStop_NoRaceNoDoubleClose(t *testing.T) {
	rt := newRoundTripper(Browser{}).(*roundTripper)

	var wg sync.WaitGroup
	const workers = 24

	// Starters: emulate the RoundTrip first-use path that launches the cleanup
	// goroutine (which reads rt.cleanupStop).
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.cleanupOnce.Do(func() { go rt.startCacheCleanup() })
		}()
	}

	// Direct stoppers.
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.StopCacheCleanup()
		}()
	}

	// CloseIdleConnections also stops cleanup internally (close-all branch).
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.CloseIdleConnections()
		}()
	}

	wg.Wait()

	// A final stop must still be safe (exactly-once close).
	rt.StopCacheCleanup()
}
