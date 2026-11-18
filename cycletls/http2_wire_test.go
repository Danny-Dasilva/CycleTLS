package cycletls

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	xhttp2 "golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// These tests read the HTTP/2 frames CycleTLS writes with a local TLS listener
// and a raw framer, so they check wire bytes, not parsed structs. No network.

const (
	testChromeUA  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/154.0.0.0 Safari/537.36"
	testFirefoxUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0"

	// The example from the README ("Firefox" HTTP/2 fingerprint).
	readmeFirefoxH2 = "1:65536;4:131072;5:16384|12517377|3:0:0:201,5:0:0:101,7:0:0:1,9:0:7:1,11:0:3:1,13:0:0:241|m,p,a,s"
	// Chrome 154 as tls-client and tls.peet.ws print it.
	chromeStyleH2 = "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p"
)

type h2Priority struct {
	StreamID  uint32
	Exclusive bool
	StreamDep uint32
	Weight    uint8 // wire byte: RFC 7540 weight minus one
}

type h2Capture struct {
	Settings        []string // "id:value" in wire order
	WindowUpdate    uint32   // connection-level increment sent before HEADERS
	Priorities      []h2Priority
	HeadersStreamID uint32
	HeadersPriority *h2Priority // nil when HEADERS carries no priority
	PseudoOrder     []string
	Headers         []string // regular header names in wire order
}

func (c h2Capture) settingsString() string { return strings.Join(c.Settings, ",") }

// newLocalTestCert returns a self-signed certificate for 127.0.0.1/localhost.
func newLocalTestCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "cycletls-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// serveOneH2 reads one connection's frames up to the first HEADERS frame,
// answers 200 and returns what it saw.
func serveOneH2(conn net.Conn) (h2Capture, error) {
	var capture h2Capture
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	preface := make([]byte, len(xhttp2.ClientPreface))
	if _, err := io.ReadFull(conn, preface); err != nil {
		return capture, fmt.Errorf("read preface: %w", err)
	}
	if !bytes.Equal(preface, []byte(xhttp2.ClientPreface)) {
		return capture, fmt.Errorf("bad client preface %q", preface)
	}

	fr := xhttp2.NewFramer(conn, conn)
	if err := fr.WriteSettings(); err != nil {
		return capture, err
	}

	for {
		frame, err := fr.ReadFrame()
		if err != nil {
			return capture, fmt.Errorf("read frame: %w", err)
		}
		switch f := frame.(type) {
		case *xhttp2.SettingsFrame:
			if f.IsAck() {
				continue
			}
			_ = f.ForeachSetting(func(s xhttp2.Setting) error {
				capture.Settings = append(capture.Settings, fmt.Sprintf("%d:%d", uint16(s.ID), s.Val))
				return nil
			})
			if err := fr.WriteSettingsAck(); err != nil {
				return capture, err
			}
		case *xhttp2.WindowUpdateFrame:
			if f.StreamID == 0 {
				capture.WindowUpdate = f.Increment
			}
		case *xhttp2.PriorityFrame:
			capture.Priorities = append(capture.Priorities, h2Priority{
				StreamID:  f.StreamID,
				Exclusive: f.Exclusive,
				StreamDep: f.StreamDep,
				Weight:    f.Weight,
			})
		case *xhttp2.HeadersFrame:
			capture.HeadersStreamID = f.StreamID
			if f.HasPriority() {
				capture.HeadersPriority = &h2Priority{
					StreamID:  f.StreamID,
					Exclusive: f.Priority.Exclusive,
					StreamDep: f.Priority.StreamDep,
					Weight:    f.Priority.Weight,
				}
			}
			fields, err := hpack.NewDecoder(4096, nil).DecodeFull(f.HeaderBlockFragment())
			if err != nil {
				return capture, fmt.Errorf("decode headers: %w", err)
			}
			for _, field := range fields {
				if strings.HasPrefix(field.Name, ":") {
					capture.PseudoOrder = append(capture.PseudoOrder, field.Name)
				} else {
					capture.Headers = append(capture.Headers, field.Name)
				}
			}
			// 0x88 is the HPACK static-table entry ":status: 200".
			err = fr.WriteHeaders(xhttp2.HeadersFrameParam{
				StreamID:      f.StreamID,
				BlockFragment: []byte{0x88},
				EndStream:     true,
				EndHeaders:    true,
			})
			return capture, err
		}
	}
}

