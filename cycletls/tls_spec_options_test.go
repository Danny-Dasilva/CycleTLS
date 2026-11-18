package cycletls

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	nhttp "net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	fhttp "github.com/Danny-Dasilva/fhttp"
	utls "github.com/refraction-networking/utls"
)

const chromeTestUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36"

// --- a local TLS server that records the ClientHello of each connection ---

type recordingConn struct {
	net.Conn
	mu  sync.Mutex
	buf []byte
}

func (c *recordingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.mu.Lock()
	if len(c.buf) < 1<<16 {
		c.buf = append(c.buf, p[:n]...)
	}
	c.mu.Unlock()
	return n, err
}

type recordingListener struct {
	net.Listener
	mu    sync.Mutex
	conns []*recordingConn
}

func (l *recordingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	rc := &recordingConn{Conn: c}
	l.mu.Lock()
	l.conns = append(l.conns, rc)
	l.mu.Unlock()
	return rc, nil
}

type helloServer struct {
	*httptest.Server
	ln *recordingListener
}

// newHelloServer starts an httptest TLS server (HTTP/1.1 and HTTP/2) on
// 127.0.0.1 that keeps the bytes each client sends first.
func newHelloServer(t *testing.T) *helloServer {
	t.Helper()
	srv := httptest.NewUnstartedServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	ln := &recordingListener{Listener: srv.Listener}
	srv.Listener = ln
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return &helloServer{Server: srv, ln: ln}
}

func (s *helloServer) addr() string { return s.Listener.Addr().String() }

// hellos returns the ClientHello records received so far, one per connection.
func (s *helloServer) hellos(t *testing.T) [][]byte {
	t.Helper()
	s.ln.mu.Lock()
	conns := append([]*recordingConn(nil), s.ln.conns...)
	s.ln.mu.Unlock()
	var out [][]byte
	for _, c := range conns {
		c.mu.Lock()
		b := append([]byte(nil), c.buf...)
		c.mu.Unlock()
		if len(b) == 0 {
			continue
		}
		if _, err := ParseClientHello(b); err != nil {
			t.Fatalf("server received something that is not a ClientHello: %v", err)
		}
		out = append(out, b)
	}
	return out
}

func (s *helloServer) lastHello(t *testing.T) []byte {
	t.Helper()
	hs := s.hellos(t)
	if len(hs) == 0 {
		t.Fatal("server recorded no ClientHello")
	}
	return hs[len(hs)-1]
}

// handshake dials the server and completes a TLS handshake with spec, the way
// roundTripper.dialTLS does.
func (s *helloServer) handshake(t *testing.T, spec *utls.ClientHelloSpec) utls.ConnectionState {
	t.Helper()
	raw, err := net.DialTimeout("tcp", s.addr(), 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(10 * time.Second))
	uc := utls.UClient(raw, &utls.Config{
		ServerName:         specTestServerName,
		OmitEmptyPsk:       true,
		InsecureSkipVerify: true,
	}, utls.HelloCustom)
	if err := uc.ApplyPreset(spec); err != nil {
		t.Fatalf("ApplyPreset: %v", err)
	}
	if err := uc.Handshake(); err != nil {
		t.Fatalf("handshake: %v", err)
	}
	return uc.ConnectionState()
}

