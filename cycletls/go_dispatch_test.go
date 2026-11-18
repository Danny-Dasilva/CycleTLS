package cycletls

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	nhttp "net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Tests for the 3.0.1 request dispatch fixes. They drive the dispatchers
// directly against local servers; nothing here needs the network.

// startDispatch sends one request down a dispatcher path. The returned channel
// carries the frames it writes and is closed when the dispatcher returns.
func startDispatch(path dispatchPath, req cycleTLSRequest) <-chan []byte {
	chanWrite := make(chan []byte, 64)
	path.start(req, chanWrite)
	return chanWrite
}

// collectFrames reads frames until the dispatcher returns.
func collectFrames(t *testing.T, name string, ch <-chan []byte) [][]byte {
	t.Helper()
	var frames [][]byte
	timeout := time.After(15 * time.Second)
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				return frames
			}
			frames = append(frames, f)
		case <-timeout:
			t.Fatalf("%s dispatcher did not return; frames so far: %v", name, frameMethods(frames))
		}
	}
}

// frameHead splits a frame into its request ID, method name and payload.
func frameHead(frame []byte) (id, method string, rest []byte) {
	readString := func(b []byte) (string, []byte) {
		if len(b) < 2 {
			return "", nil
		}
		n := int(binary.BigEndian.Uint16(b))
		if len(b) < 2+n {
			return "", nil
		}
		return string(b[2 : 2+n]), b[2+n:]
	}
	id, rest = readString(frame)
	method, rest = readString(rest)
	return id, method, rest
}

func frameMethods(frames [][]byte) []string {
	methods := make([]string, 0, len(frames))
	for _, f := range frames {
		_, method, _ := frameHead(f)
		methods = append(methods, method)
	}
	return methods
}

// framesWithMethod returns the payloads of every frame of one method.
func framesWithMethod(frames [][]byte, want string) [][]byte {
	var out [][]byte
	for _, f := range frames {
		if _, method, rest := frameHead(f); method == want {
			out = append(out, rest)
		}
	}
	return out
}

// errorMessages returns the text of every "error" frame.
func errorMessages(frames [][]byte) []string {
	var out []string
	for _, payload := range framesWithMethod(frames, "error") {
		if len(payload) < 4 {
			continue
		}
		n := int(binary.BigEndian.Uint16(payload[2:]))
		if len(payload) >= 4+n {
			out = append(out, string(payload[4:4+n]))
		}
	}
	return out
}

// wsCloseInfo decodes the code and reason of the first ws_close frame.
func wsCloseInfo(t *testing.T, frames [][]byte) (code int, reason string) {
	t.Helper()
	closes := framesWithMethod(frames, "ws_close")
	if len(closes) != 1 {
		t.Fatalf("got %d ws_close frames, want 1; frames: %v", len(closes), frameMethods(frames))
	}
	var msg struct {
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	}
	if len(closes[0]) < 4 {
		t.Fatalf("short ws_close frame")
	}
	if err := json.Unmarshal(closes[0][4:], &msg); err != nil {
		t.Fatalf("ws_close payload: %v", err)
	}
	return msg.Code, msg.Reason
}

// -----------------------------------------------------------------------------
// Item 1: bad input must fail one request, not the process
// -----------------------------------------------------------------------------