// captureHTTP2 sends one GET through Do and returns the frames the server read.
func captureHTTP2(t *testing.T, options Options) h2Capture {
	t.Helper()

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{newLocalTestCert(t)},
		NextProtos:   []string{"h2"},
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	type result struct {
		capture h2Capture
		err     error
	}
	results := make(chan result, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			results <- result{err: err}
			return
		}
		defer conn.Close()
		capture, err := serveOneH2(conn)
		results <- result{capture, err}
		// Drain until the client hangs up so its writes never block.
		_, _ = io.Copy(io.Discard, conn)
	}()

	options.InsecureSkipVerify = true
	client := Init()
	defer client.Close()

	resp, err := client.Do("https://"+listener.Addr().String()+"/", options, "GET")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	select {
	case r := <-results:
		if r.err != nil {
			t.Fatalf("server: %v (client status %d, body %q)", r.err, resp.Status, resp.Body)
		}
		if resp.Status != 200 {
			t.Fatalf("status = %d, want 200, body %q", resp.Status, resp.Body)
		}
		return r.capture
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for the HTTP/2 server (client status %d, body %q)", resp.Status, resp.Body)
	}
	return h2Capture{}
}

func assertPriorities(t *testing.T, got, want []h2Priority) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PRIORITY frames = %+v, want %+v", got, want)
	}
}

func assertHeadersPriority(t *testing.T, got *h2Priority, want *h2Priority) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Errorf("HEADERS priority = %+v, want %+v", got, want)
	case got.Exclusive != want.Exclusive || got.StreamDep != want.StreamDep || got.Weight != want.Weight:
		t.Errorf("HEADERS priority = %+v, want %+v", *got, *want)
	}
}

var (
	pseudoChrome  = []string{":method", ":authority", ":scheme", ":path"}
	pseudoFirefox = []string{":method", ":path", ":authority", ":scheme"}
	pseudoSafari  = []string{":method", ":scheme", ":authority", ":path"}

	firefoxLegacyPriorities = []h2Priority{
		{StreamID: 3, Weight: 200},
		{StreamID: 5, Weight: 100},
		{StreamID: 7, Weight: 0},
		{StreamID: 9, StreamDep: 7, Weight: 0},
		{StreamID: 11, StreamDep: 3, Weight: 0},
		{StreamID: 13, Weight: 240},
	}
)

// The README's own example must produce the frames it describes, whatever the
// User-Agent says.
func TestHTTP2FingerprintREADMEExampleOnWire(t *testing.T) {
	for _, ua := range []string{testChromeUA, testFirefoxUA} {
		t.Run(parseUserAgent(ua).UserAgent, func(t *testing.T) {
			got := captureHTTP2(t, Options{HTTP2Fingerprint: readmeFirefoxH2, UserAgent: ua})

			if s := got.settingsString(); s != "1:65536,4:131072,5:16384" {
				t.Errorf("SETTINGS = %s", s)
			}
			if got.WindowUpdate != 12517377 {
				t.Errorf("WINDOW_UPDATE = %d, want 12517377", got.WindowUpdate)
			}
			assertPriorities(t, got.Priorities, firefoxLegacyPriorities)
			if got.HeadersStreamID != 15 {
				t.Errorf("first request stream = %d, want 15", got.HeadersStreamID)
			}
			if !reflect.DeepEqual(got.PseudoOrder, pseudoFirefox) {
				t.Errorf("pseudo-header order = %v, want %v", got.PseudoOrder, pseudoFirefox)
			}
			// A PRIORITY list says nothing about the HEADERS frame: default
			// weight 256, not exclusive.
			assertHeadersPriority(t, got.HeadersPriority, &h2Priority{Weight: 255})
		})
	}
}

// A Chrome-style string (part 3 "0") sends no PRIORITY frames and opens stream
// 1. With a Firefox User-Agent fhttp used to add six legacy PRIORITY frames.
func TestHTTP2FingerprintChromeStyleOnWire(t *testing.T) {
	for _, ua := range []string{testChromeUA, testFirefoxUA} {
		t.Run(parseUserAgent(ua).UserAgent, func(t *testing.T) {
			got := captureHTTP2(t, Options{HTTP2Fingerprint: chromeStyleH2, UserAgent: ua})

			if s := got.settingsString(); s != "1:65536,2:0,4:6291456,6:262144" {
				t.Errorf("SETTINGS = %s", s)
			}
			if got.WindowUpdate != 15663105 {
				t.Errorf("WINDOW_UPDATE = %d, want 15663105", got.WindowUpdate)
			}
			assertPriorities(t, got.Priorities, nil)
			if got.HeadersStreamID != 1 {
				t.Errorf("first request stream = %d, want 1", got.HeadersStreamID)
			}
			if !reflect.DeepEqual(got.PseudoOrder, pseudoChrome) {
				t.Errorf("pseudo-header order = %v, want %v", got.PseudoOrder, pseudoChrome)
			}
			assertHeadersPriority(t, got.HeadersPriority, &h2Priority{Weight: 255})
		})
	}
}

