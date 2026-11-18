package cycletls

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	nhttp "net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	fhttp "github.com/Danny-Dasilva/fhttp"
)

// These tests cover the 3.0.1 TLS handshake options. They use local servers
// with a certificate from a CA generated in the test; nothing leaves the host.

const (
	hsTestChromeUA  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	hsTestFirefoxUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0"
	testCustomSNI   = "custom.test"

	extPreSharedKey = 41
)

// testPKI is a private CA and a server certificate signed by it.
type testPKI struct {
	caPEM      string
	caCert     *x509.Certificate
	leafCert   *x509.Certificate
	serverCert tls.Certificate // chain: leaf, CA
}

func newTestPKI(t *testing.T) *testPKI {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "CycleTLS test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "CycleTLS test server"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost", testCustomSNI},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	leafCert, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatal(err)
	}

	return &testPKI{
		caPEM:    string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})),
		caCert:   caCert,
		leafCert: leafCert,
		serverCert: tls.Certificate{
			Certificate: [][]byte{leafDER, caDER},
			PrivateKey:  leafKey,
			Leaf:        leafCert,
		},
	}
}

// seenHello is what the server read from one ClientHello.
type seenHello struct {
	sni        string
	extensions []uint16 // in wire order, GREASE removed
	alpn       []string
}

func (h seenHello) has(ext uint16) bool {
	for _, e := range h.extensions {
		if e == ext {
			return true
		}
	}
	return false
}

func (h seenHello) last() uint16 {
	if len(h.extensions) == 0 {
		return 0
	}
	return h.extensions[len(h.extensions)-1]
}

// helloRecorder is a tls.Config.GetConfigForClient hook that keeps every
// ClientHello and can fail the first n handshakes.
type helloRecorder struct {
	mu        sync.Mutex
	hellos    []seenHello
	failFirst int
}

func isGREASE(v uint16) bool {
	return v&0x0f0f == 0x0a0a && v>>8 == v&0xff
}

func (r *helloRecorder) hook(chi *tls.ClientHelloInfo) (*tls.Config, error) {
	h := seenHello{sni: chi.ServerName, alpn: append([]string(nil), chi.SupportedProtos...)}
	for _, e := range chi.Extensions {
		if !isGREASE(e) {
			h.extensions = append(h.extensions, e)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hellos = append(r.hellos, h)
	if len(r.hellos) <= r.failFirst {
		return nil, errors.New("test server: handshake refused")
	}
	return nil, nil
}

func (r *helloRecorder) all() []seenHello {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]seenHello(nil), r.hellos...)
}

// handshakeServer is an HTTPS server that reports, per request, whether the
// TLS session was resumed and which protocol was used.
type handshakeServer struct {
	*httptest.Server
	rec      *helloRecorder
	mu       sync.Mutex
	requests int
}

func (s *handshakeServer) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}

func newHandshakeServer(t *testing.T, pki *testPKI, h2 bool, failFirst int) *handshakeServer {
	t.Helper()
	hs := &handshakeServer{rec: &helloRecorder{failFirst: failFirst}}
	srv := httptest.NewUnstartedServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		hs.mu.Lock()
		hs.requests++
		hs.mu.Unlock()
		fmt.Fprintf(w, "resumed=%t proto=%s", r.TLS.DidResume, r.Proto)
	}))
	srv.TLS = &tls.Config{
		Certificates:       []tls.Certificate{pki.serverCert},
		GetConfigForClient: hs.rec.hook,
	}
	srv.EnableHTTP2 = h2
	// The refused handshakes in the retry tests are expected; keep them out of the test log.
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	hs.Server = srv
	return hs
}

// getOnce sends one GET through a client built the way the dispatchers build
// it with connection reuse off (a new roundTripper per request), then closes
// that client's connections.
func getOnce(t *testing.T, browser Browser, url string) (string, error) {
	t.Helper()
	if browser.UserAgent == "" {
		browser.UserAgent = hsTestChromeUA
	}
	client, err := newClientWithReuse(browser, 10, false, browser.UserAgent, false)
	if err != nil {
		return "", err
	}
	defer closeClientConnections(client.Transport)
	return getWith(&client, url)
}

