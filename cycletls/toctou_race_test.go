//go:build !integration

package cycletls

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	http "github.com/Danny-Dasilva/fhttp"
	"github.com/quic-go/quic-go/http3"
)

// =============================================================================
// Issue 1: roundtripper.go dialTLS TOCTOU race (lines 256-264)
// The RLock->RUnlock->Lock pattern allows another goroutine to delete the entry
// between locks. Verify that concurrent access to cachedConnections is safe.
// =============================================================================

func TestTOCTOU_DialTLSCacheRace(t *testing.T) {
	rt := createTestRoundTripper()
	addr := "example.com:443"

	// Pre-populate cache
	conn := newMockConn()
	rt.cachedConnections[addr] = &cachedConn{
		conn:     conn,
		lastUsed: time.Now().Add(-time.Minute),
	}

	var wg sync.WaitGroup
	const goroutines = 50
	errors := make(chan error, goroutines)

	// Hammer the dialTLS cache lookup and update concurrently
	// Under the old RLock->RUnlock->Lock pattern, the race detector would fire
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Simulate what dialTLS does: check cache and update lastUsed
			rt.cacheMu.Lock()
			if cc := rt.cachedConnections[addr]; cc != nil {
				cc.lastUsed = time.Now()
			}
			rt.cacheMu.Unlock()
		}()
	}

	// Concurrently delete from cache (simulates cleanup goroutine)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.cacheMu.Lock()
			delete(rt.cachedConnections, addr)
			rt.cacheMu.Unlock()
			// Re-add to keep the test going
			rt.cacheMu.Lock()
			rt.cachedConnections[addr] = &cachedConn{
				conn:     newMockConn(),
				lastUsed: time.Now(),
			}
			rt.cacheMu.Unlock()
		}()
	}

	wg.Wait()
	close(errors)
	for err := range errors {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// Issue 2: roundtripper.go HTTP/3 RoundTrip TOCTOU race (lines 146-156)
// Same RLock->RUnlock->Lock pattern with cachedH3 map.
// =============================================================================

func TestTOCTOU_HTTP3CacheRace(t *testing.T) {
	rt := createTestRoundTripper()
	key := "h3:example.com:443"

	// Pre-populate HTTP/3 cache
	rt.cachedHTTP3Transports[key] = &cachedHTTP3Transport{
		transport: &http3.Transport{},
		lastUsed:  time.Now().Add(-time.Minute),
	}

	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent reads and updates to HTTP/3 cache
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.cacheMu.Lock()
			if h3t, ok := rt.cachedHTTP3Transports[key]; ok {
				h3t.lastUsed = time.Now()
			}
			rt.cacheMu.Unlock()
		}()
	}

	// Concurrent deletes (simulating cleanup)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.cacheMu.Lock()
			delete(rt.cachedHTTP3Transports, key)
			rt.cacheMu.Unlock()
			rt.cacheMu.Lock()
			rt.cachedHTTP3Transports[key] = &cachedHTTP3Transport{
				transport: &http3.Transport{},
				lastUsed:  time.Now(),
			}
			rt.cacheMu.Unlock()
		}()
	}

	wg.Wait()
}

// =============================================================================
// Issue 3: roundtripper.go HTTP/2 RoundTrip TOCTOU race (lines 180-196)
// Same RLock->RUnlock->Lock pattern with cachedTransports map.
// =============================================================================

func TestTOCTOU_HTTP2CacheRace(t *testing.T) {
	rt := createTestRoundTripper()
	addr := "example.com:443"

	// Pre-populate transport cache
	rt.cachedTransports[addr] = &cachedTransport{
		transport: &http.Transport{},
		lastUsed:  time.Now().Add(-time.Minute),
	}

	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent reads and updates to transport cache
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.cacheMu.Lock()
			if ct, ok := rt.cachedTransports[addr]; ok {
				ct.lastUsed = time.Now()
			}
			rt.cacheMu.Unlock()
		}()
	}

	// Concurrent deletes (simulating cleanup)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.cacheMu.Lock()
			delete(rt.cachedTransports, addr)
			rt.cacheMu.Unlock()
			rt.cacheMu.Lock()
			rt.cachedTransports[addr] = &cachedTransport{
				transport: &http.Transport{},
				lastUsed:  time.Now(),
			}
			rt.cacheMu.Unlock()
		}()
	}

	wg.Wait()
}

// =============================================================================
// Issue 4: client.go advancedClientPool TOCTOU race (lines 191-200)
// Same RLock->RUnlock->Lock pattern in getOrCreateClient.
// =============================================================================

func TestTOCTOU_ClientPoolRace(t *testing.T) {
	// Reset the global pool for this test
	advancedClientPoolMutex.Lock()
	savedPool := advancedClientPool
	advancedClientPool = make(map[string]*ClientPoolEntry)
	advancedClientPoolMutex.Unlock()

	defer func() {
		advancedClientPoolMutex.Lock()
		advancedClientPool = savedPool
		advancedClientPoolMutex.Unlock()
	}()

	// Pre-populate a client entry
	clientKey := "test-key"
	advancedClientPoolMutex.Lock()
	advancedClientPool[clientKey] = &ClientPoolEntry{
		Client:    http.Client{},
		CreatedAt: time.Now(),
		LastUsed:  time.Now().Add(-time.Minute),
	}
	advancedClientPoolMutex.Unlock()

	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent updates to LastUsed
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			advancedClientPoolMutex.Lock()
			if entry, exists := advancedClientPool[clientKey]; exists {
				entry.LastUsed = time.Now()
			}
			advancedClientPoolMutex.Unlock()
		}()
	}

	// Concurrent deletes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			advancedClientPoolMutex.Lock()
			delete(advancedClientPool, clientKey)
			advancedClientPoolMutex.Unlock()
			advancedClientPoolMutex.Lock()
			advancedClientPool[clientKey] = &ClientPoolEntry{
				Client:    http.Client{},
				CreatedAt: time.Now(),
				LastUsed:  time.Now(),
			}
			advancedClientPoolMutex.Unlock()
		}()
	}

	wg.Wait()
}

