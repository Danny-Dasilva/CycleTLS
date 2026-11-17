package unit

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
	utls "github.com/refraction-networking/utls"
)

// Issue #427: a JA3 string naming extension 51764 (0xca34, trust_anchors, sent
// by Chrome 141+) was rejected with
// "Extension {{ 51764 }} is not Supported by CycleTLS please raise an issue".

const (
	// Not a Chrome user agent, so StringToSpec adds no GREASE extensions and
	// the spec's extension list lines up one to one with the JA3 string.
	issue427FirefoxUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:141.0) Gecko/20100101 Firefox/141.0"

	issue427Extensions = "0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17613-51764-65037"
	issue427JA3        = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53," + issue427Extensions + ",29-23-24,0"

	// Chrome 149 raw JA4 from refraction-networking/utls#397, with ca34 in the
	// extension list.
	issue427JA4R = "t13d1517h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,ca34,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601"
)

// trustAnchorsWire is the whole extension as Chrome sends it with no trust
// anchor IDs: type 0xca34, length 2, then an empty identifier list.
var trustAnchorsWire = []byte{0xca, 0x34, 0x00, 0x02, 0x00, 0x00}

func assertTrustAnchorsWire(t *testing.T, ext utls.TLSExtension) {
	t.Helper()
	generic, ok := ext.(*utls.GenericExtension)
	if !ok {
		t.Fatalf("extension 51764 is %T, want *utls.GenericExtension", ext)
	}
	if generic.Id != 51764 {
		t.Fatalf("extension ID = %d, want 51764", generic.Id)
	}
	wire, err := io.ReadAll(generic)
	if err != nil {
		t.Fatalf("reading extension 51764: %v", err)
	}
	if !bytes.Equal(wire, trustAnchorsWire) {
		t.Fatalf("extension 51764 wire bytes = % x, want % x", wire, trustAnchorsWire)
	}
}

func TestIssue427_JA3TrustAnchorsExtension(t *testing.T) {
	spec, err := cycletls.StringToSpec(issue427JA3, issue427FirefoxUA, false)
	if err != nil {
		t.Fatalf("StringToSpec rejected a JA3 with extension 51764: %v", err)
	}

	ids := strings.Split(issue427Extensions, "-")
	if len(spec.Extensions) != len(ids) {
		t.Fatalf("spec has %d extensions, JA3 names %d", len(spec.Extensions), len(ids))
	}
	for i, id := range ids {
		if id != "51764" {
			continue
		}
		assertTrustAnchorsWire(t, spec.Extensions[i])

		// The neighbours pin the position: ALPS (17613) before, ECH (65037) after.
		before, ok := spec.Extensions[i-1].(*utls.GenericExtension)
		if !ok || strconv.Itoa(int(before.Id)) != ids[i-1] {
			t.Fatalf("extension before 51764 is %T, want generic %s", spec.Extensions[i-1], ids[i-1])
		}
		if _, ok := spec.Extensions[i+1].(*utls.GREASEEncryptedClientHelloExtension); !ok {
			t.Fatalf("extension after 51764 is %T, want ECH", spec.Extensions[i+1])
		}
		return
	}
	t.Fatal("test JA3 does not name 51764")
}

// With a Chrome user agent StringToSpec adds GREASE extensions, which shifts
// positions. The extension must still be present exactly once.
func TestIssue427_JA3TrustAnchorsExtensionChromeUA(t *testing.T) {
	spec, err := cycletls.StringToSpec(issue427JA3, UserAgent, false)
	if err != nil {
		t.Fatalf("StringToSpec rejected a JA3 with extension 51764: %v", err)
	}
	found := 0
	for _, ext := range spec.Extensions {
		if generic, ok := ext.(*utls.GenericExtension); ok && generic.Id == 51764 {
			assertTrustAnchorsWire(t, ext)
			found++
		}
	}
	if found != 1 {
		t.Fatalf("found extension 51764 %d times, want 1", found)
	}
}

// The JA4R path already accepted unknown IDs, but sent them with an empty body.
// For trust_anchors an empty body is malformed, so check the payload here too.
func TestIssue427_JA4RTrustAnchorsExtension(t *testing.T) {
	spec, err := cycletls.JA4RStringToSpec(issue427JA4R, UserAgent, false, false, "example.com")
	if err != nil {
		t.Fatalf("JA4RStringToSpec failed: %v", err)
	}
	found := 0
	for _, ext := range spec.Extensions {
		if generic, ok := ext.(*utls.GenericExtension); ok && generic.Id == 51764 {
			assertTrustAnchorsWire(t, ext)
			found++
		}
	}
	if found != 1 {
		t.Fatalf("found extension 51764 %d times, want 1", found)
	}
}