func getWith(client *fhttp.Client, url string) (string, error) {
	client.Timeout = 10 * time.Second
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func mustGet(t *testing.T, browser Browser, url string) string {
	t.Helper()
	body, err := getOnce(t, browser, url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return body
}

// wrongPins match no certificate, in each of the two pin forms.
var wrongPins = []string{
	spkiPinPrefix + base64.StdEncoding.EncodeToString(make([]byte, sha256.Size)),
	strings.Repeat("00", sha256.Size),
}

// --- item 1: session resumption -------------------------------------------

func TestSessionResumption(t *testing.T) {
	pki := newTestPKI(t)

	cases := []struct {
		name    string
		browser Browser
		h2      bool
		proto   string
		// tls12: the hello offers TLS 1.2 only, so the session is resumed with
		// a session ticket (extension 35) and extension 41 never appears.
		tls12 bool
	}{
		{name: "chrome_default_h2", browser: Browser{UserAgent: hsTestChromeUA}, h2: true, proto: "HTTP/2.0"},
		{name: "chrome_default_h1", browser: Browser{UserAgent: hsTestChromeUA, ForceHTTP1: true}, proto: "HTTP/1.1"},
		{name: "ja3_tls13_h2", browser: Browser{UserAgent: hsTestChromeUA, JA3: DefaultChrome_JA3, TLS13AutoRetry: true}, h2: true, proto: "HTTP/2.0"},
		{name: "ja3_tls12_h2", browser: Browser{UserAgent: hsTestChromeUA, JA3: DefaultChrome_JA3}, h2: true, proto: "HTTP/2.0", tls12: true},
		{name: "firefox_default_h2", browser: Browser{UserAgent: hsTestFirefoxUA}, h2: true, proto: "HTTP/2.0"},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/on", func(t *testing.T) {
			clearSessionCaches()
			srv := newHandshakeServer(t, pki, tc.h2, 0)
			browser := tc.browser
			browser.CACerts = []string{pki.caPEM}
			browser.SessionResumption = true

			// Connection reuse is off: each request is a new roundTripper and a
			// new connection, so resumption needs the shared session cache.
			first := mustGet(t, browser, srv.URL)
			second := mustGet(t, browser, srv.URL)

			if want := "resumed=false proto=" + tc.proto; first != want {
				t.Errorf("first request: got %q, want %q", first, want)
			}
			if want := "resumed=true proto=" + tc.proto; second != want {
				t.Errorf("second request: got %q, want %q", second, want)
			}

			hellos := srv.rec.all()
			if len(hellos) != 2 {
				t.Fatalf("server saw %d ClientHellos, want 2", len(hellos))
			}
			if hellos[0].has(extPreSharedKey) {
				t.Errorf("first ClientHello carries pre_shared_key: %v", hellos[0].extensions)
			}
			resumedExts := hellos[1].extensions
			if tc.tls12 {
				if hellos[1].has(extPreSharedKey) {
					t.Errorf("TLS 1.2 hello carries pre_shared_key: %v", resumedExts)
				}
			} else {
				if !hellos[1].has(extPreSharedKey) || hellos[1].last() != extPreSharedKey {
					t.Fatalf("second ClientHello: pre_shared_key must be present and last, got %v", resumedExts)
				}
				resumedExts = resumedExts[:len(resumedExts)-1]
			}

			// Apart from the trailing 41 the resumed hello is the first hello.
			if got, want := fmt.Sprint(resumedExts), fmt.Sprint(hellos[0].extensions); got != want {
				t.Errorf("resumed ClientHello changed more than extension 41:\n first  %s\n second %s", want, got)
			}
		})

		t.Run(tc.name+"/off", func(t *testing.T) {
			clearSessionCaches()
			srv := newHandshakeServer(t, pki, tc.h2, 0)
			browser := tc.browser
			browser.CACerts = []string{pki.caPEM}

			for i := 0; i < 2; i++ {
				if got, want := mustGet(t, browser, srv.URL), "resumed=false proto="+tc.proto; got != want {
					t.Errorf("request %d: got %q, want %q", i, got, want)
				}
			}
			for i, h := range srv.rec.all() {
				if h.has(extPreSharedKey) {
					t.Errorf("ClientHello %d carries pre_shared_key with sessionResumption off: %v", i, h.extensions)
				}
			}
		})
	}
}

// Turning the option on must not change the first ClientHello to a host.
func TestSessionResumptionKeepsFirstHello(t *testing.T) {
	pki := newTestPKI(t)
	for _, ua := range []string{hsTestChromeUA, hsTestFirefoxUA} {
		clearSessionCaches()
		srv := newHandshakeServer(t, pki, true, 0)
		base := Browser{UserAgent: ua, CACerts: []string{pki.caPEM}}
		mustGet(t, base, srv.URL)
		clearSessionCaches()
		withOption := base
		withOption.SessionResumption = true
		mustGet(t, withOption, srv.URL)

		hellos := srv.rec.all()
		if len(hellos) != 2 {
			t.Fatalf("server saw %d ClientHellos, want 2", len(hellos))
		}
		if a, b := fmt.Sprint(hellos[0].extensions), fmt.Sprint(hellos[1].extensions); a != b {
			t.Errorf("UA %q: first ClientHello differs with sessionResumption on:\n off %s\n on  %s", ua, a, b)
		}
	}
}