// Before 3.0.1 the dispatch functions called log.Fatal on a client or request
// build error, so one request with proxy "ftp://..." ended the Go process and
// every other request with it. Run on the old code, this test kills the test
// binary.
func TestBadInputReturnsErrorFrame(t *testing.T) {
	good := httptest.NewServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		_, _ = w.Write([]byte("still-alive"))
	}))
	t.Cleanup(good.Close)

	type badCase struct {
		name string
		opts Options
		want string // text the error frame must carry
	}
	cases := []badCase{
		{"proxy_scheme", Options{URL: good.URL, Method: "GET", Proxy: "ftp://127.0.0.1:1"}, "ftp"},
		{"proxy_no_host", Options{URL: good.URL, Method: "GET", Proxy: "http://"}, "invalid url"},
		{"url", Options{URL: "http://%zz/", Method: "GET"}, "invalid"},
	}
	methodCase := badCase{"method", Options{URL: good.URL, Method: "BAD METHOD"}, "invalid"}

	for _, path := range dispatchPaths {
		for _, protocol := range []string{"", "sse", "websocket", "http3"} {
			protocolCases := cases
			if protocol != "websocket" { // a WebSocket handshake is always a GET
				protocolCases = append(append([]badCase(nil), cases...), methodCase)
			}
			for _, tc := range protocolCases {
				name := path.name + "/" + protocol + "/" + tc.name
				t.Run(name, func(t *testing.T) {
					opts := tc.opts
					opts.Protocol = protocol
					frames := collectFrames(t, name, startDispatch(path, cycleTLSRequest{RequestID: "bad-input", Options: opts}))

					messages := errorMessages(frames)
					if len(messages) != 1 {
						t.Fatalf("got %d error frames, want 1; frames: %v", len(messages), frameMethods(frames))
					}
					if !strings.Contains(strings.ToLower(messages[0]), tc.want) {
						t.Errorf("error %q does not mention %q", messages[0], tc.want)
					}
				})
			}
		}

		// The process is still here and still serves requests.
		frames := runAsync(t, path, "after-bad-input", good.URL, Options{})
		if !framesContain(frames, "still-alive") {
			t.Fatalf("%s: request after the bad ones failed; frames: %v", path.name, frameMethods(frames))
		}
	}
}

// -----------------------------------------------------------------------------
// Item 2: one bad message must not poison the legacy socket
// -----------------------------------------------------------------------------

// The legacy client sends every request down one socket. Before 3.0.1 a
// message whose options did not decode (a numeric header value) made
// readSocket return, the socket stayed open with nobody reading it, and every
// later request timed out.
func TestLegacySocketSurvivesBadRequest(t *testing.T) {
	target := httptest.NewServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		_, _ = w.Write([]byte("good-body"))
	}))
	t.Cleanup(target.Close)

	server := httptest.NewServer(nhttp.HandlerFunc(WSEndpoint))
	t.Cleanup(server.Close)

	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/?v=1", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	send := func(msg string) {
		t.Helper()
		if err := ws.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	send(`this is not json`)
	send(`{"requestId":"bad-1","options":{"url":"` + target.URL + `","method":"GET","headers":{"x-int":1}}}`)
	send(`{"requestId":"good-1","options":{"url":"` + target.URL + `","method":"GET"}}`)

	var badError string
	var goodFrames [][]byte
	_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
	for badError == "" || len(framesWithMethod(goodFrames, "end")) == 0 {
		_, frame, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v (bad error=%q, good frames=%v)", err, badError, frameMethods(goodFrames))
		}
		id, method, _ := frameHead(frame)
		switch {
		case id == "bad-1" && method == "error":
			badError = errorMessages([][]byte{frame})[0]
		case id == "good-1":
			goodFrames = append(goodFrames, frame)
		default:
			t.Fatalf("unexpected frame id=%q method=%q", id, method)
		}
	}

	if !strings.Contains(badError, "invalid request options") {
		t.Errorf("bad request error = %q, want it to say the options were invalid", badError)
	}
	if !framesContain(goodFrames, "good-body") {
		t.Errorf("good request got no body; frames: %v", frameMethods(goodFrames))
	}
	if len(errorMessages(goodFrames)) != 0 {
		t.Errorf("good request got an error: %v", errorMessages(goodFrames))
	}
}

// -----------------------------------------------------------------------------
// Item 4: SSE carries the caller's headers, method and body
// -----------------------------------------------------------------------------

type seenRequest struct {
	method string
	header nhttp.Header
	body   string
}

