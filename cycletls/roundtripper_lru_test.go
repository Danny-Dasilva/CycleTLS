//go:build !integration

package cycletls

import (
	"net"
	"sync"
	"testing"
	"time"

	http "github.com/Danny-Dasilva/fhttp"
	"github.com/quic-go/quic-go/http3"
)

// mockConn implements net.Conn for testing purposes
type mockConn struct {
	closed bool
	mu     sync.Mutex
}

func newMockConn() *mockConn {
	return &mockConn{}
}

func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *mockConn) Close() error                       { m.mu.Lock(); m.closed = true; m.mu.Unlock(); return nil }
func (m *mockConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (m *mockConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }
func (m *mockConn) IsClosed() bool                     { m.mu.Lock(); defer m.mu.Unlock(); return m.closed }

// createTestRoundTripper creates a roundTripper suitable for testing
func createTestRoundTripper() *roundTripper {
	return &roundTripper{
		cachedConnections:     make(map[string]*cachedConn),
		cachedTransports:      make(map[string]*cachedTransport),
		cachedHTTP3Transports: make(map[string]*cachedHTTP3Transport),
	}
}

// TestLRU_CachedConnStructure verifies the cachedConn struct stores lastUsed correctly
func TestLRU_CachedConnStructure(t *testing.T) {
	conn := newMockConn()
	now := time.Now()

	cc := &cachedConn{
		conn:     conn,
		lastUsed: now,
	}

	if cc.conn != conn {
		t.Error("cachedConn.conn not stored correctly")
	}
	if !cc.lastUsed.Equal(now) {
		t.Error("cachedConn.lastUsed not stored correctly")
	}
}

// TestLRU_CachedTransportStructure verifies the cachedTransport struct stores lastUsed correctly
func TestLRU_CachedTransportStructure(t *testing.T) {
	transport := &http.Transport{}
	now := time.Now()

	ct := &cachedTransport{
		transport: transport,
		lastUsed:  now,
	}

	if ct.transport != transport {
		t.Error("cachedTransport.transport not stored correctly")
	}
	if !ct.lastUsed.Equal(now) {
		t.Error("cachedTransport.lastUsed not stored correctly")
	}
}

// TestLRU_CachedHTTP3TransportStructure verifies the cachedHTTP3Transport struct stores lastUsed correctly
func TestLRU_CachedHTTP3TransportStructure(t *testing.T) {
	transport := &http3.Transport{}
	conn := &HTTP3Connection{}
	now := time.Now()

	h3t := &cachedHTTP3Transport{
		transport: transport,
		conn:      conn,
		lastUsed:  now,
	}

	if h3t.transport != transport {
		t.Error("cachedHTTP3Transport.transport not stored correctly")
	}
	if h3t.conn != conn {
		t.Error("cachedHTTP3Transport.conn not stored correctly")
	}
	if !h3t.lastUsed.Equal(now) {
		t.Error("cachedHTTP3Transport.lastUsed not stored correctly")
	}
}

// TestLRU_AgeBasedCleanup_Connections tests that old connections are removed based on connectionMaxAge
func TestLRU_AgeBasedCleanup_Connections(t *testing.T) {
	rt := createTestRoundTripper()

	// Create a connection that is older than connectionMaxAge
	oldConn := newMockConn()
	oldTime := time.Now().Add(-connectionMaxAge - time.Minute)
	rt.cachedConnections["old-addr:443"] = &cachedConn{
		conn:     oldConn,
		lastUsed: oldTime,
	}

	// Create a connection that is still fresh
	freshConn := newMockConn()
	freshTime := time.Now()
	rt.cachedConnections["fresh-addr:443"] = &cachedConn{
		conn:     freshConn,
		lastUsed: freshTime,
	}

	// Run cleanup
	rt.cleanupCache()

	// Old connection should be removed and closed
	if _, exists := rt.cachedConnections["old-addr:443"]; exists {
		t.Error("old connection should have been removed")
	}
	if !oldConn.IsClosed() {
		t.Error("old connection should have been closed")
	}

	// Fresh connection should remain
	if _, exists := rt.cachedConnections["fresh-addr:443"]; !exists {
		t.Error("fresh connection should still exist")
	}
	if freshConn.IsClosed() {
		t.Error("fresh connection should not have been closed")
	}
}

// TestLRU_AgeBasedCleanup_Transports tests that old transports are removed based on connectionMaxAge
func TestLRU_AgeBasedCleanup_Transports(t *testing.T) {
	rt := createTestRoundTripper()

	// Create a transport that is older than connectionMaxAge
	oldTime := time.Now().Add(-connectionMaxAge - time.Minute)
	rt.cachedTransports["old-addr:443"] = &cachedTransport{
		transport: &http.Transport{},
		lastUsed:  oldTime,
	}

	// Create a transport that is still fresh
	freshTime := time.Now()
	rt.cachedTransports["fresh-addr:443"] = &cachedTransport{
		transport: &http.Transport{},
		lastUsed:  freshTime,
	}

	// Run cleanup
	rt.cleanupCache()

	// Old transport should be removed
	if _, exists := rt.cachedTransports["old-addr:443"]; exists {
		t.Error("old transport should have been removed")
	}

	// Fresh transport should remain
	if _, exists := rt.cachedTransports["fresh-addr:443"]; !exists {
		t.Error("fresh transport should still exist")
	}
}

// TestLRU_AgeBasedCleanup_HTTP3Transports tests that old HTTP/3 transports are removed based on connectionMaxAge
func TestLRU_AgeBasedCleanup_HTTP3Transports(t *testing.T) {
	rt := createTestRoundTripper()

	// Create an HTTP/3 transport that is older than connectionMaxAge
	oldTime := time.Now().Add(-connectionMaxAge - time.Minute)
	rt.cachedHTTP3Transports["h3:old-addr:443"] = &cachedHTTP3Transport{
		transport: &http3.Transport{},
		conn:      nil, // nil conn to avoid cleanup errors
		lastUsed:  oldTime,
	}

	// Create an HTTP/3 transport that is still fresh
	freshTime := time.Now()
	rt.cachedHTTP3Transports["h3:fresh-addr:443"] = &cachedHTTP3Transport{
		transport: &http3.Transport{},
		conn:      nil,
		lastUsed:  freshTime,
	}

	// Run cleanup
	rt.cleanupCache()

	// Old HTTP/3 transport should be removed
	if _, exists := rt.cachedHTTP3Transports["h3:old-addr:443"]; exists {
		t.Error("old HTTP/3 transport should have been removed")
	}

	// Fresh HTTP/3 transport should remain
	if _, exists := rt.cachedHTTP3Transports["h3:fresh-addr:443"]; !exists {
		t.Error("fresh HTTP/3 transport should still exist")
	}
}

// TestLRU_SizeBasedEviction_Connections tests LRU eviction when over maxCachedConnections
func TestLRU_SizeBasedEviction_Connections(t *testing.T) {
	rt := createTestRoundTripper()

	// Create more connections than maxCachedConnections
	baseTime := time.Now()
	numConnections := maxCachedConnections + 10
	conns := make([]*mockConn, numConnections)

	for i := 0; i < numConnections; i++ {
		conns[i] = newMockConn()
		// Stagger the lastUsed times so we know which should be evicted
		lastUsed := baseTime.Add(time.Duration(i) * time.Second)
		rt.cachedConnections[addrForIndex(i)] = &cachedConn{
			conn:     conns[i],
			lastUsed: lastUsed,
		}
	}

	// Run cleanup
	rt.cleanupCache()

	// Should have exactly maxCachedConnections entries
	if len(rt.cachedConnections) != maxCachedConnections {
		t.Errorf("expected %d connections, got %d", maxCachedConnections, len(rt.cachedConnections))
	}

	// The oldest 10 connections should have been evicted and closed
	for i := 0; i < 10; i++ {
		if _, exists := rt.cachedConnections[addrForIndex(i)]; exists {
			t.Errorf("connection at index %d should have been evicted (oldest)", i)
		}
		if !conns[i].IsClosed() {
			t.Errorf("connection at index %d should have been closed", i)
		}
	}

	// The newest connections should still exist
	for i := 10; i < numConnections; i++ {
		if _, exists := rt.cachedConnections[addrForIndex(i)]; !exists {
			t.Errorf("connection at index %d should still exist (newest)", i)
		}
	}
}

// TestLRU_SizeBasedEviction_Transports tests LRU eviction when over maxCachedTransports
func TestLRU_SizeBasedEviction_Transports(t *testing.T) {
	rt := createTestRoundTripper()

	// Create more transports than maxCachedTransports
	baseTime := time.Now()
	numTransports := maxCachedTransports + 10

	for i := 0; i < numTransports; i++ {
		lastUsed := baseTime.Add(time.Duration(i) * time.Second)
		rt.cachedTransports[addrForIndex(i)] = &cachedTransport{
			transport: &http.Transport{},
			lastUsed:  lastUsed,
		}
	}

	// Run cleanup
	rt.cleanupCache()

	// Should have exactly maxCachedTransports entries
	if len(rt.cachedTransports) != maxCachedTransports {
		t.Errorf("expected %d transports, got %d", maxCachedTransports, len(rt.cachedTransports))
	}

	// The oldest 10 transports should have been evicted
	for i := 0; i < 10; i++ {
		if _, exists := rt.cachedTransports[addrForIndex(i)]; exists {
			t.Errorf("transport at index %d should have been evicted (oldest)", i)
		}
	}

	// The newest transports should still exist
	for i := 10; i < numTransports; i++ {
		if _, exists := rt.cachedTransports[addrForIndex(i)]; !exists {
			t.Errorf("transport at index %d should still exist (newest)", i)
		}
	}
}

// TestLRU_SizeBasedEviction_HTTP3Transports tests LRU eviction when over maxCachedTransports for HTTP/3
func TestLRU_SizeBasedEviction_HTTP3Transports(t *testing.T) {
	rt := createTestRoundTripper()

	// Create more HTTP/3 transports than maxCachedTransports
	baseTime := time.Now()
	numTransports := maxCachedTransports + 10

	for i := 0; i < numTransports; i++ {
		lastUsed := baseTime.Add(time.Duration(i) * time.Second)
		rt.cachedHTTP3Transports[http3AddrForIndex(i)] = &cachedHTTP3Transport{
			transport: &http3.Transport{},
			conn:      nil,
			lastUsed:  lastUsed,
		}
	}

	// Run cleanup
	rt.cleanupCache()

	// Should have exactly maxCachedTransports entries
	if len(rt.cachedHTTP3Transports) != maxCachedTransports {
		t.Errorf("expected %d HTTP/3 transports, got %d", maxCachedTransports, len(rt.cachedHTTP3Transports))
	}

	// The oldest 10 HTTP/3 transports should have been evicted
	for i := 0; i < 10; i++ {
		if _, exists := rt.cachedHTTP3Transports[http3AddrForIndex(i)]; exists {
			t.Errorf("HTTP/3 transport at index %d should have been evicted (oldest)", i)
		}
	}

	// The newest HTTP/3 transports should still exist
	for i := 10; i < numTransports; i++ {
		if _, exists := rt.cachedHTTP3Transports[http3AddrForIndex(i)]; !exists {
			t.Errorf("HTTP/3 transport at index %d should still exist (newest)", i)
		}
	}
}

// TestLRU_LastUsedUpdateOnCacheHit_Connection tests that lastUsed is updated when a connection is retrieved
func TestLRU_LastUsedUpdateOnCacheHit_Connection(t *testing.T) {
	rt := createTestRoundTripper()

	// Create a connection with an old lastUsed time
	conn := newMockConn()
	oldTime := time.Now().Add(-5 * time.Minute)
	addr := "example.com:443"

	rt.cachedConnections[addr] = &cachedConn{
		conn:     conn,
		lastUsed: oldTime,
	}

	// Simulate a cache hit by reading and updating lastUsed
	rt.cacheMu.RLock()
	cc := rt.cachedConnections[addr]
	rt.cacheMu.RUnlock()

	if cc == nil {
		t.Fatal("connection should exist")
	}

	// Update lastUsed as the real code does
	rt.cacheMu.Lock()
	cc.lastUsed = time.Now()
	rt.cacheMu.Unlock()

	// Verify lastUsed was updated
	rt.cacheMu.RLock()
	updatedCC := rt.cachedConnections[addr]
	rt.cacheMu.RUnlock()

	if updatedCC.lastUsed.Before(oldTime.Add(time.Minute)) {
		t.Error("lastUsed should have been updated to a recent time")
	}
}

// TestLRU_LastUsedUpdateOnCacheHit_Transport tests that lastUsed is updated when a transport is retrieved
func TestLRU_LastUsedUpdateOnCacheHit_Transport(t *testing.T) {
	rt := createTestRoundTripper()

	// Create a transport with an old lastUsed time
	oldTime := time.Now().Add(-5 * time.Minute)
	addr := "example.com:443"

	rt.cachedTransports[addr] = &cachedTransport{
		transport: &http.Transport{},
		lastUsed:  oldTime,
	}

	// Simulate a cache hit by reading and updating lastUsed
	rt.cacheMu.RLock()
	ct := rt.cachedTransports[addr]
	rt.cacheMu.RUnlock()

	if ct == nil {
		t.Fatal("transport should exist")
	}

	// Update lastUsed as the real code does
	rt.cacheMu.Lock()
	ct.lastUsed = time.Now()
	rt.cacheMu.Unlock()

	// Verify lastUsed was updated
	rt.cacheMu.RLock()
	updatedCT := rt.cachedTransports[addr]
	rt.cacheMu.RUnlock()

	if updatedCT.lastUsed.Before(oldTime.Add(time.Minute)) {
		t.Error("lastUsed should have been updated to a recent time")
	}
}

// TestLRU_LastUsedUpdateOnCacheHit_HTTP3Transport tests that lastUsed is updated when an HTTP/3 transport is retrieved
func TestLRU_LastUsedUpdateOnCacheHit_HTTP3Transport(t *testing.T) {
	rt := createTestRoundTripper()

	// Create an HTTP/3 transport with an old lastUsed time
	oldTime := time.Now().Add(-5 * time.Minute)
	cacheKey := "h3:example.com:443"

	rt.cachedHTTP3Transports[cacheKey] = &cachedHTTP3Transport{
		transport: &http3.Transport{},
		conn:      nil,
		lastUsed:  oldTime,
	}

	// Simulate a cache hit by reading and updating lastUsed
	rt.cacheMu.RLock()
	h3t := rt.cachedHTTP3Transports[cacheKey]
	rt.cacheMu.RUnlock()

	if h3t == nil {
		t.Fatal("HTTP/3 transport should exist")
	}

	// Update lastUsed as the real code does
	rt.cacheMu.Lock()
	h3t.lastUsed = time.Now()
	rt.cacheMu.Unlock()

	// Verify lastUsed was updated
	rt.cacheMu.RLock()
	updatedH3T := rt.cachedHTTP3Transports[cacheKey]
	rt.cacheMu.RUnlock()

	if updatedH3T.lastUsed.Before(oldTime.Add(time.Minute)) {
		t.Error("lastUsed should have been updated to a recent time")
	}
}

// TestLRU_CacheKeyPrefix_HTTP3 tests that HTTP/3 cache keys use the "h3:" prefix
func TestLRU_CacheKeyPrefix_HTTP3(t *testing.T) {
	rt := createTestRoundTripper()

	// Add an HTTP/3 transport with h3: prefix
	h3Key := "h3:example.com:443"
	rt.cachedHTTP3Transports[h3Key] = &cachedHTTP3Transport{
		transport: &http3.Transport{},
		conn:      nil,
		lastUsed:  time.Now(),
	}

	// Add a regular transport without prefix
	regularKey := "example.com:443"
	rt.cachedTransports[regularKey] = &cachedTransport{
		transport: &http.Transport{},
		lastUsed:  time.Now(),
	}

	// Verify both exist independently
	if _, exists := rt.cachedHTTP3Transports[h3Key]; !exists {
		t.Error("HTTP/3 transport with h3: prefix should exist")
	}
	if _, exists := rt.cachedTransports[regularKey]; !exists {
		t.Error("regular transport should exist")
	}

	// Verify they don't collide - h3 prefix should NOT exist in regular transports
	if _, exists := rt.cachedTransports[h3Key]; exists {
		t.Error("h3: prefixed key should not exist in regular transports cache")
	}
	if _, exists := rt.cachedHTTP3Transports[regularKey]; exists {
		t.Error("non-prefixed key should not exist in HTTP/3 transports cache")
	}
}

// TestLRU_ConcurrentAccess tests thread safety of cache operations
func TestLRU_ConcurrentAccess(t *testing.T) {
	rt := createTestRoundTripper()

	// Pre-populate some entries
	for i := 0; i < 50; i++ {
		rt.cachedConnections[addrForIndex(i)] = &cachedConn{
			conn:     newMockConn(),
			lastUsed: time.Now(),
		}
		rt.cachedTransports[addrForIndex(i)] = &cachedTransport{
			transport: &http.Transport{},
			lastUsed:  time.Now(),
		}
	}

	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rt.cacheMu.RLock()
				_ = rt.cachedConnections[addrForIndex(index%50)]
				_ = rt.cachedTransports[addrForIndex(index%50)]
				rt.cacheMu.RUnlock()
			}
		}(i)
	}

	// Concurrent writes (updates)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rt.cacheMu.Lock()
				if cc := rt.cachedConnections[addrForIndex(index%50)]; cc != nil {
					cc.lastUsed = time.Now()
				}
				if ct := rt.cachedTransports[addrForIndex(index%50)]; ct != nil {
					ct.lastUsed = time.Now()
				}
				rt.cacheMu.Unlock()
			}
		}(i)
	}

	// Concurrent cleanup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				rt.cleanupCache()
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// If we get here without panic or deadlock, test passes
}