// A roundTripper built without createNewClient (NewTransport and friends) keeps
// its own cache and resumes on its next connection to the host.
func TestSessionResumptionPerRoundTripper(t *testing.T) {
	pki := newTestPKI(t)
	srv := newHandshakeServer(t, pki, true, 0)

	rt := newRoundTripper(Browser{
		UserAgent:         hsTestChromeUA,
		CACerts:           []string{pki.caPEM},
		SessionResumption: true,
	})
	client := &fhttp.Client{Transport: rt}
	t.Cleanup(func() { closeClientConnections(rt) })

	first, err := getWith(client, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	rt.(*roundTripper).CloseIdleConnections()
	second, err := getWith(client, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first, "resumed=false") || !strings.HasPrefix(second, "resumed=true") {
		t.Errorf("got %q then %q, want a full handshake then a resumed one", first, second)
	}
}

// Many connections share one session cache when requests run in parallel with
// connection reuse off. Run under -race: utls reads and replaces the cached
// session on every handshake.
func TestSessionResumptionConcurrent(t *testing.T) {
	pki := newTestPKI(t)
	clearSessionCaches()
	srv := newHandshakeServer(t, pki, true, 0)
	browser := Browser{UserAgent: hsTestChromeUA, CACerts: []string{pki.caPEM}, SessionResumption: true}
	mustGet(t, browser, srv.URL)

	const workers, perWorker = 8, 4
	var wg sync.WaitGroup
	errs := make(chan error, workers*perWorker)
	var mu sync.Mutex
	resumed := 0
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				body, err := getOnce(t, browser, srv.URL)
				if err != nil {
					errs <- err
					return
				}
				if strings.HasPrefix(body, "resumed=true") {
					mu.Lock()
					resumed++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if resumed == 0 {
		t.Errorf("none of the %d parallel requests resumed", workers*perWorker)
	}
}

// Sessions must not cross configurations that differ in route or fingerprint.
func TestSessionCacheKey(t *testing.T) {
	base := Browser{UserAgent: hsTestChromeUA, JA3: DefaultChrome_JA3, SessionResumption: true}
	key := sessionCacheKey(base, "")

	same := base
	same.Cookies = []Cookie{{Name: "a", Value: "b"}}
	same.HeaderOrder = []string{"accept"}
	same.HTTP2Fingerprint = "1:65536|1|0|m,a,s,p"
	if sessionCacheKey(same, "") != key {
		t.Error("cookies, header order and HTTP/2 fingerprint must not split the session cache")
	}

	differ := map[string]string{
		"proxy": sessionCacheKey(base, "http://127.0.0.1:8080"),
	}
	for name, mutate := range map[string]func(*Browser){
		"ja3":        func(b *Browser) { b.JA3 = "" },
		"userAgent":  func(b *Browser) { b.UserAgent = hsTestFirefoxUA },
		"serverName": func(b *Browser) { b.ServerName = testCustomSNI },
		"skipVerify": func(b *Browser) { b.InsecureSkipVerify = true },
		"certPins":   func(b *Browser) { b.CertPins = []string{strings.Repeat("ab", 32)} },
		"caCerts":    func(b *Browser) { b.CACerts = []string{"x"} },
	} {
		b := base
		mutate(&b)
		differ[name] = sessionCacheKey(b, "")
	}
	for name, k := range differ {
		if k == key {
			t.Errorf("changing %s must give a different session cache", name)
		}
	}

	if sharedSessionCache("k1") != sharedSessionCache("k1") {
		t.Error("the same key must return the same cache")
	}
	if sharedSessionCache("k1") == sharedSessionCache("k2") {
		t.Error("different keys must return different caches")
	}
	clearSessionCaches()
	for i := 0; i < maxSessionCaches+10; i++ {
		sharedSessionCache(fmt.Sprintf("bound-%d", i))
	}
	sessionCaches.Lock()
	n := len(sessionCaches.byKey)
	sessionCaches.Unlock()
	if n > maxSessionCaches {
		t.Errorf("registry holds %d caches, bound is %d", n, maxSessionCaches)
	}
	clearSessionCaches()
}

// The pooled-client key must separate clients that differ in these options.
func TestClientKeyCoversTLSHandshakeOptions(t *testing.T) {
	base := Browser{UserAgent: hsTestChromeUA}
	key := generateClientKey(base, 10, false, "")
	for name, mutate := range map[string]func(*Browser){
		"sessionResumption": func(b *Browser) { b.SessionResumption = true },
		"certPins":          func(b *Browser) { b.CertPins = []string{strings.Repeat("ab", 32)} },
		"caCerts":           func(b *Browser) { b.CACerts = []string{"x"} },
	} {
		b := base
		mutate(&b)
		if generateClientKey(b, 10, false, "") == key {
			t.Errorf("changing %s must change the client pool key", name)
		}
	}
}

// --- item 2: certificate pinning ------------------------------------------

func TestParseCertPin(t *testing.T) {
	sum := sha256.Sum256([]byte("anything"))
	b64 := base64.StdEncoding.EncodeToString(sum[:])
	hexPin := hex.EncodeToString(sum[:])

	valid := []struct {
		in   string
		spki bool
	}{
		{hexPin, false},
		{strings.ToUpper(hexPin), false},
		{"  " + hexPin + "\n", false},
		{"sha256/" + b64, true},
		{"SHA256/" + b64, true},
		{" sha256/" + b64 + " ", true},
	}
	for _, tc := range valid {
		pin, err := parseCertPin(tc.in)
		if err != nil {
			t.Errorf("parseCertPin(%q): %v", tc.in, err)
			continue
		}
		if pin.digest != sum || pin.spki != tc.spki {
			t.Errorf("parseCertPin(%q) = spki %t, digest match %t; want spki %t", tc.in, pin.spki, pin.digest == sum, tc.spki)
		}
	}

	for _, in := range []string{
		"",
		"sha256/",
		"not a pin",
		b64,                   // bare base64: the form does not say what was hashed
		"sha256/" + hexPin,    // hex after the SPKI prefix
		"sha256/" + b64[:20],  // too short
		"sha256/" + b64 + "A", // bad base64
		"sha1/" + b64,
		hexPin[:62],
		hexPin + "00",
		strings.Repeat("zz", 32),
		`pin-sha256="` + b64 + `"`,
	} {
		if _, err := parseCertPin(in); err == nil {
			t.Errorf("parseCertPin(%q): want an error", in)
		}
	}

	if _, err := parseCertPins([]string{hexPin, "bogus"}); err == nil || !strings.Contains(err.Error(), "certPins[1]") {
		t.Errorf("parseCertPins: want an error naming the bad entry, got %v", err)
	}
}

// The form of a pin fixes what it is compared with: a hex pin never matches a
// SubjectPublicKeyInfo hash and a sha256/ pin never matches a certificate hash.
func TestCertPinFormsDoNotCross(t *testing.T) {
	pki := newTestPKI(t)
	chain := []*x509.Certificate{pki.leafCert}
	spki := sha256.Sum256(pki.leafCert.RawSubjectPublicKeyInfo)
	whole := sha256.Sum256(pki.leafCert.Raw)

	for name, tc := range map[string]struct {
		pin  string
		want bool
	}{
		"hex_of_certificate":    {hex.EncodeToString(whole[:]), true},
		"sha256_of_spki":        {"sha256/" + base64.StdEncoding.EncodeToString(spki[:]), true},
		"hex_of_spki":           {hex.EncodeToString(spki[:]), false},
		"sha256_of_certificate": {"sha256/" + base64.StdEncoding.EncodeToString(whole[:]), false},
	} {
		pins, err := parseCertPins([]string{tc.pin})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		err = verifyCertPins(pins, "host", chain)
		if tc.want && err != nil {
			t.Errorf("%s: want a match, got %v", name, err)
		}
		if !tc.want && !errors.Is(err, ErrCertPinMismatch) {
			t.Errorf("%s: want ErrCertPinMismatch, got %v", name, err)
		}
	}
	if err := verifyCertPins([]certPin{{}}, "host", nil); !errors.Is(err, ErrCertPinMismatch) {
		t.Errorf("no certificates: want ErrCertPinMismatch, got %v", err)
	}
}

func TestCertPins(t *testing.T) {
	pki := newTestPKI(t)

	protocols := []struct {
		name       string
		h2         bool
		forceHTTP1 bool
		proto      string
	}{
		{name: "http1", forceHTTP1: true, proto: "HTTP/1.1"},
		{name: "http2", h2: true, proto: "HTTP/2.0"},
	}

	for _, p := range protocols {
		base := Browser{UserAgent: hsTestChromeUA, ForceHTTP1: p.forceHTTP1, CACerts: []string{pki.caPEM}}

		accepted := map[string][]string{
			"spki_sha256_base64":   {spkiPin(pki.leafCert)},
			"certificate_hex":      {certFingerprint(pki.leafCert)},
			"issuer_spki_in_chain": {spkiPin(pki.caCert)},
			"issuer_hex_in_chain":  {certFingerprint(pki.caCert)},
			"one_of_several":       {wrongPins[0], wrongPins[1], spkiPin(pki.leafCert)},
		}
		for name, pins := range accepted {
			t.Run(p.name+"/pass/"+name, func(t *testing.T) {
				srv := newHandshakeServer(t, pki, p.h2, 0)
				browser := base
				browser.CertPins = pins
				if got, want := mustGet(t, browser, srv.URL), "resumed=false proto="+p.proto; got != want {
					t.Errorf("got %q, want %q", got, want)
				}
			})
		}

		t.Run(p.name+"/wrong_pin", func(t *testing.T) {
			srv := newHandshakeServer(t, pki, p.h2, 0)
			browser := base
			browser.CertPins = wrongPins
			_, err := getOnce(t, browser, srv.URL)
			if !errors.Is(err, ErrCertPinMismatch) {
				t.Fatalf("got error %v, want ErrCertPinMismatch", err)
			}
			if !strings.Contains(err.Error(), spkiPin(pki.leafCert)) || !strings.Contains(err.Error(), certFingerprint(pki.leafCert)) {
				t.Errorf("error should name both pins of the leaf the server presented: %v", err)
			}
			if n := srv.requestCount(); n != 0 {
				t.Errorf("server handled %d request(s) after a pin mismatch", n)
			}
		})

		// insecureSkipVerify turns off chain verification, not the pins.
		t.Run(p.name+"/wrong_pin_insecure_skip_verify", func(t *testing.T) {
			srv := newHandshakeServer(t, pki, p.h2, 0)
			browser := Browser{UserAgent: hsTestChromeUA, ForceHTTP1: p.forceHTTP1, InsecureSkipVerify: true, CertPins: wrongPins}
			if _, err := getOnce(t, browser, srv.URL); !errors.Is(err, ErrCertPinMismatch) {
				t.Fatalf("got error %v, want ErrCertPinMismatch", err)
			}
			browser.CertPins = []string{spkiPin(pki.leafCert)}
			mustGet(t, browser, srv.URL)
		})

		// A resumed session has no certificate message; the pins are checked
		// against the chain kept with the session.
		t.Run(p.name+"/resumed", func(t *testing.T) {
			clearSessionCaches()
			srv := newHandshakeServer(t, pki, p.h2, 0)
			browser := base
			browser.SessionResumption = true
			browser.CertPins = []string{spkiPin(pki.leafCert)}
			mustGet(t, browser, srv.URL)
			if got := mustGet(t, browser, srv.URL); !strings.HasPrefix(got, "resumed=true") {
				t.Errorf("second request: got %q, want a resumed session", got)
			}
		})
	}
}

// rawTLSServer finishes the handshake and then counts the application bytes
// the client sends. It never answers.
type rawTLSServer struct {
	addr string
	mu   sync.Mutex
	// handshakes that finished, and the application bytes read after them
	handshakes int
	appBytes   int
	done       chan struct{}
}

func newRawTLSServer(t *testing.T, pki *testPKI, alpn []string) *rawTLSServer {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{pki.serverCert},
		NextProtos:   alpn,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := &rawTLSServer{addr: ln.Addr().String(), done: make(chan struct{}, 8)}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { s.done <- struct{}{} }()
				defer c.Close()
				tc := c.(*tls.Conn)
				_ = tc.SetDeadline(time.Now().Add(5 * time.Second))
				if err := tc.Handshake(); err != nil {
					return
				}
				s.mu.Lock()
				s.handshakes++
				s.mu.Unlock()
				buf := make([]byte, 4096)
				for {
					n, err := tc.Read(buf)
					s.mu.Lock()
					s.appBytes += n
					s.mu.Unlock()
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()
	return s
}

// On a pin mismatch the connection is closed before the request (HTTP/1.1) or
// the connection preface (HTTP/2) is written.
func TestCertPinMismatchSendsNoRequestBytes(t *testing.T) {
	pki := newTestPKI(t)

	for _, tc := range []struct {
		name string
		alpn []string
	}{
		{name: "http1", alpn: []string{"http/1.1"}},
		{name: "http2", alpn: []string{"h2", "http/1.1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newRawTLSServer(t, pki, tc.alpn)
			browser := Browser{UserAgent: hsTestChromeUA, CACerts: []string{pki.caPEM}, CertPins: wrongPins}

			_, err := getOnce(t, browser, "https://"+srv.addr+"/secret-path")
			if !errors.Is(err, ErrCertPinMismatch) {
				t.Fatalf("got error %v, want ErrCertPinMismatch", err)
			}

			select {
			case <-srv.done:
			case <-time.After(6 * time.Second):
				t.Fatal("the client did not close the connection after the pin mismatch")
			}
			srv.mu.Lock()
			defer srv.mu.Unlock()
			if srv.handshakes != 1 {
				t.Errorf("server finished %d handshakes, want 1", srv.handshakes)
			}
			if srv.appBytes != 0 {
				t.Errorf("client sent %d application bytes after a pin mismatch, want 0", srv.appBytes)
			}
		})
	}
}

func TestCertPinsRejectedWhereTheyCannotBeChecked(t *testing.T) {
	pki := newTestPKI(t)
	pin := spkiPin(pki.leafCert)

	t.Run("invalid_pin", func(t *testing.T) {
		srv := newHandshakeServer(t, pki, true, 0)
		_, err := getOnce(t, Browser{CACerts: []string{pki.caPEM}, CertPins: []string{pin, "not-a-pin"}}, srv.URL)
		if err == nil || !strings.Contains(err.Error(), "invalid certPins[1]") {
			t.Fatalf("got error %v, want an invalid certPins error", err)
		}
		if n := len(srv.rec.all()); n != 0 {
			t.Errorf("server saw %d ClientHello(s); an invalid pin must fail before dialing", n)
		}
	})

	t.Run("http3", func(t *testing.T) {
		_, err := getOnce(t, Browser{ForceHTTP3: true, CertPins: []string{pin}}, "https://127.0.0.1:1/")
		if !errors.Is(err, errCertPinsHTTP3) {
			t.Fatalf("got error %v, want errCertPinsHTTP3", err)
		}
	})

	t.Run("websocket", func(t *testing.T) {
		res := dispatchWebSocketRequest(cycleTLSRequest{RequestID: "ws-pins", Options: Options{
			URL:      "wss://127.0.0.1:1/",
			Protocol: "websocket",
			CertPins: []string{pin},
		}})
		if !errors.Is(res.err, errCertPinsWebSocket) || res.wsClient != nil {
			t.Fatalf("got err %v wsClient %v, want errCertPinsWebSocket and no client", res.err, res.wsClient)
		}
	})

	t.Run("plain_http", func(t *testing.T) {
		hits := 0
		srv := httptest.NewServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) { hits++ }))
		defer srv.Close()
		_, err := getOnce(t, Browser{CertPins: []string{pin}}, srv.URL)
		if !errors.Is(err, errCertPinsPlainHTTP) {
			t.Fatalf("got error %v, want errCertPinsPlainHTTP", err)
		}
		if hits != 0 {
			t.Errorf("plain-http server handled %d request(s) with certPins set", hits)
		}
	})
}

