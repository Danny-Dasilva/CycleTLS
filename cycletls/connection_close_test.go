package cycletls

import (
	"bytes"
	"context"
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

// dispatchPath is one of the two routes a WebSocket server request takes: the
// legacy dispatcher, or the V2 dispatcher with flow control.
type dispatchPath struct {
	name string
	// start builds the request and runs the dispatcher in a goroutine. It
	// closes chanWrite once the dispatcher has returned.
	start func(req cycleTLSRequest, chanWrite chan []byte)
}

var dispatchPaths = []dispatchPath{
	{
		name: "legacy",
		start: func(req cycleTLSRequest, chanWrite chan []byte) {
			res := processRequest(req)
			go func() {
				defer close(chanWrite)
				dispatcherAsync(res, newSafeChannelWriter(chanWrite))
			}()
		},
	},
	{
		name: "v2",
		start: func(req cycleTLSRequest, chanWrite chan []byte) {
			ctx := context.Background()
			res := processRequestV2(req, ctx)
			go func() {
				defer close(chanWrite)
				dispatcherAsyncV2(res, newFrameSender(ctx, chanWrite))
			}()
		},
	},
}

// noHeaderTimeout is the Timeout the dispatcher tests send. A negative value
// turns the header timeout off. With any other value doRequestWithHeaderTimeout
// cancels the request context as soon as the headers arrive, and both
// dispatchers then stop before reading the body. That is a separate bug; these
// tests are about what happens to the connection once the body has streamed.
const noHeaderTimeout = -1

// runAsync sends one request down the given dispatcher path and returns the
// frames it wrote.
func runAsync(t *testing.T, path dispatchPath, id, url string, opts Options) [][]byte {
	t.Helper()
	opts.URL = url
	opts.Method = "GET"
	opts.Timeout = noHeaderTimeout

	chanWrite := make(chan []byte, 64)
	path.start(cycleTLSRequest{RequestID: id, Options: opts}, chanWrite)

	var frames [][]byte
	timeout := time.After(10 * time.Second)
	for {
		select {
		case f, ok := <-chanWrite:
			if !ok {
				return frames
			}
			frames = append(frames, f)
		case <-timeout:
			t.Fatalf("%s dispatcher did not return", path.name)
		}
	}
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

// The npm client reaches Go through the dispatchers, not Do(), and sends no
// enableConnectionReuse unless the caller sets it, so these are the paths most
// requests with reuse off take.
func TestDispatcherAsyncClosesConnectionsWithoutReuse(t *testing.T) {
	for _, path := range dispatchPaths {
		for _, tc := range closeCases {
			t.Run(path.name+"/"+tc.name, func(t *testing.T) {
				srv, tracker := newTrackedServer(t, tc.useTLS, tc.h2)

				for i := 0; i < closeTestRequests; i++ {
					frames := runAsync(t, path, "close-test", srv.URL, Options{
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
}

// With reuse on, the dispatchers leave the pooled connection open.
func TestDispatcherAsyncKeepsConnectionWithReuse(t *testing.T) {
	for _, path := range dispatchPaths {
		t.Run(path.name, func(t *testing.T) {
			clearAllConnections()
			srv, tracker := newTrackedServer(t, true, true)
			t.Cleanup(clearAllConnections)

			for i := 0; i < closeTestRequests; i++ {
				frames := runAsync(t, path, "reuse-test", srv.URL, Options{
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
		})
	}
}

// An SSE stream with reuse off has a client of its own too. Once the stream
// ends, its connection has to close on both dispatcher paths.
func TestDispatcherSSEClosesConnectionsWithoutReuse(t *testing.T) {
	for _, path := range dispatchPaths {
		t.Run(path.name, func(t *testing.T) {
			tracker := &connTracker{open: make(map[net.Conn]struct{})}
			srv := httptest.NewUnstartedServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("data: " + closeTestBody + "\n\n"))
			}))
			srv.Config.ConnState = tracker.track
			srv.EnableHTTP2 = true
			srv.StartTLS()
			t.Cleanup(srv.Close)

			for i := 0; i < closeTestRequests; i++ {
				frames := runAsync(t, path, "sse-close-test", srv.URL, Options{
					Protocol:              "sse",
					InsecureSkipVerify:    true,
					EnableConnectionReuse: false,
				})
				if !framesContain(frames, closeTestBody) {
					t.Fatalf("request %d: event missing from frames", i)
				}
			}

			tracker.waitForOpen(t, 0)
			if _, opened := tracker.counts(); opened != closeTestRequests {
				t.Errorf("server accepted %d connections, want one per stream (%d)", opened, closeTestRequests)
			}
		})
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

	type streamCase struct {
		path dispatchPath
		h2   bool
	}
	var cases []streamCase
	for _, path := range dispatchPaths {
		cases = append(cases, streamCase{path, true}, streamCase{path, false})
	}

	for _, sc := range cases {
		path, h2 := sc.path, sc.h2
		name := path.name + "/https_h1"
		if h2 {
			name = path.name + "/https_h2"
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
				Timeout:               noHeaderTimeout,
			}
			chanWrite := make(chan []byte, 16)
			path.start(cycleTLSRequest{RequestID: id, Options: opts}, chanWrite)

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
