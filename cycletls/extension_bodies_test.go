package cycletls

import (
	"fmt"
	"strings"
	"testing"
)

const firefoxTestUA = "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0"

// ja4rString builds a JA4_r string with correct counts from a cipher list, an
// extension list and a signature algorithm list.
func ja4rString(ciphers, exts, sigAlgs []uint16) string {
	info := &ClientHelloInfo{
		Version:             0x0303,
		CipherSuites:        ciphers,
		Extensions:          exts,
		SignatureAlgorithms: sigAlgs,
		SupportedVersions:   []uint16{0x0304},
		ALPN:                []string{"h2"},
	}
	for _, e := range exts {
		if e == extensionIDServerName {
			info.HasSNI = true
		}
	}
	// JA4_r lists neither server_name nor ALPN; JA4RStringToSpec expects them
	// in the list when they are to be sent in place, so write the list out in
	// full here.
	a, c, _ := info.ja4Parts()
	return a + "_" + c + "_" + joinHex(sortedCopy(exts)) + "_" + joinHex(sigAlgs)
}

var ja4rTestCiphers = []uint16{0x1301, 0x1302, 0x1303, 0xc02b, 0xc02f}
var ja4rTestSigAlgs = []uint16{0x0403, 0x0804, 0x0401}

// The JA4R path used to send these IDs with a zero-length body. That is not a
// valid encoding for any of them except padding, and servers answer with
// decode_error or illegal_parameter.
func TestJA4RExtensionBodiesAreNotEmpty(t *testing.T) {
	exts := []uint16{
		0x0000, 0x0005, 0x000a, 0x000b, 0x000d, 0x0010,
		0x0011, 0x0015, 0x0017, 0x0018, 0x002b, 0x002c, 0x002d, 0x0032, 0x0033,
	}
	ja4r := ja4rString(ja4rTestCiphers, exts, ja4rTestSigAlgs)

	spec, err := JA4RStringToSpec(ja4r, firefoxTestUA, false, true, specTestServerName)
	if err != nil {
		t.Fatalf("JA4RStringToSpec(%q): %v", ja4r, err)
	}
	info := mustParseHello(t, buildClientHello(t, spec))

	want := map[uint16]string{
		0x0011: "00070200040000" + "0000", // status_request_v2: one OCSP item, two empty lists
		0x0018: "",                        // token_binding: checked for length only
		0x002c: "",                        // cookie
		0x0032: "",                        // signature_algorithms_cert
		0x0015: "",                        // padding
	}
	for id, wantHex := range want {
		body, ok := info.ExtensionBody(id)
		if !ok {
			t.Errorf("extension 0x%04x is missing from the hello", id)
			continue
		}
		if len(body) == 0 {
			t.Errorf("extension 0x%04x has a zero-length body", id)
		}
		if wantHex != "" && hexBody(body) != wantHex {
			t.Errorf("extension 0x%04x body = %s, want %s", id, hexBody(body), wantHex)
		}
	}
	if body, _ := info.ExtensionBody(0x0015); len(body) > 0 {
		for _, b := range body {
			if b != 0 {
				t.Errorf("padding body = %x, want zero bytes only", body)
				break
			}
		}
	}

	// Same IDs before and after, so the JA4 the caller asked for is the JA4
	// on the wire.
	wantInfo := &ClientHelloInfo{
		Version: 0x0303, CipherSuites: ja4rTestCiphers, Extensions: exts,
		SignatureAlgorithms: ja4rTestSigAlgs, SupportedVersions: []uint16{0x0304},
		ALPN: []string{"h2"}, HasSNI: true,
	}
	if got, want := info.JA4(), wantInfo.JA4(); got != want {
		t.Errorf("JA4 on the wire = %q, want %q (from %q)", got, want, ja4r)
	}
}