// --- item 3: custom root CAs ----------------------------------------------

func TestCACerts(t *testing.T) {
	pki := newTestPKI(t)
	other := newTestPKI(t)

	for _, h2 := range []bool{false, true} {
		name := "http1"
		if h2 {
			name = "http2"
		}
		t.Run(name, func(t *testing.T) {
			srv := newHandshakeServer(t, pki, h2, 0)
			base := Browser{UserAgent: hsTestChromeUA, ForceHTTP1: !h2}

			_, err := getOnce(t, base, srv.URL)
			if err == nil || !strings.Contains(err.Error(), "certificate") {
				t.Fatalf("without caCerts: got error %v, want a certificate verification error", err)
			}
			if n := srv.requestCount(); n != 0 {
				t.Fatalf("server handled %d request(s) from a client that does not trust its CA", n)
			}

			wrongCA := base
			wrongCA.CACerts = []string{other.caPEM}
			if _, err := getOnce(t, wrongCA, srv.URL); err == nil {
				t.Fatal("a different private CA must not make the server trusted")
			}

			withCA := base
			withCA.CACerts = []string{other.caPEM + pki.caPEM}
			mustGet(t, withCA, srv.URL)
			if n := srv.requestCount(); n != 1 {
				t.Errorf("server handled %d request(s), want 1", n)
			}
		})
	}

	t.Run("invalid_pem", func(t *testing.T) {
		srv := newHandshakeServer(t, pki, true, 0)
		keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1, 2, 3}}))
		badCert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{1, 2, 3}}))
		for label, ca := range map[string]string{
			"not_pem":         "this is not a certificate",
			"empty":           "",
			"wrong_block":     keyPEM,
			"corrupt_der":     badCert,
			"trailing_text":   pki.caPEM + "garbage",
			"good_then_wrong": pki.caPEM + keyPEM,
		} {
			_, err := getOnce(t, Browser{CACerts: []string{pki.caPEM, ca}}, srv.URL)
			if err == nil || !strings.Contains(err.Error(), "invalid caCerts[1]") {
				t.Errorf("%s: got error %v, want an invalid caCerts error", label, err)
			}
		}
		if n := len(srv.rec.all()); n != 0 {
			t.Errorf("server saw %d ClientHello(s); invalid caCerts must fail before dialing", n)
		}
	})

	// The system roots stay in the pool.
	t.Run("system_roots_kept", func(t *testing.T) {
		pool, err := buildRootCAs([]string{pki.caPEM})
		if err != nil {
			t.Fatal(err)
		}
		system, err := x509.SystemCertPool()
		if err != nil || system == nil {
			t.Skip("no system cert pool on this host")
		}
		if pool.Equal(x509.NewCertPool()) {
			t.Error("pool is empty")
		}
		only := x509.NewCertPool()
		only.AddCert(pki.caCert)
		if !system.Equal(x509.NewCertPool()) && pool.Equal(only) {
			t.Error("pool holds only the custom CA; the system roots were dropped")
		}
	})
}

