package cycletls

import (
	"net"
	"sync"
	"testing"
	"time"

	http "github.com/Danny-Dasilva/fhttp"
)

// fakeCloseTransport records whether CloseIdleConnections was called on it.
type fakeCloseTransport struct {
	mu     sync.Mutex
	closed int
}

func (f *fakeCloseTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }

func (f *fakeCloseTransport) CloseIdleConnections() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed++
}

func (f *fakeCloseTransport) closes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// fakeCloseConn is a net.Conn that only remembers whether it was closed.
type fakeCloseConn struct {
	mu     sync.Mutex
	closed bool
}

func (c *fakeCloseConn) Read([]byte) (int, error)         { return 0, nil }
func (c *fakeCloseConn) Write([]byte) (int, error)        { return 0, nil }
func (c *fakeCloseConn) LocalAddr() net.Addr              { return nil }
func (c *fakeCloseConn) RemoteAddr() net.Addr             { return nil }
func (c *fakeCloseConn) SetDeadline(time.Time) error      { return nil }
func (c *fakeCloseConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakeCloseConn) SetWriteDeadline(time.Time) error { return nil }

func (c *fakeCloseConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakeCloseConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func newCloseTestRoundTripper() *roundTripper {
	return &roundTripper{
		cachedTransports:  make(map[string]http.RoundTripper),
		cachedConnections: make(map[string]net.Conn),
		addressMutexes:    make(map[string]*sync.Mutex),
	}
}

// closeAll backs EnableConnectionReuse=false: that client serves one request and
// is then discarded, so everything it opened has to go with it rather than being
// left connected with nothing holding a reference to close it.
func TestCloseAllReleasesEverything(t *testing.T) {
	rt := newCloseTestRoundTripper()
	ft := &fakeCloseTransport{}
	conn := &fakeCloseConn{}
	addr := "oneshot.example:443"

	rt.cachedTransports[addr] = ft
	rt.cachedConnections[addr] = conn

	rt.closeAll()

	if !conn.isClosed() {
		t.Error("cached connection left open")
	}
	if got := ft.closes(); got != 1 {
		t.Errorf("CloseIdleConnections called %d times, want 1", got)
	}
	if len(rt.cachedTransports) != 0 || len(rt.cachedConnections) != 0 {
		t.Errorf("caches not cleared: transports=%d conns=%d",
			len(rt.cachedTransports), len(rt.cachedConnections))
	}
}

// closeAll runs on a roundTripper that may have served nothing at all.
func TestCloseAllOnEmptyRoundTripper(t *testing.T) {
	rt := newCloseTestRoundTripper()
	rt.closeAll()
	if len(rt.cachedTransports) != 0 || len(rt.cachedConnections) != 0 {
		t.Error("empty roundTripper did not stay empty")
	}
}

// closeClientConnections is handed an arbitrary RoundTripper and must not panic
// on one it did not create.
func TestCloseClientConnectionsIgnoresForeignTransport(t *testing.T) {
	closeClientConnections(&fakeCloseTransport{})
	closeClientConnections(nil)
}