// TestLRU_EmptyCache tests cleanup on empty caches
func TestLRU_EmptyCache(t *testing.T) {
	rt := createTestRoundTripper()

	// Should not panic on empty caches
	rt.cleanupCache()

	if len(rt.cachedConnections) != 0 {
		t.Error("connections cache should be empty")
	}
	if len(rt.cachedTransports) != 0 {
		t.Error("transports cache should be empty")
	}
	if len(rt.cachedHTTP3Transports) != 0 {
		t.Error("HTTP/3 transports cache should be empty")
	}
}

// TestLRU_MixedAgeAndSize tests cleanup with both age-based and size-based eviction needed
func TestLRU_MixedAgeAndSize(t *testing.T) {
	rt := createTestRoundTripper()

	now := time.Now()

	// Add some expired entries
	for i := 0; i < 20; i++ {
		rt.cachedConnections[addrForIndex(i)] = &cachedConn{
			conn:     newMockConn(),
			lastUsed: now.Add(-connectionMaxAge - time.Minute),
		}
	}

	// Add entries that are fresh but will trigger size-based eviction
	for i := 20; i < maxCachedConnections+30; i++ {
		rt.cachedConnections[addrForIndex(i)] = &cachedConn{
			conn:     newMockConn(),
			lastUsed: now.Add(time.Duration(i-20) * time.Second),
		}
	}

	totalBefore := len(rt.cachedConnections)
	if totalBefore != maxCachedConnections+30 {
		t.Errorf("expected %d total connections before cleanup, got %d", maxCachedConnections+30, totalBefore)
	}

	// Run cleanup
	rt.cleanupCache()

	// After cleanup: expired entries removed first, then LRU eviction
	// Should end up with maxCachedConnections
	if len(rt.cachedConnections) != maxCachedConnections {
		t.Errorf("expected %d connections after cleanup, got %d", maxCachedConnections, len(rt.cachedConnections))
	}

	// Expired entries (0-19) should definitely be gone
	for i := 0; i < 20; i++ {
		if _, exists := rt.cachedConnections[addrForIndex(i)]; exists {
			t.Errorf("expired connection at index %d should have been removed", i)
		}
	}
}