// --- item 4: custom SNI on the retry dials --------------------------------

func sniList(hellos []seenHello) []string {
	out := make([]string, 0, len(hellos))
	for _, h := range hellos {
		out = append(out, h.sni)
	}
	return out
}

// A "771," JA3 with tls13AutoRetry is first sent upgraded to TLS 1.3. When
// that handshake fails, retryWithOriginalTLS12JA3 dials again with the
// original JA3. The server refuses the first handshake to force that retry.
func TestCustomSNIOnTLS12FallbackRetry(t *testing.T) {
	pki := newTestPKI(t)
	srv := newHandshakeServer(t, pki, true, 1)

	browser := Browser{
		UserAgent:      hsTestChromeUA,
		JA3:            DefaultChrome_JA3,
		TLS13AutoRetry: true,
		ServerName:     testCustomSNI,
		CACerts:        []string{pki.caPEM},
	}
	mustGet(t, browser, srv.URL)

	got := sniList(srv.rec.all())
	if want := []string{testCustomSNI, testCustomSNI}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("SNI seen by the server on first and retry dial: got %q, want %q", got, want)
	}
}

// curveErrorDialer makes the first dial fail its handshake with the error text
// dialTLS matches to start retryWithTLS13CompatibleCurves. utls raises that
// error before it writes anything, for a spec whose key shares it cannot
// generate; no JA3 or JA4_r string reaches that state at utls v1.8.2, so the
// test injects the error at the first write instead.
type curveErrorDialer struct {
	mu    sync.Mutex
	dials int
}