func newSSERecorder(t *testing.T) (*httptest.Server, func() seenRequest) {
	t.Helper()
	var mu sync.Mutex
	var seen seenRequest
	srv := httptest.NewServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = seenRequest{method: r.Method, header: r.Header.Clone(), body: string(body)}
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: sse-event\n\n"))
	}))
	t.Cleanup(srv.Close)
	return srv, func() seenRequest {
		mu.Lock()
		defer mu.Unlock()
		return seen
	}
}

// Before 3.0.1 the V2 path sent a bare GET with none of the caller's headers,
// and neither path could send a method or body.
func TestSSERequestCarriesHeadersMethodAndBody(t *testing.T) {
	for _, path := range dispatchPaths {
		t.Run(path.name, func(t *testing.T) {
			srv, seen := newSSERecorder(t)
			frames := collectFrames(t, path.name, startDispatch(path, cycleTLSRequest{
				RequestID: "sse-headers",
				Options: Options{
					URL:       srv.URL,
					Protocol:  "sse",
					Method:    "POST",
					Body:      `{"prompt":"hi"}`,
					UserAgent: "sse-test-agent",
					Headers: map[string]string{
						"Authorization": "Bearer token-1",
						"X-Custom":      "custom-value",
						"Content-Type":  "application/json",
					},
				},
			}))
			if !framesContain(frames, "sse-event") {
				t.Fatalf("event missing; frames: %v errors: %v", frameMethods(frames), errorMessages(frames))
			}

			got := seen()
			if got.method != "POST" {
				t.Errorf("method = %q, want POST", got.method)
			}
			if got.body != `{"prompt":"hi"}` {
				t.Errorf("body = %q, want the caller's body", got.body)
			}
			for name, want := range map[string]string{
				"Authorization": "Bearer token-1",
				"X-Custom":      "custom-value",
				"Content-Type":  "application/json",
				"User-Agent":    "sse-test-agent",
				"Accept":        "text/event-stream",
			} {
				if values := got.header.Values(name); len(values) != 1 || values[0] != want {
					t.Errorf("header %s = %q, want [%q]", name, values, want)
				}
			}
		})
	}
}

// A caller's own Accept wins over the default, whatever its letter case, and
// is not sent twice.
func TestSSECallerAcceptWins(t *testing.T) {
	for _, path := range dispatchPaths {
		for _, key := range []string{"accept", "Accept", "ACCEPT"} {
			t.Run(path.name+"/"+key, func(t *testing.T) {
				srv, seen := newSSERecorder(t)
				frames := collectFrames(t, path.name, startDispatch(path, cycleTLSRequest{
					RequestID: "sse-accept",
					Options: Options{
						URL:      srv.URL,
						Protocol: "sse",
						Headers:  map[string]string{key: "application/x-ndjson"},
					},
				}))
				if !framesContain(frames, "sse-event") {
					t.Fatalf("event missing; frames: %v errors: %v", frameMethods(frames), errorMessages(frames))
				}
				got := seen()
				if got.method != "GET" {
					t.Errorf("method = %q, want GET when none is given", got.method)
				}
				if values := got.header.Values("Accept"); len(values) != 1 || values[0] != "application/x-ndjson" {
					t.Errorf("Accept = %q, want only the caller's value", values)
				}
			})
		}
	}
}

// -----------------------------------------------------------------------------
// Item 5: enableConnectionReuse defaults to true for JSON requests
// -----------------------------------------------------------------------------