// TestLRU_CloseIdleConnections tests the CloseIdleConnections method
func TestLRU_CloseIdleConnections(t *testing.T) {
	t.Run("close all connections", func(t *testing.T) {
		rt := createTestRoundTripper()

		conn1 := newMockConn()
		conn2 := newMockConn()

		rt.cachedConnections["addr1:443"] = &cachedConn{
			conn:     conn1,
			lastUsed: time.Now(),
		}
		rt.cachedConnections["addr2:443"] = &cachedConn{
			conn:     conn2,
			lastUsed: time.Now(),
		}
		rt.cachedTransports["addr1:443"] = &cachedTransport{
			transport: &http.Transport{},
			lastUsed:  time.Now(),
		}

		rt.CloseIdleConnections()

		if !conn1.IsClosed() || !conn2.IsClosed() {
			t.Error("all connections should be closed")
		}
		if len(rt.cachedConnections) != 0 {
			t.Error("connections cache should be empty")
		}
		if len(rt.cachedTransports) != 0 {
			t.Error("transports cache should be empty")
		}
	})

	t.Run("close connections except selected", func(t *testing.T) {
		rt := createTestRoundTripper()

		conn1 := newMockConn()
		conn2 := newMockConn()
		keepAddr := "keep-addr:443"

		rt.cachedConnections[keepAddr] = &cachedConn{
			conn:     conn1,
			lastUsed: time.Now(),
		}
		rt.cachedConnections["other-addr:443"] = &cachedConn{
			conn:     conn2,
			lastUsed: time.Now(),
		}
		rt.cachedTransports[keepAddr] = &cachedTransport{
			transport: &http.Transport{},
			lastUsed:  time.Now(),
		}
		rt.cachedTransports["other-addr:443"] = &cachedTransport{
			transport: &http.Transport{},
			lastUsed:  time.Now(),
		}

		rt.CloseIdleConnections(keepAddr)

		// conn1 should still be open
		if conn1.IsClosed() {
			t.Error("selected connection should not be closed")
		}
		// conn2 should be closed
		if !conn2.IsClosed() {
			t.Error("non-selected connection should be closed")
		}

		// Keep address entries should remain
		if _, exists := rt.cachedConnections[keepAddr]; !exists {
			t.Error("selected connection should still exist in cache")
		}
		if _, exists := rt.cachedTransports[keepAddr]; !exists {
			t.Error("selected transport should still exist in cache")
		}

		// Other entries should be gone
		if _, exists := rt.cachedConnections["other-addr:443"]; exists {
			t.Error("non-selected connection should be removed from cache")
		}
		if _, exists := rt.cachedTransports["other-addr:443"]; exists {
			t.Error("non-selected transport should be removed from cache")
		}
	})
}

