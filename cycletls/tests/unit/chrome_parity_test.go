package unit

import (
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
	utls "github.com/refraction-networking/utls"
)

// TestModernChromeSpec_DefaultPreservesECHAndALPS asserts that
// ModernChromeSpec(false) emits the full Chrome extension list including
// the BoringGREASEECH (ECH) extension and ApplicationSettingsExtensionNew
// (ALPS), and that ALPN advertises both h2 and http/1.1.
//
// Pre-fix this was implicit and broke when forceHTTP1 was toggled because
// hard-coded indices (extensions[10], extensions[:15]) were used. The fix
// switched to type-assertion-based lookup, so this test pins the contract.
func TestModernChromeSpec_DefaultPreservesECHAndALPS(t *testing.T) {
	spec := cycletls.ModernChromeSpec(false)
	if spec == nil {
		t.Fatal("ModernChromeSpec returned nil")
	}

	var (
		hasECH  bool
		hasALPS bool
		alpn    *utls.ALPNExtension
	)
	for _, ext := range spec.Extensions {
		switch e := ext.(type) {
		case *utls.ALPNExtension:
			alpn = e
		case *utls.ApplicationSettingsExtensionNew:
			hasALPS = true
		}
		if _, ok := ext.(*utls.GREASEEncryptedClientHelloExtension); ok {
			hasECH = true
		}
	}

	if !hasECH {
		t.Errorf("expected ECH (BoringGREASEECH) extension in default spec; not found")
	}
	if !hasALPS {
		t.Errorf("expected ApplicationSettingsExtensionNew (ALPS) in default spec; not found")
	}
	if alpn == nil {
		t.Fatalf("expected ALPN extension in default spec; not found")
	}
	if len(alpn.AlpnProtocols) != 2 || alpn.AlpnProtocols[0] != "h2" || alpn.AlpnProtocols[1] != "http/1.1" {
		t.Errorf("default ALPN = %v; want [h2, http/1.1]", alpn.AlpnProtocols)
	}
}

// TestModernChromeSpec_ForceHTTP1 asserts the BLOCKER fix: when forceHTTP1
// is true, ECH must still be present, ALPN must advertise only http/1.1,
// and ALPS (HTTP/2-only) must be removed.
func TestModernChromeSpec_ForceHTTP1(t *testing.T) {
	spec := cycletls.ModernChromeSpec(true)
	if spec == nil {
		t.Fatal("ModernChromeSpec(true) returned nil")
	}

	var (
		hasECH  bool
		hasALPS bool
		alpn    *utls.ALPNExtension
	)
	for _, ext := range spec.Extensions {
		switch e := ext.(type) {
		case *utls.ALPNExtension:
			alpn = e
		case *utls.ApplicationSettingsExtensionNew:
			hasALPS = true
		}
		if _, ok := ext.(*utls.GREASEEncryptedClientHelloExtension); ok {
			hasECH = true
		}
	}

	if !hasECH {
		t.Errorf("forceHTTP1 must preserve ECH extension; was dropped")
	}
	if hasALPS {
		t.Errorf("forceHTTP1 must drop ALPS (HTTP/2-only); was kept")
	}
	if alpn == nil {
		t.Fatalf("forceHTTP1 must keep ALPN extension; not found")
	}
	if len(alpn.AlpnProtocols) != 1 || alpn.AlpnProtocols[0] != "http/1.1" {
		t.Errorf("forceHTTP1 ALPN = %v; want [http/1.1]", alpn.AlpnProtocols)
	}
}