// pre_shared_key (0x0029) must be the last extension of a ClientHello and
// needs identities and binders. With no session to resume there is nothing
// valid to send, so it is left out; it must never sit mid-list with an empty
// body.
func TestJA4RPreSharedKeyIsLastOrAbsent(t *testing.T) {
	exts := []uint16{
		0x0000, 0x0005, 0x000a, 0x000b, 0x000d, 0x0010,
		0x0017, 0x0029, 0x002b, 0x002d, 0x0033,
	}
	ja4r := ja4rString(ja4rTestCiphers, exts, ja4rTestSigAlgs)
	spec, err := JA4RStringToSpec(ja4r, firefoxTestUA, false, true, specTestServerName)
	if err != nil {
		t.Fatalf("JA4RStringToSpec: %v", err)
	}
	info := mustParseHello(t, buildClientHello(t, spec))
	for i, id := range info.Extensions {
		if id != 0x0029 {
			continue
		}
		if i != len(info.Extensions)-1 {
			t.Errorf("pre_shared_key is extension %d of %d, want last", i+1, len(info.Extensions))
		}
		if len(info.ExtensionData[i]) == 0 {
			t.Errorf("pre_shared_key has a zero-length body")
		}
	}
	// Every other ID from the string is still sent.
	for _, id := range exts {
		if id == 0x0029 {
			continue
		}
		if _, ok := info.ExtensionBody(id); !ok {
			t.Errorf("extension 0x%04x is missing from the hello", id)
		}
	}
}

// JA3 strings with these IDs used to fail (15, 30031) or carry a body no
// client sends (17 empty, 30032 one zero byte).
func TestJA3ExtensionIDs(t *testing.T) {
	const extList = "0-23-65281-10-11-35-16-5-15-17-13-30031-30032-17613-51-45-43"
	ja3 := "771,4865-4866-4867-49195-49199," + extList + ",29-23-24,0"

	spec, err := StringToSpec(ja3, firefoxTestUA, false)
	if err != nil {
		t.Fatalf("StringToSpec(%q): %v", ja3, err)
	}
	info := mustParseHello(t, buildClientHello(t, spec))

	// Position: the JA3 of the hello is the JA3 that was asked for.
	if got := info.JA3(); got != ja3 {
		t.Errorf("JA3 on the wire = %q\nwant %q", got, ja3)
	}

	wantBodies := map[uint16]string{
		15:    "01",                 // heartbeat: peer_allowed_to_send
		17:    "000702000400000000", // status_request_v2
		30031: "",                   // channel_id (old codepoint)
		30032: "",                   // channel_id
		17613: "0003026832",         // ALPS, new codepoint: ["h2"]
	}
	for id, want := range wantBodies {
		body, ok := info.ExtensionBody(id)
		if !ok {
			t.Errorf("extension %d is missing from the hello", id)
			continue
		}
		if hexBody(body) != want {
			t.Errorf("extension %d body = %q, want %q", id, hexBody(body), want)
		}
	}
}

// Each new ID also works on its own and lands where the JA3 puts it.
func TestJA3NewExtensionIDsOneByOne(t *testing.T) {
	for _, id := range []string{"15", "30031"} {
		t.Run(id, func(t *testing.T) {
			for _, exts := range []string{
				id + "-0-10-11-13-43-51",
				"0-10-11-" + id + "-13-43-51",
				"0-10-11-13-43-51-" + id,
			} {
				ja3 := fmt.Sprintf("771,4865-4866-49195,%s,29-23,0", exts)
				spec, err := StringToSpec(ja3, firefoxTestUA, false)
				if err != nil {
					t.Fatalf("StringToSpec(%q): %v", ja3, err)
				}
				got := mustParseHello(t, buildClientHello(t, spec)).JA3()
				if strings.Split(got, ",")[2] != exts {
					t.Errorf("extensions on the wire = %q, want %q", strings.Split(got, ",")[2], exts)
				}
			}
		})
	}

	// An ID with no known valid body is still refused.
	if _, err := StringToSpec("771,4865,0-10-11-42-13,29,0", firefoxTestUA, false); err == nil {
		t.Errorf("StringToSpec accepted extension 42 (early_data), want an error")
	}
}