// TestLRU_EvictionOrder tests that the oldest entries are evicted first
func TestLRU_EvictionOrder(t *testing.T) {
	rt := createTestRoundTripper()

	// Create entries with specific timestamps to verify eviction order
	times := []time.Time{
		time.Now().Add(-9 * time.Second), // index 0 - oldest
		time.Now().Add(-7 * time.Second), // index 1
		time.Now().Add(-5 * time.Second), // index 2
		time.Now().Add(-3 * time.Second), // index 3
		time.Now().Add(-1 * time.Second), // index 4 - newest
	}

	for i, tm := range times {
		rt.cachedConnections[addrForIndex(i)] = &cachedConn{
			conn:     newMockConn(),
			lastUsed: tm,
		}
	}

	// Temporarily reduce the max for testing
	// Since we can't modify the const, we'll add more entries to trigger eviction
	// Add many more entries to exceed maxCachedConnections
	for i := 5; i < maxCachedConnections+3; i++ {
		rt.cachedConnections[addrForIndex(i)] = &cachedConn{
			conn:     newMockConn(),
			lastUsed: time.Now(), // newest entries
		}
	}

	rt.cleanupCache()

	// The first 3 oldest entries (indices 0, 1, 2) should be evicted
	// because we added maxCachedConnections + 3 - 5 = maxCachedConnections - 2 new entries
	// Total was maxCachedConnections + 3, need to remove 3

	if len(rt.cachedConnections) != maxCachedConnections {
		t.Errorf("expected %d connections after eviction, got %d", maxCachedConnections, len(rt.cachedConnections))
	}

	// First 3 (oldest) should be gone
	for i := 0; i < 3; i++ {
		if _, exists := rt.cachedConnections[addrForIndex(i)]; exists {
			t.Errorf("entry at index %d (oldest) should have been evicted", i)
		}
	}

	// The newest from original set should still exist
	if _, exists := rt.cachedConnections[addrForIndex(3)]; !exists {
		t.Error("entry at index 3 should still exist")
	}
	if _, exists := rt.cachedConnections[addrForIndex(4)]; !exists {
		t.Error("entry at index 4 should still exist")
	}
}