// get sends one request through a CycleTLS round tripper built from browser
// and returns the ClientHello the server received for it.
func (s *helloServer) get(t *testing.T, browser Browser) ([]byte, error) {
	t.Helper()
	browser.InsecureSkipVerify = true
	if browser.ServerName == "" {
		// The server listens on 127.0.0.1 and utls sends no SNI for an IP
		// address, so name the host the way a real request would.
		browser.ServerName = specTestServerName
	}
	before := len(s.hellos(t))
	rt := newRoundTripper(browser)
	defer rt.(*roundTripper).CloseIdleConnections()
	client := &fhttp.Client{Transport: rt, Timeout: 15 * time.Second}
	resp, err := client.Get(s.URL)
	if err != nil {
		return nil, err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	hs := s.hellos(t)
	if len(hs) <= before {
		t.Fatal("request made no new TLS connection")
	}
	return hs[before], nil
}

// --- comparison helpers ---

// sortedExtJA3 is the JA3 string with the extension list sorted, for specs
// that shuffle their extensions.
func sortedExtJA3(info *ClientHelloInfo) string {
	parts := strings.Split(info.JA3(), ",")
	ids := strings.Split(parts[2], "-")
	sort.Strings(ids)
	parts[2] = strings.Join(ids, "-")
	return strings.Join(parts, ",")
}

// withoutPadding returns a copy of the hello without the padding extension.
// BoringSSL-style padding is sent only when the rest of the hello is 256 to
// 511 bytes long, and the GREASE ECH extension of the Chrome specs has a
// random length, so two hellos from one spec can differ in whether they carry
// padding.
func withoutPadding(info *ClientHelloInfo) *ClientHelloInfo {
	out := *info
	out.Extensions, out.ExtensionData = nil, nil
	for i, id := range info.Extensions {
		if id == 0x0015 {
			continue
		}
		out.Extensions = append(out.Extensions, id)
		out.ExtensionData = append(out.ExtensionData, info.ExtensionData[i])
	}
	return &out
}

func extOrder(info *ClientHelloInfo) string {
	return joinHex(withoutGREASE(info.Extensions))
}

func referenceHello(t *testing.T, id utls.ClientHelloID) *ClientHelloInfo {
	t.Helper()
	raw, err := buildClientHelloErr(nil, id)
	if err != nil {
		t.Fatalf("build reference hello for %s: %v", id.Str(), err)
	}
	return withoutPadding(mustParseHello(t, raw))
}

// --- item 3: named profiles ---

func TestTLSProfileNames(t *testing.T) {
	names := TLSProfileNames()
	if !sort.StringsAreSorted(names) {
		t.Errorf("TLSProfileNames is not sorted: %v", names)
	}
	have := map[string]bool{}
	for _, n := range names {
		have[n] = true
		if n != normalizeTLSProfileName(n) {
			t.Errorf("profile name %q is not in normal form", n)
		}
	}
	for _, want := range []string{"chrome_133", "firefox_120", "safari_16_0", "ios_14", "edge_85"} {
		if !have[want] {
			t.Errorf("TLSProfileNames lacks %q", want)
		}
	}

	_, err := TLSProfileToSpec("netscape_4", false)
	if err == nil {
		t.Fatal("TLSProfileToSpec accepted an unknown name")
	}
	for _, n := range names {
		if !strings.Contains(err.Error(), n) {
			t.Errorf("error for an unknown profile does not list %q: %v", n, err)
		}
	}

	for _, alias := range []string{"Chrome_133", " chrome-133 ", "Safari_16.0", "safari 16.0"} {
		if _, err := TLSProfileToSpec(alias, false); err != nil {
			t.Errorf("TLSProfileToSpec(%q): %v", alias, err)
		}
	}
}

// Every profile completes a handshake with a local TLS server, and the hello
// the server receives has the JA3 and JA4 of the utls spec it is named after.
func TestTLSProfilesHandshakeAndMatchUTLS(t *testing.T) {
	srv := newHelloServer(t)
	for _, name := range TLSProfileNames() {
		t.Run(name, func(t *testing.T) {
			spec, err := TLSProfileToSpec(name, false)
			if err != nil {
				t.Fatalf("TLSProfileToSpec: %v", err)
			}
			state := srv.handshake(t, spec)
			if !state.HandshakeComplete {
				t.Fatal("handshake did not complete")
			}
			got := withoutPadding(mustParseHello(t, srv.lastHello(t)))
			if got.ServerName != specTestServerName {
				t.Errorf("SNI = %q, want %q", got.ServerName, specTestServerName)
			}

			id := tlsProfiles[name]
			ref, ref2 := referenceHello(t, id), referenceHello(t, id)
			if got.JA4() != ref.JA4() {
				t.Errorf("JA4 = %q, want %q (utls %s)", got.JA4(), ref.JA4(), id.Str())
			}
			if ref.JA3() == ref2.JA3() {
				// The utls spec has a fixed extension order.
				if got.JA3() != ref.JA3() {
					t.Errorf("JA3 = %q\nwant %q (utls %s)", got.JA3(), ref.JA3(), id.Str())
				}
			} else if sortedExtJA3(got) != sortedExtJA3(ref) {
				// The utls spec shuffles: compare with the order taken out.
				t.Errorf("JA3 with sorted extensions = %q\nwant %q (utls %s)", sortedExtJA3(got), sortedExtJA3(ref), id.Str())
			}
		})
	}
}

func TestTLSProfileForceHTTP1(t *testing.T) {
	srv := newHelloServer(t)
	spec, err := TLSProfileToSpec("chrome_133", true)
	if err != nil {
		t.Fatal(err)
	}
	state := srv.handshake(t, spec)
	// Go's TLS server answers a client that offers http/1.1 alone with no
	// ALPN at all, which also means HTTP/1.1.
	if p := state.NegotiatedProtocol; p != "http/1.1" && p != "" {
		t.Errorf("negotiated %q, want http/1.1", p)
	}
	info := mustParseHello(t, srv.lastHello(t))
	if len(info.ALPN) != 1 || info.ALPN[0] != "http/1.1" {
		t.Errorf("ALPN = %v, want [http/1.1]", info.ALPN)
	}
	for _, alps := range []uint16{17513, 17613} {
		if _, ok := info.ExtensionBody(alps); ok {
			t.Errorf("ALPS extension %d sent with forceHTTP1", alps)
		}
	}
}

// The option reaches the wire through a real request, and wins over ja3 and
// ja4r.
func TestTLSProfileThroughRoundTripper(t *testing.T) {
	srv := newHelloServer(t)
	const ja3 = "771,4865-4866-4867-49195-49199,0-23-65281-10-11-35-16-5-13-51-45-43,29-23-24,0"
	for _, name := range []string{"chrome_133", "firefox_120", "safari_16_0"} {
		t.Run(name, func(t *testing.T) {
			raw, err := srv.get(t, Browser{TLSProfile: name, JA3: ja3, JA4r: foxioJA4R, UserAgent: chromeTestUA})
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			got := withoutPadding(mustParseHello(t, raw))
			if want := referenceHelloFor(t, name, got.ServerName); got.JA4() != want {
				t.Errorf("JA4 = %q, want the profile's %q", got.JA4(), want)
			}
		})
	}

	t.Run("unknown name fails the request", func(t *testing.T) {
		_, err := srv.get(t, Browser{TLSProfile: "netscape_4", UserAgent: chromeTestUA})
		if err == nil || !strings.Contains(err.Error(), "unknown tlsProfile") || !strings.Contains(err.Error(), "chrome_133") {
			t.Errorf("error = %v, want one that names the valid profiles", err)
		}
	})
}

// referenceHelloFor returns the JA4 of the utls hello for a profile, built for
// the given server name (padding depends on the hello length).
func referenceHelloFor(t *testing.T, name, serverName string) string {
	t.Helper()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	uc := utls.UClient(c1, &utls.Config{ServerName: serverName, OmitEmptyPsk: true, InsecureSkipVerify: true}, tlsProfiles[name])
	if err := uc.BuildHandshakeState(); err != nil {
		t.Fatalf("reference hello: %v", err)
	}
	return withoutPadding(mustParseHello(t, uc.HandshakeState.Hello.Raw)).JA4()
}

// --- item 4: extension shuffle ---

func assertShuffles(t *testing.T, build func() *ClientHelloInfo) {
	t.Helper()
	first := build()
	orders := map[string]bool{extOrder(first): true}
	for i := 0; i < 5; i++ {
		next := build()
		orders[extOrder(next)] = true
		if next.JA4() != first.JA4() {
			t.Errorf("JA4 changed across builds: %q then %q", first.JA4(), next.JA4())
		}
		if sortedExtJA3(next) != sortedExtJA3(first) {
			t.Errorf("extension set changed across builds:\n%q\n%q", sortedExtJA3(first), sortedExtJA3(next))
		}
	}
	if len(orders) < 2 {
		t.Errorf("extension order was the same in 6 builds: %s", extOrder(first))
	}
}

func assertFixedOrder(t *testing.T, build func() *ClientHelloInfo) {
	t.Helper()
	first := extOrder(build())
	for i := 0; i < 3; i++ {
		if next := extOrder(build()); next != first {
			t.Errorf("extension order changed without shuffleExtensions:\n%s\n%s", first, next)
		}
	}
}

func TestShuffleExtensions(t *testing.T) {
	srv := newHelloServer(t)
	viaRequest := func(b Browser) func() *ClientHelloInfo {
		return func() *ClientHelloInfo {
			raw, err := srv.get(t, b)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			return mustParseHello(t, raw)
		}
	}
	ja4rExts := []uint16{0x0000, 0x0005, 0x000a, 0x000b, 0x000d, 0x0010, 0x0017, 0x0023, 0x002b, 0x002d, 0x0033, 0xff01}
	ja4r := ja4rString(ja4rTestCiphers, ja4rExts, ja4rTestSigAlgs)

	t.Run("default Chrome", func(t *testing.T) {
		assertFixedOrder(t, viaRequest(Browser{UserAgent: chromeTestUA}))
		assertShuffles(t, viaRequest(Browser{UserAgent: chromeTestUA, ShuffleExtensions: true}))
	})
	t.Run("default Chrome keeps GREASE first and last", func(t *testing.T) {
		raw, err := srv.get(t, Browser{UserAgent: chromeTestUA, ShuffleExtensions: true})
		if err != nil {
			t.Fatal(err)
		}
		ids := mustParseHello(t, raw).Extensions
		if !IsGREASEValue(ids[0]) || !IsGREASEValue(ids[len(ids)-1]) {
			t.Errorf("GREASE moved: first 0x%04x, last 0x%04x", ids[0], ids[len(ids)-1])
		}
	})
	t.Run("ja4r", func(t *testing.T) {
		assertFixedOrder(t, viaRequest(Browser{JA4r: ja4r, UserAgent: firefoxTestUA, DisableGrease: true}))
		assertShuffles(t, viaRequest(Browser{JA4r: ja4r, UserAgent: firefoxTestUA, DisableGrease: true, ShuffleExtensions: true}))
	})
	t.Run("profile", func(t *testing.T) {
		// firefox_120 has a fixed order in utls, so any change comes from the option.
		assertFixedOrder(t, viaRequest(Browser{TLSProfile: "firefox_120", UserAgent: firefoxTestUA}))
		assertShuffles(t, viaRequest(Browser{TLSProfile: "firefox_120", UserAgent: firefoxTestUA, ShuffleExtensions: true}))
	})
	t.Run("explicit ja3 is not shuffled", func(t *testing.T) {
		const ja3 = "771,4865-4866-4867-49195-49199,0-23-65281-10-11-35-16-5-13-51-45-43,29-23-24,0"
		build := viaRequest(Browser{JA3: ja3, UserAgent: firefoxTestUA, ShuffleExtensions: true})
		for i := 0; i < 4; i++ {
			if got := build().JA3(); got != ja3 {
				t.Fatalf("JA3 on the wire = %q, want %q", got, ja3)
			}
		}
	})
	t.Run("imported hello is not shuffled", func(t *testing.T) {
		hello := hex.EncodeToString(buildClientHello(t, ModernChromeSpec(false)))
		assertFixedOrder(t, viaRequest(Browser{ClientHello: hello, UserAgent: chromeTestUA, ShuffleExtensions: true}))
	})
}

// --- item 5: ClientHello importer ---

// appendExtension adds one extension to the end of a ClientHello handshake
// message and fixes the two length fields.
func appendExtension(t *testing.T, msg []byte, id uint16, body []byte) []byte {
	t.Helper()
	info := mustParseHello(t, msg)
	extBytes := 0
	for _, b := range info.ExtensionData {
		extBytes += 4 + len(b)
	}
	extLenPos := len(msg) - extBytes - 2
	out := append([]byte(nil), msg...)
	out = append(out, byte(id>>8), byte(id), byte(len(body)>>8), byte(len(body)))
	out = append(out, body...)
	newExt := extBytes + 4 + len(body)
	out[extLenPos], out[extLenPos+1] = byte(newExt>>8), byte(newExt)
	n := len(out) - 4
	out[1], out[2], out[3] = byte(n>>16), byte(n>>8), byte(n)
	if got := mustParseHello(t, out); got.Extensions[len(got.Extensions)-1] != id {
		t.Fatalf("appendExtension produced a hello without extension 0x%04x last", id)
	}
	return out
}

func TestClientHelloImportRoundTrip(t *testing.T) {
	const ja3 = "771,4865-4866-4867-49195-49199,0-23-65281-10-11-35-16-5-15-17-13-30031-51-45-43,29-23-24,0"
	sources := map[string]func() *utls.ClientHelloSpec{
		"default Chrome": func() *utls.ClientHelloSpec { return ModernChromeSpec(false) },
		"firefox_120": func() *utls.ClientHelloSpec {
			s, err := TLSProfileToSpec("firefox_120", false)
			if err != nil {
				t.Fatal(err)
			}
			return s
		},
		"safari_16_0": func() *utls.ClientHelloSpec {
			s, err := TLSProfileToSpec("safari_16_0", false)
			if err != nil {
				t.Fatal(err)
			}
			return s
		},
		"ja3": func() *utls.ClientHelloSpec {
			s, err := StringToSpec(ja3, firefoxTestUA, false)
			if err != nil {
				t.Fatal(err)
			}
			return s
		},
	}
	srv := newHelloServer(t)
	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			msg := buildClientHello(t, source())
			orig := mustParseHello(t, msg)

			forms := map[string]string{
				"record":            hex.EncodeToString(wrapInRecords(msg, len(msg))),
				"handshake message": hex.EncodeToString(msg),
				"split records":     strings.ToUpper(hex.EncodeToString(wrapInRecords(msg, 200))),
				"0x and spaces":     "0x" + spaced(hex.EncodeToString(wrapInRecords(msg, len(msg)))),
			}
			for form, hexHello := range forms {
				spec, err := ClientHelloToSpec(hexHello, false)
				if err != nil {
					t.Fatalf("%s: ClientHelloToSpec: %v", form, err)
				}
				got := mustParseHello(t, buildClientHello(t, spec))
				if got.JA3() != orig.JA3() {
					t.Errorf("%s: JA3 = %q\nwant %q", form, got.JA3(), orig.JA3())
				}
				if got.JA4() != orig.JA4() || got.JA4R() != orig.JA4R() {
					t.Errorf("%s: JA4_r = %q\nwant %q", form, got.JA4R(), orig.JA4R())
				}
				if strings.Join(got.ALPN, ",") != strings.Join(orig.ALPN, ",") {
					t.Errorf("%s: ALPN = %v, want %v", form, got.ALPN, orig.ALPN)
				}
				// Bodies that hold no per-connection value come back byte for byte.
				for _, id := range []uint16{5, 11, 13, 15, 16, 17, 27, 28, 34, 45, 17613, 30031} {
					want, ok := orig.ExtensionBody(id)
					if !ok {
						continue
					}
					if body, _ := got.ExtensionBody(id); hexBody(body) != hexBody(want) {
						t.Errorf("%s: extension %d body = %s, want %s", form, id, hexBody(body), hexBody(want))
					}
				}
			}

			// The imported hello completes a handshake.
			spec, err := ClientHelloToSpec(forms["record"], false)
			if err != nil {
				t.Fatal(err)
			}
			srv.handshake(t, spec)
			if got := mustParseHello(t, srv.lastHello(t)); got.JA3() != orig.JA3() {
				t.Errorf("JA3 received by the server = %q\nwant %q", got.JA3(), orig.JA3())
			}
		})
	}
}

