package cycletls

import (
	"bytes"
	"io"
	"net"
	nhttp "net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// connTracker counts the connections a test server has accepted and how many
// of them are still open.
type connTracker struct {
	mu     sync.Mutex
	open   map[net.Conn]struct{}
	opened int
}

func (c *connTracker) track(conn net.Conn, state nhttp.ConnState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch state {
	case nhttp.StateNew:
		c.open[conn] = struct{}{}
		c.opened++
	case nhttp.StateClosed, nhttp.StateHijacked:
		delete(c.open, conn)
	}
}

func (c *connTracker) counts() (open, opened int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.open), c.opened
}

// waitForOpen polls until the server sees want open connections. The server
// learns about a client-side close asynchronously, so a single read can race.
func (c *connTracker) waitForOpen(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		open, opened := c.counts()
		if open == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("server has %d open connections of %d accepted, want %d open", open, opened, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

const closeTestBody = "close-test-body"

func newTrackedServer(t *testing.T, useTLS, h2 bool) (*httptest.Server, *connTracker) {
	t.Helper()
	tracker := &connTracker{open: make(map[net.Conn]struct{})}
	srv := httptest.NewUnstartedServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		_, _ = w.Write([]byte(closeTestBody))
	}))
	srv.Config.ConnState = tracker.track
	srv.EnableHTTP2 = h2
	if useTLS {
		srv.StartTLS()
	} else {
		srv.Start()
	}
	t.Cleanup(srv.Close)
	return srv, tracker
}

type closeCase struct {
	name       string
	useTLS, h2 bool
	forceHTTP1 bool
}

var closeCases = []closeCase{
	{name: "https_h2", useTLS: true, h2: true},
	{name: "https_h1", useTLS: true, forceHTTP1: true},
	{name: "http", useTLS: false},
}

const closeTestRequests = 4

// Do with EnableConnectionReuse off builds a client per request. Each one must
// take its connection with it, on every protocol the roundTripper speaks.
func TestDoClosesConnectionsWithoutReuse(t *testing.T) {
	for _, tc := range closeCases {
		t.Run(tc.name, func(t *testing.T) {
			srv, tracker := newTrackedServer(t, tc.useTLS, tc.h2)
			client := CycleTLS{}

			for i := 0; i < closeTestRequests; i++ {
				resp, err := client.Do(srv.URL, Options{
					InsecureSkipVerify:    true,
					ForceHTTP1:            tc.forceHTTP1,
					EnableConnectionReuse: false,
				}, "GET")
				if err != nil {
					t.Fatalf("request %d: %v", i, err)
				}
				if resp.Status != 200 || resp.Body != closeTestBody {
					t.Fatalf("request %d: status=%d body=%q", i, resp.Status, resp.Body)
				}
			}

			tracker.waitForOpen(t, 0)
			if _, opened := tracker.counts(); opened != closeTestRequests {
				t.Errorf("server accepted %d connections, want one per request (%d)", opened, closeTestRequests)
			}
		})
	}
}

// With reuse on, the pooled client keeps its connection between requests.
func TestDoKeepsConnectionWithReuse(t *testing.T) {
	srv, tracker := newTrackedServer(t, true, true)
	t.Cleanup(clearAllConnections)
	client := CycleTLS{}

	for i := 0; i < closeTestRequests; i++ {
		resp, err := client.Do(srv.URL, Options{
			InsecureSkipVerify:    true,
			EnableConnectionReuse: true,
		}, "GET")
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if resp.Status != 200 || resp.Body != closeTestBody {
			t.Fatalf("request %d: status=%d body=%q", i, resp.Status, resp.Body)
		}
	}

	open, opened := tracker.counts()
	if open != 1 || opened != 1 {
		t.Errorf("open=%d opened=%d, want one connection shared by every request", open, opened)
	}
}

// runAsync sends one request down the path the WebSocket server uses and
// returns the frames it wrote.
func runAsync(t *testing.T, id, url string, opts Options) [][]byte {
	t.Helper()
	opts.URL = url
	opts.Method = "GET"
	res := processRequest(cycleTLSRequest{RequestID: id, Options: opts})

	chanWrite := make(chan []byte, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		dispatcherAsync(res, chanWrite)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("dispatcherAsync did not return")
	}
	close(chanWrite)

	var frames [][]byte
	for f := range chanWrite {
		frames = append(frames, f)
	}
	return frames
}

// framesContain reports whether any frame carries the given bytes.
func framesContain(frames [][]byte, want string) bool {
	for _, f := range frames {
		if bytes.Contains(f, []byte(want)) {
			return true
		}
	}
	return false
}

// The npm client reaches Go through dispatcherAsync, and sends no
// enableConnectionReuse unless the caller sets it, so this is the path most
// requests with reuse off take.
func TestDispatcherAsyncClosesConnectionsWithoutReuse(t *testing.T) {
	for _, tc := range closeCases {
		t.Run(tc.name, func(t *testing.T) {
			srv, tracker := newTrackedServer(t, tc.useTLS, tc.h2)

			for i := 0; i < closeTestRequests; i++ {
				frames := runAsync(t, "close-test", srv.URL, Options{
					InsecureSkipVerify:    true,
					ForceHTTP1:            tc.forceHTTP1,
					EnableConnectionReuse: false,
				})
				if !framesContain(frames, closeTestBody) {
					t.Fatalf("request %d: body missing from frames", i)
				}
				if framesContain(frames, "error") {
					t.Fatalf("request %d: got an error frame", i)
				}
			}

			tracker.waitForOpen(t, 0)
			if _, opened := tracker.counts(); opened != closeTestRequests {
				t.Errorf("server accepted %d connections, want one per request (%d)", opened, closeTestRequests)
			}
		})
	}
}

