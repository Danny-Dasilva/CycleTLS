package cycletls

import (
	"crypto/tls"
	"errors"
	"net"
	nhttp "net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	http "github.com/Danny-Dasilva/fhttp"
	"github.com/quic-go/quic-go/http3"
	utls "github.com/refraction-networking/utls"
)

// Local HTTP/3 and UDP fixtures only. No network.

// startLocalHTTP3Server serves HTTP/3 on a loopback UDP port with a self-signed
// certificate. The handler answers 200 with the sorted request header names,
// joined by commas, as the body.
func startLocalHTTP3Server(t *testing.T) string {
	t.Helper()

	udpConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	server := &http3.Server{
		TLSConfig: http3.ConfigureTLSConfig(&tls.Config{
			Certificates: []tls.Certificate{newLocalTestCert(t)},
		}),
		Handler: nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
			names := make([]string, 0, len(r.Header))
			for name := range r.Header {
				names = append(names, strings.ToLower(name))
			}
			sort.Strings(names)
			w.WriteHeader(200)
			_, _ = w.Write([]byte(strings.Join(names, ",")))
		}),
	}
	go func() { _ = server.Serve(udpConn) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = udpConn.Close()
	})
	return udpConn.LocalAddr().String()
}

// countUDPPackets listens on a loopback UDP port and counts every datagram.
func countUDPPackets(t *testing.T) (addr string, count *atomic.Int64) {
	t.Helper()
	udpConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	count = &atomic.Int64{}
	go func() {
		buf := make([]byte, 2048)
		for {
			if _, _, err := udpConn.ReadFrom(buf); err != nil {
				return
			}
			count.Add(1)
		}
	}()
	t.Cleanup(func() { _ = udpConn.Close() })
	return udpConn.LocalAddr().String(), count
}

// proxy + HTTP/3 must fail before any packet leaves: QUIC cannot go through the
// TCP proxy, so sending it anyway would expose the real IP.
func TestHTTP3WithProxyReturnsErrorAndSendsNoPackets(t *testing.T) {
	addr, packets := countUDPPackets(t)
	url := "https://" + addr + "/"
	const proxyURL = "http://127.0.0.1:9"

	t.Run("dispatchHTTP3Request", func(t *testing.T) {
		for _, opts := range []Options{
			{URL: url, Method: "GET", Proxy: proxyURL, ForceHTTP3: true, Timeout: 2},
			{URL: url, Method: "GET", Proxy: proxyURL, Protocol: "http3", Timeout: 2},
		} {
			res := processRequest(cycleTLSRequest{RequestID: "h3-proxy", Options: opts})
			if res.cancel != nil {
				defer res.cancel()
			}
			if !errors.Is(res.err, ErrHTTP3ProxyUnsupported) {
				t.Errorf("processRequest err = %v, want ErrHTTP3ProxyUnsupported", res.err)
			}
		}
	})

	t.Run("Do", func(t *testing.T) {
		client := Init()
		defer client.Close()
		_, err := client.Do(url, Options{Proxy: proxyURL, ForceHTTP3: true, Timeout: 2}, "GET")
		if !errors.Is(err, ErrHTTP3ProxyUnsupported) {
			t.Errorf("Do err = %v, want ErrHTTP3ProxyUnsupported", err)
		}
	})

	t.Run("RoundTrip", func(t *testing.T) {
		dialer, err := newConnectDialer(proxyURL, testChromeUA, true)
		if err != nil {
			t.Fatalf("newConnectDialer: %v", err)
		}
		rt := newRoundTripper(Browser{UserAgent: testChromeUA, ForceHTTP3: true}, dialer)
		defer closeClientConnections(rt)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		client := http.Client{Transport: rt, Timeout: 2 * time.Second}
		resp, err := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		if !errors.Is(err, ErrHTTP3ProxyUnsupported) {
			t.Errorf("RoundTrip err = %v, want ErrHTTP3ProxyUnsupported", err)
		}
	})

	time.Sleep(300 * time.Millisecond)
	if n := packets.Load(); n != 0 {
		t.Fatalf("UDP listener saw %d packets, want 0: HTTP/3 bypassed the proxy", n)
	}
}