func spaced(h string) string {
	var b strings.Builder
	for i := 0; i < len(h); i += 2 {
		if i > 0 {
			if i%32 == 0 {
				b.WriteString("\n")
			} else {
				b.WriteString(" ")
			}
		}
		b.WriteString(h[i : i+2])
	}
	return b.String()
}

// The server name of the capture is not replayed: the SNI extension is sent
// with the host of the new connection.
func TestClientHelloImportBlanksSNI(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	uc := utls.UClient(c1, &utls.Config{ServerName: "captured.test", OmitEmptyPsk: true}, utls.HelloCustom)
	if err := uc.ApplyPreset(ModernChromeSpec(false)); err != nil {
		t.Fatal(err)
	}
	if err := uc.BuildHandshakeState(); err != nil {
		t.Fatal(err)
	}
	captured := uc.HandshakeState.Hello.Raw
	if got := mustParseHello(t, captured).ServerName; got != "captured.test" {
		t.Fatalf("captured hello has SNI %q", got)
	}

	spec, err := ClientHelloToSpec(hex.EncodeToString(captured), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, ext := range spec.Extensions {
		if sni, ok := ext.(*utls.SNIExtension); ok && sni.ServerName != "" {
			t.Errorf("spec keeps the captured server name %q", sni.ServerName)
		}
	}
	got := mustParseHello(t, buildClientHello(t, spec))
	if got.ServerName != specTestServerName {
		t.Errorf("SNI = %q, want %q", got.ServerName, specTestServerName)
	}
}

// A captured pre_shared_key carries another session's ticket and binder. It is
// not replayed.
func TestClientHelloImportDropsPSK(t *testing.T) {
	msg := buildClientHello(t, ModernChromeSpec(false))
	without := mustParseHello(t, msg)

	// identities: one 16-byte ticket with an obfuscated age; binders: one of 32 bytes.
	var psk []byte
	psk = append(psk, 0x00, 22, 0x00, 16)
	psk = append(psk, make([]byte, 16)...)
	psk = append(psk, 1, 2, 3, 4)
	psk = append(psk, 0x00, 33, 32)
	psk = append(psk, make([]byte, 32)...)
	withPSK := appendExtension(t, msg, 0x0029, psk)

	spec, err := ClientHelloToSpec(hex.EncodeToString(wrapInRecords(withPSK, len(withPSK))), false)
	if err != nil {
		t.Fatalf("ClientHelloToSpec: %v", err)
	}
	for _, ext := range spec.Extensions {
		if _, ok := ext.(utls.PreSharedKeyExtension); ok {
			t.Errorf("spec keeps the captured pre_shared_key extension")
		}
	}
	got := mustParseHello(t, buildClientHello(t, spec))
	if _, ok := got.ExtensionBody(0x0029); ok {
		t.Errorf("pre_shared_key was replayed")
	}
	if got.JA3() != without.JA3() {
		t.Errorf("JA3 = %q\nwant the hello without the PSK: %q", got.JA3(), without.JA3())
	}
}

func TestClientHelloImportForceHTTP1(t *testing.T) {
	msg := buildClientHello(t, ModernChromeSpec(false))
	spec, err := ClientHelloToSpec(hex.EncodeToString(msg), true)
	if err != nil {
		t.Fatal(err)
	}
	got := mustParseHello(t, buildClientHello(t, spec))
	if len(got.ALPN) != 1 || got.ALPN[0] != "http/1.1" {
		t.Errorf("ALPN = %v, want [http/1.1]", got.ALPN)
	}
	if _, ok := got.ExtensionBody(17613); ok {
		t.Errorf("ALPS sent with forceHTTP1")
	}
}

func TestClientHelloImportErrors(t *testing.T) {
	serverHello := "160303000402000000"
	for name, in := range map[string]string{
		"not hex":         "zz16030100",
		"odd length":      "16030",
		"empty":           "  ",
		"not a hello":     serverHello,
		"truncated hello": hex.EncodeToString(buildClientHello(t, ModernChromeSpec(false))[:60]),
	} {
		if spec, err := ClientHelloToSpec(in, false); err == nil || spec != nil {
			t.Errorf("%s: ClientHelloToSpec returned spec=%v err=%v, want an error", name, spec != nil, err)
		} else if !strings.HasPrefix(err.Error(), "clientHello:") {
			t.Errorf("%s: error %q does not name the option", name, err)
		}
	}
}

// The option reaches the wire through a real request and wins over tlsProfile
// and ja3; bad hex fails the request.
func TestClientHelloThroughRoundTripper(t *testing.T) {
	srv := newHelloServer(t)
	ffSpec, err := TLSProfileToSpec("firefox_120", false)
	if err != nil {
		t.Fatal(err)
	}
	msg := buildClientHello(t, ffSpec) // SNI: specTestServerName
	want := mustParseHello(t, msg)

	raw, err := srv.get(t, Browser{
		ClientHello: hex.EncodeToString(wrapInRecords(msg, len(msg))),
		TLSProfile:  "chrome_133",
		JA3:         "771,4865-4866,0-10-11-13-43-51,29-23,0",
		UserAgent:   chromeTestUA,
		ServerName:  "other.test",
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	got := mustParseHello(t, raw)
	if got.JA3() != want.JA3() || got.JA4() != want.JA4() {
		t.Errorf("hello on the wire: JA3 %q JA4 %q\nwant the imported: JA3 %q JA4 %q", got.JA3(), got.JA4(), want.JA3(), want.JA4())
	}
	if got.ServerName != "other.test" {
		t.Errorf("SNI = %q, want other.test (the capture had %q)", got.ServerName, want.ServerName)
	}

	if _, err := srv.get(t, Browser{ClientHello: "nothex", UserAgent: chromeTestUA}); err == nil || !strings.Contains(err.Error(), "not valid hex") {
		t.Errorf("error = %v, want a hex error", err)
	}
}

// --- plumbing ---

func TestTLSSpecOptionsDecodeAndReachBrowser(t *testing.T) {
	var opts Options
	in := `{"url":"https://example.com","tlsProfile":"chrome_133","shuffleExtensions":true,"clientHello":"1603"}`
	if err := json.Unmarshal([]byte(in), &opts); err != nil {
		t.Fatal(err)
	}
	if opts.TLSProfile != "chrome_133" || !opts.ShuffleExtensions || opts.ClientHello != "1603" {
		t.Fatalf("Options = %+v", opts)
	}
	b := browserFromOptions(opts)
	if b.TLSProfile != "chrome_133" || !b.ShuffleExtensions || b.ClientHello != "1603" {
		t.Errorf("Browser = profile %q shuffle %v hello %q", b.TLSProfile, b.ShuffleExtensions, b.ClientHello)
	}
	rt := newRoundTripper(b).(*roundTripper)
	if rt.TLSProfile != "chrome_133" || !rt.ShuffleExtensions || rt.ClientHello != "1603" {
		t.Errorf("roundTripper = profile %q shuffle %v hello %q", rt.TLSProfile, rt.ShuffleExtensions, rt.ClientHello)
	}
}

func TestTLSSpecOptionsChangeClientKey(t *testing.T) {
	base := generateClientKey(Browser{UserAgent: chromeTestUA}, 10, false, "")
	variants := map[string]Browser{
		"tlsProfile":        {UserAgent: chromeTestUA, TLSProfile: "chrome_133"},
		"other tlsProfile":  {UserAgent: chromeTestUA, TLSProfile: "firefox_120"},
		"shuffleExtensions": {UserAgent: chromeTestUA, ShuffleExtensions: true},
		"clientHello":       {UserAgent: chromeTestUA, ClientHello: "1603"},
	}
	seen := map[string]string{base: "base"}
	for name, b := range variants {
		key := generateClientKey(b, 10, false, "")
		if other, dup := seen[key]; dup {
			t.Errorf("%s shares a pool key with %s", name, other)
		}
		seen[key] = name
	}
	// Two spellings of one profile are the same client.
	a := generateClientKey(Browser{TLSProfile: "Safari_16.0"}, 10, false, "")
	b := generateClientKey(Browser{TLSProfile: "safari_16_0"}, 10, false, "")
	if a != b {
		t.Errorf("two spellings of one profile got different pool keys")
	}
}