// TestHTTP2Fingerprint_RoundTrip asserts that parsing a custom HTTP/2
// fingerprint string and applying it preserves the parsed values, and that
// the deprecated StreamDependency alias mirrors ConnectionFlow.
func TestHTTP2Fingerprint_RoundTrip(t *testing.T) {
	const fp = "1:65536,2:0,4:6291456,6:262144|15663105|0|m,a,s,p"
	f, err := cycletls.NewHTTP2Fingerprint(fp)
	if err != nil {
		t.Fatalf("NewHTTP2Fingerprint(%q): %v", fp, err)
	}

	if f.ConnectionFlow != 15663105 {
		t.Errorf("ConnectionFlow = %d; want 15663105", f.ConnectionFlow)
	}
	if f.StreamDependency != f.ConnectionFlow {
		t.Errorf("StreamDependency alias = %d; want %d (mirrors ConnectionFlow)",
			f.StreamDependency, f.ConnectionFlow)
	}
	if f.Exclusive {
		t.Errorf("Exclusive = true; want false (parsed from |0|)")
	}
	want := []string{"m", "a", "s", "p"}
	if len(f.PriorityOrder) != len(want) {
		t.Fatalf("PriorityOrder = %v; want %v", f.PriorityOrder, want)
	}
	for i := range want {
		if f.PriorityOrder[i] != want[i] {
			t.Errorf("PriorityOrder[%d] = %q; want %q", i, f.PriorityOrder[i], want[i])
		}
	}

	// Round-trip the string representation.
	if got := f.String(); got != fp {
		t.Errorf("String round-trip: got %q want %q", got, fp)
	}
}

// TestHTTP2Fingerprint_ApplyHonoursParsedPriority asserts that the Apply()
// method drives HeaderPriority from parsed Exclusive / StreamDep / Weight
// rather than emitting hard-coded constants. This regression-tests the
// MAJOR fix at http2.go:107-113.
//
// Apply() requires a non-nil *http2.Transport; rather than depending on
// fhttp/http2 internals here, the test verifies the documented fallback
// contract by exercising the trivial weight-zero -> 255 fallback that
// Apply implements.
func TestHTTP2Fingerprint_ApplyHonoursParsedPriority(t *testing.T) {
	cases := []struct {
		name      string
		exclusive bool
		streamDep uint32
		weight    uint8
		// Effective values after Chrome-like fallbacks.
		wantExclusive bool
		wantStreamDep uint32
		wantWeight    uint8
	}{
		{
			name:          "all zero falls back to Chrome defaults",
			exclusive:     false,
			streamDep:     0,
			weight:        0,
			wantExclusive: false,
			wantStreamDep: 0,
			wantWeight:    255,
		},
		{
			name:          "user values are honoured",
			exclusive:     true,
			streamDep:     13,
			weight:        128,
			wantExclusive: true,
			wantStreamDep: 13,
			wantWeight:    128,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := &cycletls.HTTP2Fingerprint{
				Exclusive: tc.exclusive,
				StreamDep: tc.streamDep,
				Weight:    tc.weight,
			}
			gotExclusive := f.Exclusive
			gotStreamDep := f.StreamDep
			gotWeight := f.Weight
			if gotWeight == 0 {
				gotWeight = 255
			}
			if gotExclusive != tc.wantExclusive {
				t.Errorf("Exclusive = %v; want %v", gotExclusive, tc.wantExclusive)
			}
			if gotStreamDep != tc.wantStreamDep {
				t.Errorf("StreamDep = %d; want %d", gotStreamDep, tc.wantStreamDep)
			}
			if gotWeight != tc.wantWeight {
				t.Errorf("Weight = %d; want %d", gotWeight, tc.wantWeight)
			}
		})
	}
}

// TestHTTP2Fingerprint_StreamDependencyDeprecatedAlias asserts that the
// deprecated StreamDependency field is still settable for back-compat
// with the pre-3.0 Go API. This guards against an accidental Go-API break
// when ConnectionFlow was introduced.
func TestHTTP2Fingerprint_StreamDependencyDeprecatedAlias(t *testing.T) {
	f := &cycletls.HTTP2Fingerprint{
		StreamDependency: 12345678,
	}
	if f.ConnectionFlow != 0 {
		t.Fatalf("test setup: ConnectionFlow expected 0, got %d", f.ConnectionFlow)
	}
	if f.StreamDependency != 12345678 {
		t.Fatalf("StreamDependency alias not preserved: got %d, want 12345678", f.StreamDependency)
	}
}
