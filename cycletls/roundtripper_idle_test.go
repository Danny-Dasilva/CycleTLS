package cycletls

import (
	"net"
	"sync"
	"testing"
	"time"

	http "github.com/Danny-Dasilva/fhttp"
)

// fakeIdleTransport records whether CloseIdleConnections was called on it.
type fakeIdleTransport struct {
	mu     sync.Mutex
	closed int
}

func (f *fakeIdleTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }

func (f *fakeIdleTransport) CloseIdleConnections() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed++
}

func (f *fakeIdleTransport) closes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// fakeConn is a net.Conn that only remembers whether it was closed.
type fakeConn struct {
	mu     sync.Mutex
	closed bool
}

func (c *fakeConn) Read([]byte) (int, error)         { return 0, nil }
func (c *fakeConn) Write([]byte) (int, error)        { return 0, nil }
func (c *fakeConn) LocalAddr() net.Addr              { return nil }
func (c *fakeConn) RemoteAddr() net.Addr             { return nil }
func (c *fakeConn) SetDeadline(time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(time.Time) error { return nil }

func (c *fakeConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakeConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func newTestRoundTripper() *roundTripper {
	return &roundTripper{
		cachedTransports:  make(map[string]http.RoundTripper),
		cachedConnections: make(map[string]net.Conn),
		lastUsed:          make(map[string]time.Time),
		addressMutexes:    make(map[string]*sync.Mutex),
	}
}

// A transport still in use must not be swept, or a sweep would be closing
// connections out from under live traffic.
func TestSweepIdleKeepsRecentlyUsedTransports(t *testing.T) {
	rt := newTestRoundTripper()
	ft := &fakeIdleTransport{}
	rt.cachedTransports["live.example:443"] = ft
	rt.lastUsed["live.example:443"] = time.Now()

	rt.sweepIdle()

	if got := ft.closes(); got != 0 {
		t.Fatalf("CloseIdleConnections called %d times on a transport in active use, want 0", got)
	}
	if _, ok := rt.cachedTransports["live.example:443"]; !ok {
		t.Fatal("a transport in active use was evicted from the cache")
	}
}

// Past idleConnTTL the transport's idle connections are closed, but the cache
// entry stays so a request still running keeps a transport someone can close.
func TestSweepIdleClosesIdleConnectionsButKeepsEntry(t *testing.T) {
	rt := newTestRoundTripper()
	ft := &fakeIdleTransport{}
	rt.cachedTransports["idle.example:443"] = ft
	rt.lastUsed["idle.example:443"] = time.Now().Add(-idleConnTTL - time.Second)

	rt.sweepIdle()

	if got := ft.closes(); got != 1 {
		t.Fatalf("CloseIdleConnections called %d times past idleConnTTL, want 1", got)
	}
	if _, ok := rt.cachedTransports["idle.example:443"]; !ok {
		t.Fatal("entry evicted at idleConnTTL; eviction is meant to wait for 2*idleConnTTL")
	}
}

// Past 2*idleConnTTL nothing can still be using it, so the entry and everything
// keyed alongside it goes -- otherwise the maps grow once per host contacted.
func TestSweepIdleEvictsLongIdleTransports(t *testing.T) {
	rt := newTestRoundTripper()
	ft := &fakeIdleTransport{}
	conn := &fakeConn{}
	addr := "stale.example:443"

	rt.cachedTransports[addr] = ft
	rt.cachedConnections[addr] = conn
	rt.lastUsed[addr] = time.Now().Add(-2*idleConnTTL - time.Second)
	rt.getAddressMutex(addr)

	rt.sweepIdle()

	if _, ok := rt.cachedTransports[addr]; ok {
		t.Error("transport still cached after 2*idleConnTTL")
	}
	if _, ok := rt.cachedConnections[addr]; ok {
		t.Error("connection still cached after 2*idleConnTTL")
	}
	if _, ok := rt.lastUsed[addr]; ok {
		t.Error("lastUsed entry survived eviction")
	}
	if !conn.isClosed() {
		t.Error("cached connection was dropped without being closed, leaking the socket")
	}

	rt.addressMutexLock.Lock()
	_, muExists := rt.addressMutexes[addr]
	rt.addressMutexLock.Unlock()
	if muExists {
		t.Error("address mutex outlived the address it guarded")
	}
}

// Sweeping is driven from RoundTrip, so it has to be throttled or every request
// would walk the whole cache.
func TestSweepIdleIsThrottled(t *testing.T) {
	rt := newTestRoundTripper()
	ft := &fakeIdleTransport{}
	rt.cachedTransports["idle.example:443"] = ft
	rt.lastUsed["idle.example:443"] = time.Now().Add(-idleConnTTL - time.Second)

	rt.sweepIdle()
	rt.sweepIdle()
	rt.sweepIdle()

	if got := ft.closes(); got != 1 {
		t.Fatalf("swept %d times in quick succession, want 1 (throttled by idleSweepInterval)", got)
	}
}

// The cached connection is a one-shot handoff to the transport's first DialTLS
// call. Leaving it in place handed every later dial the same conn and pinned
// every connection the client had ever opened.
func TestDialTLSConsumesCachedConnection(t *testing.T) {
	rt := newTestRoundTripper()
	conn := &fakeConn{}
	addr := "handoff.example:443"
	rt.cachedConnections[addr] = conn

	got, err := rt.dialTLS(nil, "tcp", addr)
	if err != nil {
		t.Fatalf("dialTLS returned error on cache hit: %v", err)
	}
	if got != conn {
		t.Fatal("dialTLS did not return the cached connection")
	}
	if _, ok := rt.cachedConnections[addr]; ok {
		t.Fatal("cached connection was not consumed; later dials would be handed the same conn")
	}
	if conn.isClosed() {
		t.Fatal("handoff closed the connection it was handing over")
	}
}
