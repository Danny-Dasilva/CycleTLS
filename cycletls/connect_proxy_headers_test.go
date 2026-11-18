package cycletls

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"
)

// A local CONNECT proxy records the CONNECT request, then plays the origin
// inside the tunnel and records that request too. No network: the target host
// is never resolved because the proxy does not dial it.

type proxyCapture struct {
	connect string // raw CONNECT request head
	inner   string // raw request head sent through the tunnel
}

func readRequestHead(reader *bufio.Reader) (string, error) {
	var raw strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return raw.String(), err
		}
		raw.WriteString(line)
		if line == "\r\n" {
			return raw.String(), nil
		}
	}
}

func startRecordingConnectProxy(t *testing.T) (addr string, captures <-chan proxyCapture) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	out := make(chan proxyCapture, 4)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				reader := bufio.NewReader(conn)

				var capture proxyCapture
				var err error
				if capture.connect, err = readRequestHead(reader); err != nil {
					return
				}
				if _, err = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
					return
				}
				if capture.inner, err = readRequestHead(reader); err != nil {
					return
				}
				_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nOK"))
				out <- capture
			}(conn)
		}
	}()
	return listener.Addr().String(), out
}

func requestThroughRecordingProxy(t *testing.T, proxyURL string, captures <-chan proxyCapture, options Options) proxyCapture {
	t.Helper()
	options.Proxy = proxyURL
	options.ForceHTTP1 = true
	options.Timeout = 5

	client := Init()
	defer client.Close()
	resp, err := client.Do("http://origin.invalid/path", options, "GET")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200, body %q", resp.Status, resp.Body)
	}
	select {
	case capture := <-captures:
		return capture
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the proxy capture")
	}
	return proxyCapture{}
}

func TestProxyHeadersSentOnConnect(t *testing.T) {
	addr, captures := startRecordingConnectProxy(t)

	got := requestThroughRecordingProxy(t, "http://user:pass@"+addr, captures, Options{
		UserAgent: "CycleTLS-Test",
		Headers:   map[string]string{"X-Origin-Only": "o"},
		ProxyHeaders: map[string]string{
			"X-Proxy-Session": "abc123",
			"x-LOWER-case":    "kept",
		},
	})

	if !strings.HasPrefix(got.connect, "CONNECT origin.invalid:80 HTTP/1.1\r\n") {
		t.Fatalf("not a CONNECT request:\n%s", got.connect)
	}
	assertHeaderLine(t, got.connect, "X-Proxy-Session: abc123")
	assertHeaderLine(t, got.connect, "x-LOWER-case: kept")
	// Defaults stay unless overridden.
	assertHeaderLine(t, got.connect, "User-Agent: CycleTLS-Test")
	assertHeaderLine(t, got.connect, "Proxy-Authorization: Basic dXNlcjpwYXNz")
	// Request headers are for the origin, not the proxy.
	assertNoHeaderLine(t, got.connect, "X-Origin-Only:")

	// Proxy headers stop at the proxy.
	assertNoHeaderLine(t, got.inner, "X-Proxy-Session:")
	assertNoHeaderLine(t, got.inner, "x-LOWER-case:")
	assertNoHeaderLine(t, got.inner, "Proxy-Authorization:")
	assertHeaderLine(t, got.inner, "X-Origin-Only: o")
}

func TestProxyHeadersOverrideDefaults(t *testing.T) {
	addr, captures := startRecordingConnectProxy(t)

	got := requestThroughRecordingProxy(t, "http://user:pass@"+addr, captures, Options{
		UserAgent: "CycleTLS-Test",
		ProxyHeaders: map[string]string{
			"user-agent":          "proxy-agent",
			"Proxy-Authorization": "Bearer token",
		},
	})

	lower := strings.ToLower(got.connect)
	if n := strings.Count(lower, "\r\nuser-agent:"); n != 1 {
		t.Errorf("user-agent lines = %d, want 1\n%s", n, got.connect)
	}
	if !strings.Contains(lower, "\r\nuser-agent: proxy-agent\r\n") {
		t.Errorf("user-agent not overridden:\n%s", got.connect)
	}
	if n := strings.Count(lower, "\r\nproxy-authorization:"); n != 1 {
		t.Errorf("proxy-authorization lines = %d, want 1\n%s", n, got.connect)
	}
	assertHeaderLine(t, got.connect, "Proxy-Authorization: Bearer token")
}

func TestNoProxyHeadersLeavesConnectUnchanged(t *testing.T) {
	addr, captures := startRecordingConnectProxy(t)
	got := requestThroughRecordingProxy(t, "http://"+addr, captures, Options{UserAgent: "CycleTLS-Test"})

	want := "CONNECT origin.invalid:80 HTTP/1.1\r\nHost: origin.invalid:80\r\nUser-Agent: CycleTLS-Test\r\n\r\n"
	if got.connect != want {
		t.Errorf("CONNECT request changed:\n got %q\nwant %q", got.connect, want)
	}
}

func TestProxyHeadersJSONAndSocks(t *testing.T) {
	var opts Options
	if err := json.Unmarshal([]byte(`{"proxy":"http://127.0.0.1:1","proxyHeaders":{"X-Proxy-Session":"abc"}}`), &opts); err != nil {
		t.Fatal(err)
	}
	if opts.ProxyHeaders["X-Proxy-Session"] != "abc" {
		t.Fatalf("proxyHeaders not decoded: %+v", opts.ProxyHeaders)
	}
	if b := browserFromOptions(opts); b.ProxyHeaders["X-Proxy-Session"] != "abc" {
		t.Fatalf("browserFromOptions dropped proxyHeaders: %+v", b.ProxyHeaders)
	}

	// SOCKS has no CONNECT request; the option is accepted and does nothing.
	dialer, err := newConnectDialer("socks5://127.0.0.1:1", "ua", true)
	if err != nil {
		t.Fatal(err)
	}
	setProxyHeaders(dialer, map[string]string{"X-Proxy-Session": "abc"})
	// A dialer of another type is left alone.
	setProxyHeaders(&SocksDialer{}, map[string]string{"X-Proxy-Session": "abc"})
}