func TestEnableConnectionReuseJSONDefault(t *testing.T) {
	cases := []struct {
		json string
		want bool
	}{
		{`{}`, true},
		{`{"url":"https://example.com"}`, true},
		{`{"enableConnectionReuse":null}`, true},
		{`{"enableConnectionReuse":true}`, true},
		{`{"enableConnectionReuse":false}`, false},
	}
	for _, tc := range cases {
		var opts Options
		if err := json.Unmarshal([]byte(tc.json), &opts); err != nil {
			t.Fatalf("%s: %v", tc.json, err)
		}
		if opts.EnableConnectionReuse != tc.want {
			t.Errorf("%s: EnableConnectionReuse = %v, want %v", tc.json, opts.EnableConnectionReuse, tc.want)
		}

		// The same options inside a request, as the clients send them.
		var req cycleTLSRequest
		if err := json.Unmarshal([]byte(`{"requestId":"r","options":`+tc.json+`}`), &req); err != nil {
			t.Fatalf("request %s: %v", tc.json, err)
		}
		if req.Options.EnableConnectionReuse != tc.want {
			t.Errorf("request %s: EnableConnectionReuse = %v, want %v", tc.json, req.Options.EnableConnectionReuse, tc.want)
		}
	}

	// A Go struct literal keeps the bool zero value: no API change for Do callers.
	if (Options{}).EnableConnectionReuse {
		t.Error("Options{} zero value changed")
	}
}

// Requests as the Node clients send them, with the field left out, share one
// connection. Before 3.0.1 each one opened and closed its own.
func TestJSONRequestsReuseConnectionByDefault(t *testing.T) {
	for _, path := range dispatchPaths {
		t.Run(path.name, func(t *testing.T) {
			clearAllConnections()
			srv, tracker := newTrackedServer(t, true, true)
			t.Cleanup(clearAllConnections)

			for i := 0; i < closeTestRequests; i++ {
				var req cycleTLSRequest
				msg := `{"requestId":"json-reuse","options":{"url":"` + srv.URL + `","method":"GET","insecureSkipVerify":true}}`
				if err := json.Unmarshal([]byte(msg), &req); err != nil {
					t.Fatal(err)
				}
				frames := collectFrames(t, path.name, startDispatch(path, req))
				if !framesContain(frames, closeTestBody) {
					t.Fatalf("request %d: body missing; errors: %v", i, errorMessages(frames))
				}
			}

			if open, opened := tracker.counts(); open != 1 || opened != 1 {
				t.Errorf("open=%d opened=%d, want one connection shared by every request", open, opened)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Item 6: WebSocket requests go through the proxy
// -----------------------------------------------------------------------------

// connectProxy is a plain HTTP proxy that only answers CONNECT.
type connectProxy struct {
	mu      sync.Mutex
	targets []string
}

func (p *connectProxy) seen() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.targets...)
}

func newConnectProxy(t *testing.T) (*httptest.Server, *connectProxy) {
	t.Helper()
	proxy := &connectProxy{}
	srv := httptest.NewServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		if r.Method != nhttp.MethodConnect {
			w.WriteHeader(nhttp.StatusMethodNotAllowed)
			return
		}
		proxy.mu.Lock()
		proxy.targets = append(proxy.targets, r.Host)
		proxy.mu.Unlock()

		target, err := net.Dial("tcp", r.Host)
		if err != nil {
			w.WriteHeader(nhttp.StatusBadGateway)
			return
		}
		defer target.Close()

		client, rw, err := w.(nhttp.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer client.Close()
		_, _ = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = rw.Flush()

		done := make(chan struct{}, 2)
		go func() { _, _ = io.Copy(target, rw); done <- struct{}{} }()
		go func() { _, _ = io.Copy(client, target); done <- struct{}{} }()
		<-done
	}))
	t.Cleanup(srv.Close)
	return srv, proxy
}

// newWSUpstream starts a WebSocket server. handle runs once per connection;
// the connection is closed when it returns.
func newWSUpstream(t *testing.T, useTLS bool, handle func(conn *websocket.Conn, r *nhttp.Request)) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*nhttp.Request) bool { return true }}
	srv := httptest.NewUnstartedServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handle(conn, r)
	}))
	if useTLS {
		srv.StartTLS()
	} else {
		srv.Start()
	}
	t.Cleanup(srv.Close)
	return srv
}