type failingWriteConn struct{ net.Conn }

func (c failingWriteConn) Write([]byte) (int, error) {
	return 0, errors.New("tls: CurvePreferences includes unsupported curve")
}

func (d *curveErrorDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dials++
	if d.dials == 1 {
		return failingWriteConn{conn}, nil
	}
	return conn, nil
}

func (d *curveErrorDialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

// The other retry: the first handshake fails with the unsupported-curve error
// and retryWithTLS13CompatibleCurves dials again.
func TestCustomSNIOnCurveRetry(t *testing.T) {
	pki := newTestPKI(t)

	for _, tc := range []struct {
		name    string
		pins    []string
		wantErr error
	}{
		{name: "sni"},
		{name: "matching_pin", pins: []string{spkiPin(pki.leafCert)}},
		{name: "wrong_pin", pins: wrongPins, wantErr: ErrCertPinMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newHandshakeServer(t, pki, true, 0)
			dialer := &curveErrorDialer{}
			rt := newRoundTripper(Browser{
				UserAgent:      hsTestChromeUA,
				TLS13AutoRetry: true,
				ServerName:     testCustomSNI,
				CACerts:        []string{pki.caPEM},
				CertPins:       tc.pins,
			}, dialer)
			t.Cleanup(func() { closeClientConnections(rt) })

			_, err := getWith(&fhttp.Client{Transport: rt}, srv.URL)
			if tc.wantErr == nil && err != nil {
				t.Fatal(err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("got error %v, want %v", err, tc.wantErr)
			}
			if dialer.dials != 2 {
				t.Fatalf("dialed %d times, want 2 (first dial and the curve retry)", dialer.dials)
			}
			// The first dial never got a ClientHello out, so the server saw the retry only.
			got := sniList(srv.rec.all())
			if want := []string{testCustomSNI}; fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("SNI seen by the server on the retry dial: got %q, want %q", got, want)
			}
			if tc.wantErr != nil && srv.requestCount() != 0 {
				t.Errorf("server handled %d request(s) after a pin mismatch on the retry dial", srv.requestCount())
			}
		})
	}
}

