//go:build !integration

package cycletls

import (
	"strings"
	"testing"

	uquic "github.com/refraction-networking/uquic"
	utls "github.com/refraction-networking/utls"
)

// baseKeyBrowser returns a fully-populated Browser used as the reference config
// for client-key tests. Mutating a single field must change generateClientKey.
func baseKeyBrowser() Browser {
	return Browser{
		JA3:         "771,4865-4866,0-23,29-23-24,0",
		UserAgent:   "Mozilla/5.0",
		ServerName:  "example.com",
		HeaderOrder: []string{"host", "user-agent"},
		Cookies:     []Cookie{{Name: "a", Value: "1"}, {Name: "b", Value: "2"}},
	}
}

// TestGenerateClientKey_LooksLikeFNVHash asserts the key is a fixed-size FNV-1a
// hex digest, not the raw config string.
func TestGenerateClientKey_LooksLikeFNVHash(t *testing.T) {
	key := generateClientKey(baseKeyBrowser(), 30, false, "")
	if len(key) != 16 {
		t.Fatalf("expected 16-char FNV-1a hex key, got %d chars: %q", len(key), key)
	}
	for _, c := range key {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("key contains non-hex char %q: %s", c, key)
		}
	}
	if strings.Contains(key, "ja3:") {
		t.Fatalf("key looks like the raw config string, not a hash: %s", key)
	}
}

// TestGenerateClientKey_FieldSensitivity asserts that changing ONLY one
// behavior-affecting field (or call parameter) changes the generated key.
// The five fields M4 flagged as previously omitted (HeaderOrder, DisableGrease,
// TLS13AutoRetry, ProxyInsecureSkipVerify, USpec) are covered alongside
// regression coverage for the fields that were already included.
func TestGenerateClientKey_FieldSensitivity(t *testing.T) {
	baseKey := generateClientKey(baseKeyBrowser(), 30, false, "")

	trueVal := true
	falseVal := false

	cases := []struct {
		name   string
		mutate func(b *Browser, timeout *int, redirect *bool, proxy *string)
	}{
		{"JA3", func(b *Browser, _ *int, _ *bool, _ *string) { b.JA3 = "771,4865,0" }},
		{"JA4r", func(b *Browser, _ *int, _ *bool, _ *string) { b.JA4r = "t13d1516h2_abc_def" }},
		{"HTTP2Fingerprint", func(b *Browser, _ *int, _ *bool, _ *string) { b.HTTP2Fingerprint = "1:65536;2:0" }},
		{"QUICFingerprint", func(b *Browser, _ *int, _ *bool, _ *string) { b.QUICFingerprint = "quicfp" }},
		{"UserAgent", func(b *Browser, _ *int, _ *bool, _ *string) { b.UserAgent = "Other/1.0" }},
		{"ServerName", func(b *Browser, _ *int, _ *bool, _ *string) { b.ServerName = "other.com" }},
		{"DisableGrease", func(b *Browser, _ *int, _ *bool, _ *string) { b.DisableGrease = true }},
		{"TLS13AutoRetry", func(b *Browser, _ *int, _ *bool, _ *string) { b.TLS13AutoRetry = true }},
		{"ForceHTTP1", func(b *Browser, _ *int, _ *bool, _ *string) { b.ForceHTTP1 = true }},
		{"ForceHTTP3", func(b *Browser, _ *int, _ *bool, _ *string) { b.ForceHTTP3 = true }},
		{"InsecureSkipVerify", func(b *Browser, _ *int, _ *bool, _ *string) { b.InsecureSkipVerify = true }},
		{"HeaderOrder", func(b *Browser, _ *int, _ *bool, _ *string) { b.HeaderOrder = []string{"user-agent", "host"} }},
		{"ProxyInsecureSkipVerify_false", func(b *Browser, _ *int, _ *bool, _ *string) { b.ProxyInsecureSkipVerify = &falseVal }},
		{"ProxyInsecureSkipVerify_true", func(b *Browser, _ *int, _ *bool, _ *string) { b.ProxyInsecureSkipVerify = &trueVal }},
		{"USpec", func(b *Browser, _ *int, _ *bool, _ *string) { b.USpec = &uquic.QUICSpec{} }},
		{"TLSConfig", func(b *Browser, _ *int, _ *bool, _ *string) { b.TLSConfig = &utls.Config{ServerName: "tls.example"} }},
		{"Cookies", func(b *Browser, _ *int, _ *bool, _ *string) {
			b.Cookies = append(b.Cookies, Cookie{Name: "c", Value: "3"})
		}},
		{"timeout", func(_ *Browser, timeout *int, _ *bool, _ *string) { *timeout = 60 }},
		{"disableRedirect", func(_ *Browser, _ *int, redirect *bool, _ *string) { *redirect = true }},
		{"proxyURL", func(_ *Browser, _ *int, _ *bool, proxy *string) { *proxy = "http://proxy:8080" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := baseKeyBrowser()
			timeout, redirect, proxy := 30, false, ""
			tc.mutate(&b, &timeout, &redirect, &proxy)
			if got := generateClientKey(b, timeout, redirect, proxy); got == baseKey {
				t.Errorf("changing %s did not change the client key (still %s)", tc.name, got)
			}
		})
	}

	// The two ProxyInsecureSkipVerify states must also differ from each other,
	// not merely from the nil base.
	bFalse := baseKeyBrowser()
	bFalse.ProxyInsecureSkipVerify = &falseVal
	bTrue := baseKeyBrowser()
	bTrue.ProxyInsecureSkipVerify = &trueVal
	if generateClientKey(bFalse, 30, false, "") == generateClientKey(bTrue, 30, false, "") {
		t.Error("ProxyInsecureSkipVerify=false and =true produced the same key")
	}
}

// TestGenerateClientKey_StableAndOrderIndependent asserts identical configs hash
// to the same key, cookie ordering is irrelevant (cookies are a set), and
// HeaderOrder ordering IS significant.
func TestGenerateClientKey_StableAndOrderIndependent(t *testing.T) {
	if k1, k2 := generateClientKey(baseKeyBrowser(), 30, false, ""), generateClientKey(baseKeyBrowser(), 30, false, ""); k1 != k2 {
		t.Fatalf("identical configs produced different keys: %s vs %s", k1, k2)
	}

	reordered := baseKeyBrowser()
	reordered.Cookies = []Cookie{{Name: "b", Value: "2"}, {Name: "a", Value: "1"}}
	if k1, k2 := generateClientKey(baseKeyBrowser(), 30, false, ""), generateClientKey(reordered, 30, false, ""); k1 != k2 {
		t.Errorf("reordering cookies changed the key: %s vs %s", k1, k2)
	}

	hdr := baseKeyBrowser()
	hdr.HeaderOrder = []string{"user-agent", "host"}
	if k1, k2 := generateClientKey(baseKeyBrowser(), 30, false, ""), generateClientKey(hdr, 30, false, ""); k1 == k2 {
		t.Errorf("reordering HeaderOrder should change the key but did not: %s", k1)
	}
}