// closeNormally sends a close frame and waits for the peer to hang up.
func closeNormally(conn *websocket.Conn) {
	_ = conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"), time.Now().Add(time.Second))
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// Before 3.0.1 a WebSocket request ignored its proxy option and dialled the
// server from the caller's own address. It also dropped userAgent and cookies.
func TestWebSocketUsesProxy(t *testing.T) {
	type proxyCase struct {
		name        string
		upstreamTLS bool
		h2Proxy     bool
	}
	proxyCases := []proxyCase{
		{name: "ws_http_proxy"},
		{name: "wss_http_proxy", upstreamTLS: true},
		{name: "wss_https_h2_proxy", upstreamTLS: true, h2Proxy: true},
	}

	for _, path := range dispatchPaths {
		for _, tc := range proxyCases {
			t.Run(path.name+"/"+tc.name, func(t *testing.T) {
				var mu sync.Mutex
				var handshake nhttp.Header
				upstream := newWSUpstream(t, tc.upstreamTLS, func(conn *websocket.Conn, r *nhttp.Request) {
					mu.Lock()
					handshake = r.Header.Clone()
					mu.Unlock()
					_ = conn.WriteMessage(websocket.TextMessage, []byte("hello-through-proxy"))
					closeNormally(conn)
				})

				var proxyURL string
				var connects func() int
				if tc.h2Proxy {
					srv, tracker := newH2ConnectProxy(t)
					proxyURL = srv.URL
					connects = func() int { _, opened := tracker.counts(); return opened }
				} else {
					srv, proxy := newConnectProxy(t)
					proxyURL = srv.URL
					connects = func() int {
						targets := proxy.seen()
						for _, target := range targets {
							if target != strings.TrimPrefix(strings.TrimPrefix(upstream.URL, "https://"), "http://") {
								t.Errorf("proxy was asked for %q, want the upstream", target)
							}
						}
						return len(targets)
					}
				}

				frames := collectFrames(t, path.name, startDispatch(path, cycleTLSRequest{
					RequestID: "ws-proxy",
					Options: Options{
						URL:                upstream.URL,
						Protocol:           "websocket",
						Proxy:              proxyURL,
						InsecureSkipVerify: true,
						UserAgent:          "ws-test-agent",
						Cookies:            []Cookie{{Name: "session", Value: "abc"}},
						Headers:            map[string]string{"cookie": "theme=dark", "X-Custom": "kept"},
					},
				}))

				if len(framesWithMethod(frames, "ws_open")) != 1 || !framesContain(frames, "hello-through-proxy") {
					t.Fatalf("no open or message through the proxy; frames: %v errors: %v %s",
						frameMethods(frames), errorMessages(frames), framesWithMethod(frames, "ws_error"))
				}
				if got := connects(); got != 1 {
					t.Errorf("proxy saw %d connections, want 1: the WebSocket did not use it", got)
				}

				mu.Lock()
				defer mu.Unlock()
				if got := handshake.Get("User-Agent"); got != "ws-test-agent" {
					t.Errorf("User-Agent = %q, want the userAgent option", got)
				}
				if got := handshake.Get("Cookie"); got != "theme=dark; session=abc" {
					t.Errorf("Cookie = %q, want the header and the cookies option joined", got)
				}
				if got := handshake.Get("X-Custom"); got != "kept" {
					t.Errorf("X-Custom = %q, want kept", got)
				}
			})
		}
	}
}

// A proxy that refuses the tunnel fails the request. The WebSocket must not
// fall back to a direct connection.
func TestWebSocketProxyFailureDoesNotDialDirect(t *testing.T) {
	refusing := httptest.NewServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		w.WriteHeader(nhttp.StatusForbidden)
	}))
	t.Cleanup(refusing.Close)

	for _, path := range dispatchPaths {
		t.Run(path.name, func(t *testing.T) {
			var mu sync.Mutex
			reached := false
			upstream := newWSUpstream(t, false, func(conn *websocket.Conn, r *nhttp.Request) {
				mu.Lock()
				reached = true
				mu.Unlock()
				closeNormally(conn)
			})

			frames := collectFrames(t, path.name, startDispatch(path, cycleTLSRequest{
				RequestID: "ws-proxy-refused",
				Options:   Options{URL: upstream.URL, Protocol: "websocket", Proxy: refusing.URL},
			}))
			if len(framesWithMethod(frames, "ws_error")) == 0 {
				t.Errorf("want a ws_error frame; frames: %v", frameMethods(frames))
			}
			if len(framesWithMethod(frames, "ws_open")) != 0 {
				t.Errorf("connection opened although the proxy refused; frames: %v", frameMethods(frames))
			}
			mu.Lock()
			defer mu.Unlock()
			if reached {
				t.Error("upstream was reached directly, around the proxy")
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Item 7: Do
// -----------------------------------------------------------------------------

// Before 3.0.1 Do read only the first Content-Encoding value and knew no zstd.
func TestDoDecodesEveryContentEncoding(t *testing.T) {
	payload := []byte(`{"ok":true,"filler":"` + strings.Repeat("cycletls ", 2000) + `"}`)
	gzipBody := encodeWith(t, "gzip", payload)

	cases := []struct {
		name     string
		encoding string
		body     []byte
	}{
		{"gzip", "gzip", gzipBody},
		{"gzip_then_br", "gzip, br", encodeWith(t, "br", gzipBody)},
		{"zstd", "zstd", encodeWith(t, "zstd", payload)},
		{"mixed_case", "GZip", gzipBody},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
				w.Header().Set("Content-Encoding", tc.encoding)
				_, _ = w.Write(tc.body)
			}))
			t.Cleanup(srv.Close)

			resp, err := CycleTLS{}.Do(srv.URL, Options{}, "GET")
			if err != nil {
				t.Fatal(err)
			}
			if resp.Body != string(payload) {
				t.Fatalf("body not decoded: got %d bytes, want %d", len(resp.Body), len(payload))
			}
			if string(resp.BodyBytes) != string(payload) {
				t.Errorf("BodyBytes not decoded")
			}
			if got, ok := resp.Headers["Content-Encoding"]; ok {
				t.Errorf("Content-Encoding %q still reported for a decoded body", got)
			}
		})
	}
}

