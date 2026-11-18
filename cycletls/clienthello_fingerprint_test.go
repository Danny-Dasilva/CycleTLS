package cycletls

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"testing"

	utls "github.com/refraction-networking/utls"
)

const specTestServerName = "example.com"

// buildClientHello applies spec the way roundTripper.dialTLS does and returns
// the ClientHello handshake message utls would send. Nothing is written to the
// network: the conn is one end of an in-memory pipe.
func buildClientHello(t *testing.T, spec *utls.ClientHelloSpec) []byte {
	t.Helper()
	raw, err := buildClientHelloErr(spec, utls.HelloCustom)
	if err != nil {
		t.Fatalf("build ClientHello: %v", err)
	}
	return raw
}

func buildClientHelloErr(spec *utls.ClientHelloSpec, id utls.ClientHelloID) ([]byte, error) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	uc := utls.UClient(c1, &utls.Config{
		ServerName:         specTestServerName,
		OmitEmptyPsk:       true,
		InsecureSkipVerify: true,
	}, id)
	if spec != nil {
		if err := uc.ApplyPreset(spec); err != nil {
			return nil, fmt.Errorf("ApplyPreset: %w", err)
		}
	}
	if err := uc.BuildHandshakeState(); err != nil {
		return nil, fmt.Errorf("BuildHandshakeState: %w", err)
	}
	return append([]byte(nil), uc.HandshakeState.Hello.Raw...), nil
}

func mustParseHello(t *testing.T, raw []byte) *ClientHelloInfo {
	t.Helper()
	info, err := ParseClientHello(raw)
	if err != nil {
		t.Fatalf("ParseClientHello: %v", err)
	}
	return info
}

// The JA4 and JA4_r pair below is the Chrome example from the FoxIO JA4
// specification (technical_details/JA4.md).
const (
	foxioJA4R = "t13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0005,000a,000b,000d,0012,0015,0017,001b,0023,002b,002d,0033,4469,ff01_0403,0804,0401,0503,0805,0501,0806,0601"
	foxioJA4  = "t13d1516h2_8daaf6152771_e5627efa2ab1"
)

// foxioHello is a ClientHello with the fields of the FoxIO example, in an
// unsorted wire order and with GREASE values that JA4 must ignore.
func foxioHello() *ClientHelloInfo {
	return &ClientHelloInfo{
		Version: 0x0303,
		CipherSuites: []uint16{
			0x0a0a, 0x1301, 0x1302, 0x1303, 0xc02b, 0xc02f, 0xc02c, 0xc030,
			0xcca9, 0xcca8, 0xc013, 0xc014, 0x009c, 0x009d, 0x002f, 0x0035,
		},
		Extensions: []uint16{
			0x1a1a, 0x0000, 0x0017, 0xff01, 0x000a, 0x000b, 0x0023, 0x0010, 0x0005,
			0x000d, 0x0012, 0x0033, 0x002d, 0x002b, 0x001b, 0x4469, 0x2a2a, 0x0015,
		},
		SignatureAlgorithms: []uint16{0x0403, 0x0804, 0x0401, 0x0503, 0x0805, 0x0501, 0x0806, 0x0601},
		SupportedVersions:   []uint16{0x3a3a, 0x0304, 0x0303},
		ALPN:                []string{"h2", "http/1.1"},
		HasSNI:              true,
	}
}

func TestJA4MatchesFoxIOExample(t *testing.T) {
	info := foxioHello()
	if got := info.JA4(); got != foxioJA4 {
		t.Errorf("JA4 = %q, want %q", got, foxioJA4)
	}
	if got := info.JA4R(); got != foxioJA4R {
		t.Errorf("JA4_r = %q, want %q", got, foxioJA4R)
	}
}