func TestHTTP3InsecureSkipVerifyOption(t *testing.T) {
	addr := startLocalHTTP3Server(t)
	client := Init()
	defer client.Close()

	resp, err := client.Do("https://"+addr+"/", Options{
		ForceHTTP3:         true,
		InsecureSkipVerify: true,
		UserAgent:          testChromeUA,
		Timeout:            5,
	}, "GET")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("insecureSkipVerify=true: status = %d, want 200, body %q", resp.Status, resp.Body)
	}

	// Verification stays on unless asked: the self-signed cert must be refused.
	resp, err = client.Do("https://"+addr+"/", Options{
		ForceHTTP3: true,
		UserAgent:  testChromeUA,
		Timeout:    5,
	}, "GET")
	if err == nil && resp.Status == 200 {
		t.Fatalf("insecureSkipVerify=false: self-signed certificate was accepted")
	}
}

// headerOrder + HTTP/3 used to put fhttp's "Header-Order:" bookkeeping key into
// the header map, which quic-go refuses as an invalid header name.
func TestHTTP3HeaderOrderDoesNotBreakRequest(t *testing.T) {
	addr := startLocalHTTP3Server(t)

	// Browser.TLSConfig carries InsecureSkipVerify on the HTTP/3 path in 3.0.0
	// too, so this test fails there for the header reason alone.
	rt := newRoundTripper(Browser{
		UserAgent:   testChromeUA,
		ForceHTTP3:  true,
		HeaderOrder: []string{"x-second", "x-first"},
		TLSConfig:   &utls.Config{InsecureSkipVerify: true},
	})
	defer closeClientConnections(rt)

	req, err := http.NewRequest("GET", "https://"+addr+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header["x-first"] = []string{"1"}
	req.Header["x-second"] = []string{"2"}
	// A caller that built the request for HTTP/2 may have set this one as well.
	req.Header[http.PHeaderOrderKey] = []string{":method", ":authority", ":scheme", ":path"}

	client := http.Client{Transport: rt, Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body := make([]byte, 512)
	n, _ := resp.Body.Read(body)
	seen := string(body[:n])

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	for _, want := range []string{"x-first", "x-second", "user-agent"} {
		if !strings.Contains(seen, want) {
			t.Errorf("server did not see header %q (saw %q)", want, seen)
		}
	}
	if strings.Contains(seen, "order") {
		t.Errorf("bookkeeping key reached the server: %q", seen)
	}
}

func TestHTTP3HeaderOrderThroughDo(t *testing.T) {
	addr := startLocalHTTP3Server(t)
	client := Init()
	defer client.Close()

	resp, err := client.Do("https://"+addr+"/", Options{
		ForceHTTP3:         true,
		InsecureSkipVerify: true,
		UserAgent:          testChromeUA,
		Headers:            map[string]string{"x-first": "1", "x-second": "2"},
		HeaderOrder:        []string{"x-second", "x-first"},
		Timeout:            5,
	}, "GET")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200, body %q", resp.Status, resp.Body)
	}
	if !strings.Contains(resp.Body, "x-first") || !strings.Contains(resp.Body, "x-second") {
		t.Fatalf("server saw %q, want x-first and x-second", resp.Body)
	}
}

// quicFingerprint must not touch the TCP ClientHello. In 3.0.0 it replaced the
// ALPN list with "h3", which no TCP server speaks.
func TestQUICFingerprintLeavesTCPClientHelloAlone(t *testing.T) {
	var mu sync.Mutex
	var offered []string

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{newLocalTestCert(t)},
		NextProtos:   []string{"http/1.1"},
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			mu.Lock()
			offered = append([]string(nil), hello.SupportedProtos...)
			mu.Unlock()
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(5 * time.Second))
				buf := make([]byte, 4096)
				if _, err := c.Read(buf); err != nil {
					return
				}
				_, _ = c.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nOK"))
			}(conn)
		}
	}()

	client := Init()
	defer client.Close()
	resp, err := client.Do("https://"+listener.Addr().String()+"/", Options{
		QUICFingerprint:    strings.Repeat("16030106f2010006ee0303", 10),
		UserAgent:          testChromeUA,
		InsecureSkipVerify: true,
		Timeout:            5,
	}, "GET")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	mu.Lock()
	got := append([]string(nil), offered...)
	mu.Unlock()

	for _, proto := range got {
		if proto == "h3" {
			t.Fatalf("TCP ClientHello offered ALPN %v: quicFingerprint changed the TCP handshake", got)
		}
	}
	want := []string{"h2", "http/1.1"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("TCP ClientHello ALPN = %v, want %v", got, want)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200, body %q", resp.Status, resp.Body)
	}
}