// With reuse on, dispatcherAsync leaves the pooled connection open.
func TestDispatcherAsyncKeepsConnectionWithReuse(t *testing.T) {
	srv, tracker := newTrackedServer(t, true, true)
	t.Cleanup(clearAllConnections)

	for i := 0; i < closeTestRequests; i++ {
		frames := runAsync(t, "reuse-test", srv.URL, Options{
			InsecureSkipVerify:    true,
			EnableConnectionReuse: true,
		})
		if !framesContain(frames, closeTestBody) {
			t.Fatalf("request %d: body missing from frames", i)
		}
	}

	open, opened := tracker.counts()
	if open != 1 || opened != 1 {
		t.Errorf("open=%d opened=%d, want one connection shared by every request", open, opened)
	}
}

// newH2ConnectProxy starts an HTTPS proxy that answers CONNECT over HTTP/2,
// the case where connectDialer keeps its connection to the proxy.
func newH2ConnectProxy(t *testing.T) (*httptest.Server, *connTracker) {
	t.Helper()
	tracker := &connTracker{open: make(map[net.Conn]struct{})}
	srv := httptest.NewUnstartedServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		if r.Method != nhttp.MethodConnect {
			w.WriteHeader(nhttp.StatusMethodNotAllowed)
			return
		}
		target, err := net.Dial("tcp", r.Host)
		if err != nil {
			w.WriteHeader(nhttp.StatusBadGateway)
			return
		}
		defer target.Close()

		w.WriteHeader(nhttp.StatusOK)
		flusher := w.(nhttp.Flusher)
		flusher.Flush()

		go func() {
			_, _ = io.Copy(target, r.Body)
			_ = target.Close()
		}()
		buf := make([]byte, 16*1024)
		for {
			n, err := target.Read(buf)
			if n > 0 {
				if _, werr := w.Write(buf[:n]); werr != nil {
					return
				}
				flusher.Flush()
			}
			if err != nil {
				return
			}
		}
	}))
	srv.Config.ConnState = tracker.track
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv, tracker
}

// Through an HTTP/2 proxy the dialer holds a connection of its own. With reuse
// off that has to close too, or each request strands one at the proxy.
func TestDoClosesH2ProxyConnectionWithoutReuse(t *testing.T) {
	srv, tracker := newTrackedServer(t, true, true)
	proxy, proxyTracker := newH2ConnectProxy(t)
	client := CycleTLS{}

	for i := 0; i < closeTestRequests; i++ {
		resp, err := client.Do(srv.URL, Options{
			InsecureSkipVerify:    true,
			EnableConnectionReuse: false,
			Proxy:                 proxy.URL,
		}, "GET")
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if resp.Status != 200 || resp.Body != closeTestBody {
			t.Fatalf("request %d: status=%d body=%q", i, resp.Status, resp.Body)
		}
	}

	tracker.waitForOpen(t, 0)
	proxyTracker.waitForOpen(t, 0)
	if _, opened := proxyTracker.counts(); opened != closeTestRequests {
		t.Errorf("proxy accepted %d connections, want one per request (%d)", opened, closeTestRequests)
	}
}

// The close must wait for the body. A response sent in many flushed chunks
// arrives whole with reuse off, and the connection goes once it has.
func TestDispatcherAsyncStreamsWholeBodyWithoutReuse(t *testing.T) {
	const chunks, chunkSize = 200, 8 * 1024
	chunk := bytes.Repeat([]byte("x"), chunkSize)

	for _, h2 := range []bool{true, false} {
		name := "https_h1"
		if h2 {
			name = "https_h2"
		}
		t.Run(name, func(t *testing.T) {
			tracker := &connTracker{open: make(map[net.Conn]struct{})}
			srv := httptest.NewUnstartedServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
				for i := 0; i < chunks; i++ {
					_, _ = w.Write(chunk)
					w.(nhttp.Flusher).Flush()
				}
			}))
			srv.Config.ConnState = tracker.track
			srv.EnableHTTP2 = h2
			srv.StartTLS()
			t.Cleanup(srv.Close)

			const id = "stream-test"
			opts := Options{
				URL:                   srv.URL,
				Method:                "GET",
				InsecureSkipVerify:    true,
				ForceHTTP1:            !h2,
				EnableConnectionReuse: false,
			}
			res := processRequest(cycleTLSRequest{RequestID: id, Options: opts})

			chanWrite := make(chan []byte, 16)
			go func() {
				dispatcherAsync(res, chanWrite)
				close(chanWrite)
			}()

			// Frame layout: 2-byte id length, id, 2-byte method length, method,
			// then for "data" a 4-byte payload length and the payload.
			dataPrefix := append([]byte{0, byte(len(id))}, id...)
			dataPrefix = append(dataPrefix, 0, 4, 'd', 'a', 't', 'a')
			errPrefix := append([]byte{0, byte(len(id))}, id...)
			errPrefix = append(errPrefix, 0, 5, 'e', 'r', 'r', 'o', 'r')

			got := 0
			for f := range chanWrite {
				if bytes.HasPrefix(f, errPrefix) {
					t.Fatalf("error frame: %q", f)
				}
				if bytes.HasPrefix(f, dataPrefix) {
					got += len(f) - len(dataPrefix) - 4
				}
			}
			if want := chunks * chunkSize; got != want {
				t.Fatalf("body is %d bytes, want %d", got, want)
			}
			tracker.waitForOpen(t, 0)
		})
	}
}