// The retry dial must be checked against the pins like the first dial.
func TestCertPinsOnRetryDial(t *testing.T) {
	pki := newTestPKI(t)

	for _, tc := range []struct {
		name string
		h2   bool
	}{{"http1", false}, {"http2", true}} {
		t.Run(tc.name, func(t *testing.T) {
			browser := Browser{
				UserAgent:      hsTestChromeUA,
				JA3:            DefaultChrome_JA3,
				TLS13AutoRetry: true,
				ForceHTTP1:     !tc.h2,
				CACerts:        []string{pki.caPEM},
			}

			srv := newHandshakeServer(t, pki, tc.h2, 1)
			browser.CertPins = wrongPins
			_, err := getOnce(t, browser, srv.URL)
			if !errors.Is(err, ErrCertPinMismatch) {
				t.Fatalf("got error %v, want ErrCertPinMismatch", err)
			}
			if n := len(srv.rec.all()); n != 2 {
				t.Fatalf("server saw %d ClientHellos, want 2 (first dial and retry)", n)
			}
			if n := srv.requestCount(); n != 0 {
				t.Errorf("server handled %d request(s) after a pin mismatch on the retry dial", n)
			}

			srv = newHandshakeServer(t, pki, tc.h2, 1)
			browser.CertPins = []string{spkiPin(pki.leafCert)}
			mustGet(t, browser, srv.URL)
		})
	}
}