// A body that does not decode comes back as it was received, with its
// headers, as before.
func TestDoLeavesUndecodableBodyAlone(t *testing.T) {
	srv := httptest.NewServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		w.Header().Set("Content-Encoding", "br")
		_, _ = w.Write([]byte("plain text, not brotli at all"))
	}))
	t.Cleanup(srv.Close)

	resp, err := CycleTLS{}.Do(srv.URL, Options{}, "GET")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Body != "plain text, not brotli at all" {
		t.Errorf("body = %q, want the bytes as received", resp.Body)
	}
	if resp.Headers["Content-Encoding"] != "br" {
		t.Errorf("Content-Encoding = %q, want br kept for a body left alone", resp.Headers["Content-Encoding"])
	}
}

// Before 3.0.1 Do built its Browser without ServerName, so the SNI override
// never reached the handshake.
func TestDoSendsServerName(t *testing.T) {
	var mu sync.Mutex
	var sni string
	srv := httptest.NewUnstartedServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	srv.TLS = &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			mu.Lock()
			sni = hello.ServerName
			mu.Unlock()
			return nil, nil
		},
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	resp, err := CycleTLS{}.Do(srv.URL, Options{
		InsecureSkipVerify: true,
		ServerName:         "sni-override.example.test",
	}, "GET")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, body = %q", resp.Status, resp.Body)
	}
	mu.Lock()
	defer mu.Unlock()
	if sni != "sni-override.example.test" {
		t.Errorf("server saw SNI %q, want the ServerName option", sni)
	}
}