func TestJA4PrefixRules(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ClientHelloInfo)
		want   string
	}{
		{"no SNI", func(c *ClientHelloInfo) {
			c.HasSNI = false
			c.Extensions = c.Extensions[2:] // drop GREASE and server_name
		}, "t13i1515h2"},
		{"no ALPN", func(c *ClientHelloInfo) { c.ALPN = nil }, "t13d151600"},
		{"http/1.1 first", func(c *ClientHelloInfo) { c.ALPN = []string{"http/1.1"} }, "t13d1516h1"},
		{"non-alphanumeric ALPN uses hex", func(c *ClientHelloInfo) { c.ALPN = []string{"\xab\xcd"} }, "t13d1516ad"},
		{"no supported_versions falls back to legacy version", func(c *ClientHelloInfo) { c.SupportedVersions = nil }, "t12d1516h2"},
		{"TLS 1.0", func(c *ClientHelloInfo) { c.SupportedVersions = nil; c.Version = 0x0301 }, "t10d1516h2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := foxioHello()
			tt.mutate(info)
			if got := strings.SplitN(info.JA4(), "_", 2)[0]; got != tt.want {
				t.Errorf("JA4 prefix = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("no signature algorithms leaves out the second list", func(t *testing.T) {
		info := foxioHello()
		info.SignatureAlgorithms = nil
		if r := info.JA4R(); strings.Count(r, "_") != 2 {
			t.Errorf("JA4_r = %q, want exactly three sections", r)
		}
	})
	t.Run("empty lists hash to zeros", func(t *testing.T) {
		info := &ClientHelloInfo{Version: 0x0303}
		if got, want := info.JA4(), "t12i000000_000000000000_000000000000"; got != want {
			t.Errorf("JA4 = %q, want %q", got, want)
		}
	})
}

// GenerateJA4 used to return a string that was not JA4. It now returns the
// FoxIO format for the inputs it is given.
func TestGenerateJA4ReturnsFoxIOFormat(t *testing.T) {
	info := foxioHello()
	got := GenerateJA4(0x0304, info.CipherSuites, info.Extensions, nil, "ignored")
	// No ALPN list and no signature algorithms reach GenerateJA4, so the ALPN
	// part is "00" and the last hash covers the extension IDs alone.
	want := "t13d151600_8daaf6152771_" + ja4Hash("0005,000a,000b,000d,0012,0015,0017,001b,0023,002b,002d,0033,4469,ff01")
	if got != want {
		t.Errorf("GenerateJA4 = %q, want %q", got, want)
	}
}

// TestParseClientHelloFromWire checks the parser against a hello built by utls
// from a spec whose fields are known, both as a bare handshake message and
// wrapped in TLS records.
func TestParseClientHelloFromWire(t *testing.T) {
	spec := &utls.ClientHelloSpec{
		TLSVersMin:         utls.VersionTLS12,
		TLSVersMax:         utls.VersionTLS13,
		CipherSuites:       []uint16{utls.GREASE_PLACEHOLDER, 0x1301, 0x1302, 0xc02b, 0xc02f},
		CompressionMethods: []byte{0},
		Extensions: []utls.TLSExtension{
			&utls.UtlsGREASEExtension{},
			&utls.SNIExtension{},
			&utls.SupportedCurvesExtension{Curves: []utls.CurveID{utls.GREASE_PLACEHOLDER, utls.X25519, utls.CurveP256}},
			&utls.SupportedPointsExtension{SupportedPoints: []byte{0}},
			&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{0x0403, 0x0804}},
			&utls.ALPNExtension{AlpnProtocols: []string{"h2", "http/1.1"}},
			&utls.KeyShareExtension{KeyShares: []utls.KeyShare{{Group: utls.X25519}}},
			&utls.SupportedVersionsExtension{Versions: []uint16{utls.GREASE_PLACEHOLDER, utls.VersionTLS13, utls.VersionTLS12}},
		},
	}
	msg := buildClientHello(t, spec)

	const wantJA3 = "771,4865-4866-49195-49199,0-10-11-13-16-51-43,29-23,0"
	wantJA4R := "t13d0407h2_1301,1302,c02b,c02f_000a,000b,000d,002b,0033_0403,0804"

	check := func(t *testing.T, raw []byte) {
		t.Helper()
		info := mustParseHello(t, raw)
		if info.ServerName != specTestServerName || !info.HasSNI {
			t.Errorf("ServerName = %q (HasSNI %v), want %q", info.ServerName, info.HasSNI, specTestServerName)
		}
		if got := info.JA3(); got != wantJA3 {
			t.Errorf("JA3 = %q, want %q", got, wantJA3)
		}
		if got := info.JA4R(); got != wantJA4R {
			t.Errorf("JA4_r = %q, want %q", got, wantJA4R)
		}
		ja3, hash, err := JA3FromClientHello(raw)
		if err != nil || ja3 != wantJA3 || len(hash) != 32 {
			t.Errorf("JA3FromClientHello = %q, %q, %v", ja3, hash, err)
		}
		ja4, err := JA4FromClientHello(raw)
		if err != nil || ja4 != info.JA4() || !strings.HasPrefix(ja4, "t13d0407h2_") {
			t.Errorf("JA4FromClientHello = %q, %v", ja4, err)
		}
		if body, ok := info.ExtensionBody(0x000b); !ok || !bytes.Equal(body, []byte{1, 0}) {
			t.Errorf("ec_point_formats body = %x (present %v), want 0100", body, ok)
		}
	}

	t.Run("handshake message", func(t *testing.T) { check(t, msg) })
	t.Run("one record", func(t *testing.T) { check(t, wrapInRecords(msg, len(msg))) })
	t.Run("split across records", func(t *testing.T) { check(t, wrapInRecords(msg, 100)) })

	for name, bad := range map[string][]byte{
		"empty":            nil,
		"not a hello":      {0x02, 0x00, 0x00, 0x00},
		"truncated":        msg[:len(msg)-10],
		"truncated record": wrapInRecords(msg, len(msg))[:40],
	} {
		if _, err := ParseClientHello(bad); err == nil {
			t.Errorf("ParseClientHello(%s) returned no error", name)
		}
	}
}

// wrapInRecords splits a handshake message into TLS records of at most size
// payload bytes.
func wrapInRecords(msg []byte, size int) []byte {
	var out []byte
	for len(msg) > 0 {
		n := size
		if n > len(msg) {
			n = len(msg)
		}
		out = append(out, 0x16, 0x03, 0x01, byte(n>>8), byte(n))
		out = append(out, msg[:n]...)
		msg = msg[n:]
	}
	return out
}

func hexBody(b []byte) string { return hex.EncodeToString(b) }