// Part 4 decides the pseudo-header order even when it disagrees with the
// User-Agent, and "," works as the SETTINGS separator too.
func TestHTTP2FingerprintPseudoHeaderOrderOnWire(t *testing.T) {
	got := captureHTTP2(t, Options{
		HTTP2Fingerprint: "2:0,3:100,4:2097152,9:1|10420225|0|m,s,a,p",
		UserAgent:        testChromeUA,
	})
	if s := got.settingsString(); s != "2:0,3:100,4:2097152,9:1" {
		t.Errorf("SETTINGS = %s", s)
	}
	if got.WindowUpdate != 10420225 {
		t.Errorf("WINDOW_UPDATE = %d, want 10420225", got.WindowUpdate)
	}
	if !reflect.DeepEqual(got.PseudoOrder, pseudoSafari) {
		t.Errorf("pseudo-header order = %v, want %v", got.PseudoOrder, pseudoSafari)
	}
}

// 3.0.0 strings with a bare "1" keep their meaning: exclusive HEADERS priority,
// and the User-Agent's PRIORITY frames (none for Chrome, six for Firefox).
func TestHTTP2FingerprintLegacyExclusiveFlagOnWire(t *testing.T) {
	const fp = "1:65536,2:0,4:6291456,6:262144|15663105|1|m,a,s,p"

	chromeGot := captureHTTP2(t, Options{HTTP2Fingerprint: fp, UserAgent: testChromeUA})
	assertPriorities(t, chromeGot.Priorities, nil)
	assertHeadersPriority(t, chromeGot.HeadersPriority, &h2Priority{Exclusive: true, Weight: 255})
	if chromeGot.HeadersStreamID != 1 {
		t.Errorf("chrome: first request stream = %d, want 1", chromeGot.HeadersStreamID)
	}

	firefoxGot := captureHTTP2(t, Options{HTTP2Fingerprint: fp, UserAgent: testFirefoxUA})
	assertPriorities(t, firefoxGot.Priorities, firefoxLegacyPriorities)
	assertHeadersPriority(t, firefoxGot.HeadersPriority, &h2Priority{Exclusive: true, Weight: 255})
	if firefoxGot.HeadersStreamID != 15 {
		t.Errorf("firefox: first request stream = %d, want 15", firefoxGot.HeadersStreamID)
	}
}

// Without http2Fingerprint nothing changes: the User-Agent picks fhttp's preset.
func TestHTTP2DefaultsUnchangedOnWire(t *testing.T) {
	chromeGot := captureHTTP2(t, Options{UserAgent: testChromeUA})
	if s := chromeGot.settingsString(); s != "1:65536,3:1000,4:6291456,5:16384,6:262144" {
		t.Errorf("chrome SETTINGS = %s", s)
	}
	assertPriorities(t, chromeGot.Priorities, nil)
	assertHeadersPriority(t, chromeGot.HeadersPriority, nil)
	if !reflect.DeepEqual(chromeGot.PseudoOrder, pseudoChrome) {
		t.Errorf("chrome pseudo-header order = %v", chromeGot.PseudoOrder)
	}

	firefoxGot := captureHTTP2(t, Options{UserAgent: testFirefoxUA})
	if s := firefoxGot.settingsString(); s != "1:65536,4:131072,5:16384" {
		t.Errorf("firefox SETTINGS = %s", s)
	}
	assertPriorities(t, firefoxGot.Priorities, firefoxLegacyPriorities)
	assertHeadersPriority(t, firefoxGot.HeadersPriority, nil)
	if !reflect.DeepEqual(firefoxGot.PseudoOrder, pseudoFirefox) {
		t.Errorf("firefox pseudo-header order = %v", firefoxGot.PseudoOrder)
	}
}