// --- item 5: Firefox default path and forceHTTP1 --------------------------

func TestFirefoxDefaultHonoursForceHTTP1(t *testing.T) {
	pki := newTestPKI(t)

	srv := newHandshakeServer(t, pki, true, 0)
	forced := Browser{UserAgent: hsTestFirefoxUA, ForceHTTP1: true, CACerts: []string{pki.caPEM}}
	if got, want := mustGet(t, forced, srv.URL), "resumed=false proto=HTTP/1.1"; got != want {
		t.Errorf("forceHTTP1 with a Firefox UA: got %q, want %q", got, want)
	}
	hellos := srv.rec.all()
	if len(hellos) != 1 || fmt.Sprint(hellos[0].alpn) != "[http/1.1]" {
		t.Errorf("forceHTTP1 with a Firefox UA: ClientHello ALPN = %v, want [http/1.1]", hellos)
	}

	// Not forced: the Firefox hello is unchanged and negotiates h2.
	srv = newHandshakeServer(t, pki, true, 0)
	plain := Browser{UserAgent: hsTestFirefoxUA, CACerts: []string{pki.caPEM}}
	if got, want := mustGet(t, plain, srv.URL), "resumed=false proto=HTTP/2.0"; got != want {
		t.Errorf("Firefox UA without forceHTTP1: got %q, want %q", got, want)
	}
	hellos = srv.rec.all()
	if len(hellos) != 1 || fmt.Sprint(hellos[0].alpn) != "[h2 http/1.1]" {
		t.Errorf("Firefox UA without forceHTTP1: ClientHello ALPN = %v, want [h2 http/1.1]", hellos)
	}
}

// --- options plumbing -----------------------------------------------------

func TestTLSHandshakeOptionsFromJSON(t *testing.T) {
	var opts Options
	raw := `{"url":"https://example.test","sessionResumption":true,"certPins":["sha256/AAAA"],"caCerts":["PEM"]}`
	if err := json.Unmarshal([]byte(raw), &opts); err != nil {
		t.Fatal(err)
	}
	b := browserFromOptions(opts)
	if !b.SessionResumption || fmt.Sprint(b.CertPins) != "[sha256/AAAA]" || fmt.Sprint(b.CACerts) != "[PEM]" {
		t.Errorf("options not carried to Browser: %+v", b)
	}

	var defaults Options
	if err := json.Unmarshal([]byte(`{"url":"https://example.test"}`), &defaults); err != nil {
		t.Fatal(err)
	}
	if d := browserFromOptions(defaults); d.SessionResumption || d.CertPins != nil || d.CACerts != nil {
		t.Errorf("defaults must leave the options off: %+v", d)
	}
}

// CycleTLS.Do builds its Browser by hand; it must carry the options too.
func TestDoCarriesTLSHandshakeOptions(t *testing.T) {
	pki := newTestPKI(t)
	clearSessionCaches()
	srv := newHandshakeServer(t, pki, true, 0)
	client := CycleTLS{}
	opts := Options{
		UserAgent:         hsTestChromeUA,
		CACerts:           []string{pki.caPEM},
		CertPins:          []string{spkiPin(pki.leafCert)},
		SessionResumption: true,
	}

	var bodies []string
	for i := 0; i < 2; i++ {
		resp, err := client.Do(srv.URL, opts, "GET")
		if err != nil {
			t.Fatal(err)
		}
		if resp.Status != 200 {
			t.Fatalf("request %d: status %d body %q", i, resp.Status, resp.Body)
		}
		bodies = append(bodies, resp.Body)
	}
	if !strings.HasPrefix(bodies[0], "resumed=false") || !strings.HasPrefix(bodies[1], "resumed=true") {
		t.Errorf("got %q, want a full handshake then a resumed one", bodies)
	}

	opts.CertPins = wrongPins
	resp, err := client.Do(srv.URL, opts, "GET")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status == 200 || !strings.Contains(resp.Body, ErrCertPinMismatch.Error()) {
		t.Errorf("wrong pin through Do: status %d body %q, want a pin mismatch", resp.Status, resp.Body)
	}
}