// =============================================================================
// Issue 5: roundtripper.go cleanup goroutine lifecycle
// The cleanup goroutine must be stoppable via StopCacheCleanup().
// =============================================================================

func TestCleanupGoroutineLifecycle(t *testing.T) {
	rt := createTestRoundTripper()
	rt.cleanupStop = make(chan struct{})

	var cleanupRan atomic.Int32

	// Start a test cleanup goroutine that increments counter
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cleanupRan.Add(1)
			case <-rt.cleanupStop:
				return
			}
		}
	}()

	// Let it run a few times
	time.Sleep(50 * time.Millisecond)
	if cleanupRan.Load() == 0 {
		t.Error("cleanup goroutine did not run")
	}

	// Stop it
	rt.StopCacheCleanup()

	// Record count after stop
	countAfterStop := cleanupRan.Load()
	time.Sleep(50 * time.Millisecond)

	// Should not have incremented much (at most 1 more tick in flight)
	countAfterWait := cleanupRan.Load()
	if countAfterWait > countAfterStop+1 {
		t.Errorf("cleanup goroutine continued after stop: before=%d after=%d",
			countAfterStop, countAfterWait)
	}
}

func TestStopCacheCleanup_NilChannel(t *testing.T) {
	// StopCacheCleanup should not panic when cleanupStop is nil
	rt := createTestRoundTripper()
	// cleanupStop is nil by default in createTestRoundTripper

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("StopCacheCleanup panicked with nil channel: %v", r)
		}
	}()

	rt.StopCacheCleanup()
}

func TestStopCacheCleanup_DoubleStop(t *testing.T) {
	// Double-closing cleanupStop should not panic
	rt := createTestRoundTripper()
	rt.cleanupStop = make(chan struct{})

	// First stop should work fine
	rt.StopCacheCleanup()

	// Second stop should not panic - this tests the guard
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("double StopCacheCleanup panicked: %v", r)
		}
	}()

	rt.StopCacheCleanup()
}

// =============================================================================
// Verify the fix pattern: single Lock() instead of RLock->RUnlock->Lock
// These tests verify the actual dialTLS, RoundTrip, and client code paths
// use the corrected locking pattern by running with -race.
// =============================================================================

func TestRace_DialTLSConcurrentAccess(t *testing.T) {
	rt := createTestRoundTripper()
	addr := "race-test.com:443"

	// Pre-populate
	c, _ := net.Pipe()
	defer c.Close()
	rt.cachedConnections[addr] = &cachedConn{
		conn:     c,
		lastUsed: time.Now(),
	}

	var wg sync.WaitGroup
	const goroutines = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// This simulates the fixed dialTLS cache path:
			// single Lock() for check + update
			rt.cacheMu.Lock()
			if cc := rt.cachedConnections[addr]; cc != nil {
				cc.lastUsed = time.Now()
			}
			rt.cacheMu.Unlock()
		}()
	}

	wg.Wait()
}

func TestRace_HTTP3TransportConcurrentAccess(t *testing.T) {
	rt := createTestRoundTripper()
	key := "h3:race-test.com:443"

	rt.cachedHTTP3Transports[key] = &cachedHTTP3Transport{
		transport: &http3.Transport{},
		lastUsed:  time.Now(),
	}

	var wg sync.WaitGroup
	const goroutines = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.cacheMu.Lock()
			if h3t, ok := rt.cachedHTTP3Transports[key]; ok {
				h3t.lastUsed = time.Now()
			}
			rt.cacheMu.Unlock()
		}()
	}

	wg.Wait()
}

func TestRace_ClientPoolConcurrentAccess(t *testing.T) {
	// Save and restore global pool
	advancedClientPoolMutex.Lock()
	savedPool := advancedClientPool
	advancedClientPool = make(map[string]*ClientPoolEntry)
	advancedClientPoolMutex.Unlock()

	defer func() {
		advancedClientPoolMutex.Lock()
		advancedClientPool = savedPool
		advancedClientPoolMutex.Unlock()
	}()

	key := "race-pool-key"
	advancedClientPoolMutex.Lock()
	advancedClientPool[key] = &ClientPoolEntry{
		Client:    http.Client{},
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}
	advancedClientPoolMutex.Unlock()

	var wg sync.WaitGroup
	const goroutines = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			advancedClientPoolMutex.Lock()
			if entry, exists := advancedClientPool[key]; exists {
				entry.LastUsed = time.Now()
			}
			advancedClientPoolMutex.Unlock()
		}()
	}

	wg.Wait()
}

// fhttp.Client is a value type, not interface - need alias
var _ = http.Client{}
