//go:build !integration

package cycletls

import (
	"strings"
	"testing"
)

// TestGenerateClientKey_IsStableHash verifies generateClientKey returns a stable,
// fixed-width FNV-1a hex digest of the config (not the raw config string).
func TestGenerateClientKey_IsStableHash(t *testing.T) {
	browser := Browser{
		JA3:       "771,52244-52243-52245,0-23-35-13,23-24,0",
		UserAgent: "Mozilla/5.0 Test",
	}

	key := generateClientKey(browser, 30, false, "")
	if key != generateClientKey(browser, 30, false, "") {
		t.Error("generateClientKey() should be deterministic for identical config")
	}
	if len(key) != 16 {
		t.Errorf("expected 16-char FNV-1a hex key, got %d chars: %q", len(key), key)
	}
	for _, c := range key {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("key should be lowercase hex, got %q", key)
			break
		}
	}
	if strings.Contains(key, "ja3:") {
		t.Errorf("key should be a hash, not the raw config string, got: %s", key)
	}
}

// TestGenerateClientKey_Deterministic verifies that same options produce same key
func TestGenerateClientKey_Deterministic(t *testing.T) {
	browser := Browser{
		JA3:                "771,52244-52243-52245,0-23-35-13,23-24,0",
		JA4r:               "t13d1516h2_8daaf6152771_02713d6af862",
		HTTP2Fingerprint:   "1:65536,2:0,3:1000,4:6291456,6:262144",
		QUICFingerprint:    "quic_fingerprint_test",
		UserAgent:          "Mozilla/5.0 Test Agent",
		ServerName:         "example.com",
		InsecureSkipVerify: true,
		ForceHTTP1:         false,
		ForceHTTP3:         true,
		Cookies: []Cookie{
			{Name: "session", Value: "abc123"},
			{Name: "token", Value: "xyz789"},
		},
	}

	key1 := generateClientKey(browser, 30, true, "http://proxy.example.com:8080")
	key2 := generateClientKey(browser, 30, true, "http://proxy.example.com:8080")
	key3 := generateClientKey(browser, 30, true, "http://proxy.example.com:8080")

	if key1 != key2 {
		t.Errorf("generateClientKey() not deterministic: key1=%s, key2=%s", key1, key2)
	}
	if key2 != key3 {
		t.Errorf("generateClientKey() not deterministic: key2=%s, key3=%s", key2, key3)
	}
}

// TestGenerateClientKey_DifferentOptionsDifferentKeys verifies that different options produce different keys
func TestGenerateClientKey_DifferentOptionsDifferentKeys(t *testing.T) {
	baseBrowser := Browser{
		JA3:       "771,52244-52243-52245,0-23-35-13,23-24,0",
		UserAgent: "Mozilla/5.0 Test",
	}
	baseTimeout := 30
	baseDisableRedirect := false
	baseProxyURL := ""

	baseKey := generateClientKey(baseBrowser, baseTimeout, baseDisableRedirect, baseProxyURL)

	diffJA3Browser := baseBrowser
	diffJA3Browser.JA3 = "771,49196-49195-49188,0-23-35-13,23-24,0"
	keyDiffJA3 := generateClientKey(diffJA3Browser, baseTimeout, baseDisableRedirect, baseProxyURL)
	if baseKey == keyDiffJA3 {
		t.Error("Different JA3 should produce different key")
	}

	keyDiffRedirect := generateClientKey(baseBrowser, baseTimeout, true, baseProxyURL)
	if baseKey == keyDiffRedirect {
		t.Error("Different disableRedirect should produce different key")
	}

	keyDiffProxy := generateClientKey(baseBrowser, baseTimeout, baseDisableRedirect, "http://proxy.example.com:8080")
	if baseKey == keyDiffProxy {
		t.Error("Different proxy should produce different key")
	}
}

// TestGenerateClientKey_KeyFormatValid verifies the key is a hashed hex digest and
// does not leak the raw config field markers.
func TestGenerateClientKey_KeyFormatValid(t *testing.T) {
	browser := Browser{
		JA3:       "test",
		UserAgent: "test",
	}

	key := generateClientKey(browser, 30, false, "")

	if len(key) != 16 {
		t.Errorf("expected 16-char FNV-1a hex key, got %d chars: %q", len(key), key)
	}
	for _, field := range []string{"ja3:", "ua:", "sni:", "proxy:"} {
		if strings.Contains(key, field) {
			t.Errorf("key should be hashed, but leaks raw field %q: %s", field, key)
		}
	}
}

// TestGenerateClientKey_DistinctConfigsDistinctKeys verifies distinct configs hash
// to distinct keys and an identical config hashes to the same key. (The key is a
// fixed-width hash, so its length no longer grows with the config.)
func TestGenerateClientKey_DistinctConfigsDistinctKeys(t *testing.T) {
	emptyBrowser := Browser{}
	minimalBrowser := Browser{JA3: "test"}
	fullBrowser := Browser{
		JA3:              "771,52244-52243-52245,0-23-35-13,23-24,0",
		JA4r:             "t13d1516h2_8daaf6152771_02713d6af862",
		HTTP2Fingerprint: "1:65536,2:0,3:1000,4:6291456,6:262144",
		UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
	}

	emptyKey := generateClientKey(emptyBrowser, 0, false, "")
	minimalKey := generateClientKey(minimalBrowser, 30, false, "")
	fullKey := generateClientKey(fullBrowser, 120, false, "http://proxy:8080")

	seen := map[string]string{}
	for name, k := range map[string]string{"empty": emptyKey, "minimal": minimalKey, "full": fullKey} {
		if other, dup := seen[k]; dup {
			t.Errorf("distinct configs %q and %q produced the same key %s", other, name, k)
		}
		seen[k] = name
	}

	if minimalKey != generateClientKey(minimalBrowser, 30, false, "") {
		t.Error("identical config should produce the same key")
	}
}

// TestGenerateClientKey_EmptyBrowser verifies handling of empty Browser struct
func TestGenerateClientKey_EmptyBrowser(t *testing.T) {
	browser := Browser{}
	key := generateClientKey(browser, 0, false, "")

	if key == "" {
		t.Error("generateClientKey() should not return empty string for empty browser")
	}
	if len(key) != 16 {
		t.Errorf("expected 16-char FNV-1a hex key, got %d chars: %q", len(key), key)
	}
	if key != generateClientKey(browser, 0, false, "") {
		t.Error("empty-browser key should be deterministic")
	}
}

// TestGenerateClientKey_ConcurrentAccess verifies thread safety
func TestGenerateClientKey_ConcurrentAccess(t *testing.T) {
	browser := Browser{
		JA3:       "test_ja3",
		UserAgent: "test_ua",
	}

	expectedKey := generateClientKey(browser, 30, false, "")

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			key := generateClientKey(browser, 30, false, "")
			if key != expectedKey {
				t.Errorf("Concurrent access produced different key: got %s, expected %s", key, expectedKey)
			}
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}