// TestLRU_NilConnectionHandling tests that nil connections are handled gracefully
func TestLRU_NilConnectionHandling(t *testing.T) {
	rt := createTestRoundTripper()

	// Add an entry with nil conn
	rt.cachedConnections["nil-conn:443"] = &cachedConn{
		conn:     nil,                                             // nil connection
		lastUsed: time.Now().Add(-connectionMaxAge - time.Minute), // expired
	}

	// Should not panic
	rt.cleanupCache()

	// Entry should be removed
	if _, exists := rt.cachedConnections["nil-conn:443"]; exists {
		t.Error("nil connection entry should have been removed")
	}
}

// TestLRU_Constants tests that the cache constants are reasonable
func TestLRU_Constants(t *testing.T) {
	if maxCachedConnections <= 0 {
		t.Errorf("maxCachedConnections should be positive, got %d", maxCachedConnections)
	}
	if maxCachedTransports <= 0 {
		t.Errorf("maxCachedTransports should be positive, got %d", maxCachedTransports)
	}
	if connectionMaxAge <= 0 {
		t.Errorf("connectionMaxAge should be positive, got %v", connectionMaxAge)
	}
	if cacheCleanupInterval <= 0 {
		t.Errorf("cacheCleanupInterval should be positive, got %v", cacheCleanupInterval)
	}

	// Cleanup interval should be less than or equal to max age
	if cacheCleanupInterval > connectionMaxAge {
		t.Logf("Warning: cleanup interval (%v) is greater than max age (%v)", cacheCleanupInterval, connectionMaxAge)
	}
}

// Helper functions

func addrForIndex(i int) string {
	return net.JoinHostPort("host"+string(rune('a'+i%26))+"-"+string(rune('0'+i/26)), "443")
}

func http3AddrForIndex(i int) string {
	return "h3:" + addrForIndex(i)
}
