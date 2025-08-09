package unit

import (
	"net/http"
	"testing"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	fhttp "github.com/Danny-Dasilva/fhttp"
	http2 "github.com/Danny-Dasilva/fhttp/http2"
	utls "github.com/refraction-networking/utls"
)

func TestGenerateJA4(t *testing.T) {
	// Test case 1: Basic JA4 generation
	tlsVersion := uint16(0x0304) // TLS 1.3
	cipherSuites := []uint16{0x1301, 0x1302, 0x1303}
	extensions := []uint16{0x0000, 0x000a, 0x000b}
	headers := http.Header{
		"User-Agent":      []string{"Mozilla/5.0"},
		"Accept":          []string{"*/*"},
		"Accept-Language": []string{"en-US"},
	}
	userAgent := "Mozilla/5.0"

	ja4 := cycletls.GenerateJA4(tlsVersion, cipherSuites, extensions, headers, userAgent)

	// Check that the format is correct: t13XXXX_XXXX_XXXX_XXXX
	if len(ja4) != 19 {
		t.Errorf("JA4 length incorrect: got %d, want 19", len(ja4))
	}

	// Check TLS version prefix
	if ja4[:3] != "t13" {
		t.Errorf("JA4 TLS version incorrect: got %s, want t13", ja4[:3])
	}

	// Test case 2: HTTP-only JA4 generation
	ja4http := cycletls.GenerateJA4HTTP(headers, userAgent)

	// Check that the format is correct: XXXX_XXXX
	if len(ja4http) != 9 {
		t.Errorf("JA4 HTTP length incorrect: got %d, want 9", len(ja4http))
	}

	// Check that it has the format XXXX_XXXX
	if ja4http[4:5] != "_" {
		t.Errorf("JA4 HTTP format incorrect: expected underscore at position 4, got %s", ja4http[4:5])
	}
}

func TestGenerateJA4H2(t *testing.T) {
	// Create sample HTTP/2 settings
	h2Settings := []http2.Setting{
		{ID: 1, Val: 65536},       // HEADER_TABLE_SIZE
		{ID: 2, Val: 0},           // ENABLE_PUSH
		{ID: 4, Val: 6291456},     // INITIAL_WINDOW_SIZE
		{ID: 6, Val: 262144},      // MAX_HEADER_LIST_SIZE
	}

	streamDependency := uint32(15663105)
	exclusive := false
	priorityOrder := []string{"m", "a", "s", "p"}

	ja4h2 := cycletls.GenerateJA4H2(h2Settings, streamDependency, exclusive, priorityOrder)

	// Check that the output format is correct
	expected := "1:65536,2:0,4:6291456,6:262144|15663105|0|m,a,s,p"
	if ja4h2 != expected {
		t.Errorf("JA4 H2 incorrect: got %s, want %s", ja4h2, expected)
	}
}

func TestParseJA4String(t *testing.T) {
	// Test case 1: Valid JA4 string (3-part format)
	ja4String := "t13d1717h2_5b57614c22b0_f2748d6cd58d"
	components, err := cycletls.ParseJA4String(ja4String)
	if err != nil {
		t.Errorf("ParseJA4String failed: %v", err)
	}

	if components.TLSVersion != "t13" {
		t.Errorf("TLS version incorrect: got %s, want t13", components.TLSVersion)
	}

	if components.CipherHash != "5b57614c22b0" {
		t.Errorf("Cipher hash incorrect: got %s, want 5b57614c22b0", components.CipherHash)
	}

	if components.ExtensionsHash != "f2748d6cd58d" {
		t.Errorf("Extensions hash incorrect: got %s, want f2748d6cd58d", components.ExtensionsHash)
	}

	// Test case 2: Invalid JA4 string - too short
	_, err = cycletls.ParseJA4String("t13")
	if err == nil {
		t.Error("Expected error for short JA4 string")
	}

	// Test case 3: Invalid JA4 string - wrong format (2 parts instead of 3)
	_, err = cycletls.ParseJA4String("t13d1717h2_5b57614c22b0")
	if err == nil {
		t.Error("Expected error for malformed JA4 string")
	}

	// Test case 4: Invalid JA4 string - 4 parts (old format)
	_, err = cycletls.ParseJA4String("t13d_cd89_1952_bb99")
	if err == nil {
		t.Error("Expected error for old 4-part JA4 format")
	}

	// Test case 5: Real TLS 1.2 JA4 string from provided data
	ja4String = "t12d1209h2_d34a8e72043a_b39be8c56a14"
	components, err = cycletls.ParseJA4String(ja4String)
	if err != nil {
		t.Errorf("ParseJA4String failed for real TLS 1.2 JA4: %v", err)
	}

	if components.TLSVersion != "t12" {
		t.Errorf("TLS version incorrect for real JA4: got %s, want t12", components.TLSVersion)
	}

	if components.CipherHash != "d34a8e72043a" {
		t.Errorf("Cipher hash incorrect for real JA4: got %s, want d34a8e72043a", components.CipherHash)
	}

	if components.ExtensionsHash != "b39be8c56a14" {
		t.Errorf("Extensions hash incorrect for real JA4: got %s, want b39be8c56a14", components.ExtensionsHash)
	}
}

func TestJA4StringToSpec(t *testing.T) {
	// Test case 1: TLS 1.3 JA4 (3-part format)
	ja4String := "t13d1516h2_8daaf6152771_02713d6af862"
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	
	spec, err := cycletls.JA4StringToSpec(ja4String, userAgent, false)
	if err != nil {
		t.Errorf("JA4StringToSpec failed: %v", err)
	}

	if spec == nil {
		t.Error("Spec should not be nil")
	}

	// Check TLS version
	if spec.TLSVersMax != 0x0304 { // TLS 1.3
		t.Errorf("TLS max version incorrect: got %x, want %x", spec.TLSVersMax, 0x0304)
	}

	// Test case 2: TLS 1.2 JA4 (3-part format)
	ja4String = "t12d1516h2_8daaf6152771_02713d6af862"
	
	spec, err = cycletls.JA4StringToSpec(ja4String, userAgent, false)
	if err != nil {
		t.Errorf("JA4StringToSpec failed for TLS 1.2: %v", err)
	}

	if spec.TLSVersMax != 0x0303 { // TLS 1.2
		t.Errorf("TLS max version incorrect for TLS 1.2: got %x, want %x", spec.TLSVersMax, 0x0303)
	}

	// Test case 3: Force HTTP/1
	spec, err = cycletls.JA4StringToSpec(ja4String, userAgent, true)
	if err != nil {
		t.Errorf("JA4StringToSpec failed with forceHTTP1: %v", err)
	}

	// Check that ALPN extension contains only http/1.1
	found := false
	for _, ext := range spec.Extensions {
		if alpn, ok := ext.(*utls.ALPNExtension); ok {
			if len(alpn.AlpnProtocols) == 1 && alpn.AlpnProtocols[0] == "http/1.1" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("Expected ALPN extension with http/1.1 when forceHTTP1 is true")
	}

	// Test case 4: Invalid TLS version (3-part format)
	ja4String = "t99d1516h2_8daaf6152771_02713d6af862"
	_, err = cycletls.JA4StringToSpec(ja4String, userAgent, false)
	if err == nil {
		t.Error("Expected error for invalid TLS version")
	}

	// Test case 5: Real TLS 1.2 JA4 fingerprint from provided data
	ja4String = "t12d1209h2_d34a8e72043a_b39be8c56a14"
	spec, err = cycletls.JA4StringToSpec(ja4String, userAgent, false)
	if err != nil {
		t.Errorf("JA4StringToSpec failed for real TLS 1.2 JA4: %v", err)
	}

	if spec == nil {
		t.Error("Spec should not be nil for real TLS 1.2 JA4")
	}

	// Check TLS version
	if spec.TLSVersMax != 0x0303 { // TLS 1.2
		t.Errorf("TLS max version incorrect for real TLS 1.2 JA4: got %x, want %x", spec.TLSVersMax, 0x0303)
	}
}

func TestParseJA4HString(t *testing.T) {
	// Test case 1: Valid JA4H string (HTTP Client fingerprint)
	ja4hString := "ge11_73a4f1e_8b3fce7"
	components, err := cycletls.ParseJA4HString(ja4hString)
	if err != nil {
		t.Errorf("ParseJA4HString failed: %v", err)
	}

	if components.HTTPMethodVersion != "ge11" {
		t.Errorf("HTTP method/version incorrect: got %s, want ge11", components.HTTPMethodVersion)
	}

	if components.HeadersHash != "73a4f1e" {
		t.Errorf("Headers hash incorrect: got %s, want 73a4f1e", components.HeadersHash)
	}

	if components.CookiesHash != "8b3fce7" {
		t.Errorf("Cookies hash incorrect: got %s, want 8b3fce7", components.CookiesHash)
	}

	// Test case 2: Valid JA4H string with POST HTTP/2.0
	ja4hString = "po20_ab123cd_ef456gh"
	components, err = cycletls.ParseJA4HString(ja4hString)
	if err != nil {
		t.Errorf("ParseJA4HString failed: %v", err)
	}

	if components.HTTPMethodVersion != "po20" {
		t.Errorf("HTTP method/version incorrect: got %s, want po20", components.HTTPMethodVersion)
	}

	// Test case 3: Invalid JA4H string - too short
	_, err = cycletls.ParseJA4HString("ge1")
	if err == nil {
		t.Error("Expected error for short JA4H string")
	}

	// Test case 4: Invalid JA4H string - wrong format (2 parts instead of 3)
	_, err = cycletls.ParseJA4HString("ge11_73a4f1e")
	if err == nil {
		t.Error("Expected error for malformed JA4H string")
	}

	// Test case 5: Invalid JA4H string - method/version too short
	_, err = cycletls.ParseJA4HString("ge_73a4f1e_8b3fce7")
	if err == nil {
		t.Error("Expected error for short method/version in JA4H string")
	}
}

func TestApplyJA4HToRequest(t *testing.T) {
	// Create a test request
	req, err := fhttp.NewRequest("GET", "https://example.com", nil)
	if err != nil {
		t.Errorf("Failed to create test request: %v", err)
	}

	// Test case 1: Apply GET HTTP/1.1 JA4H
	ja4hString := "ge11_73a4f1e_8b3fce7"
	err = cycletls.ApplyJA4HToRequest(req, ja4hString)
	if err != nil {
		t.Errorf("ApplyJA4HToRequest failed: %v", err)
	}

	if req.Method != "GET" {
		t.Errorf("Method not applied correctly: got %s, want GET", req.Method)
	}

	if req.Proto != "HTTP/1.1" {
		t.Errorf("Protocol not applied correctly: got %s, want HTTP/1.1", req.Proto)
	}

	if req.ProtoMajor != 1 || req.ProtoMinor != 1 {
		t.Errorf("Protocol version not applied correctly: got %d.%d, want 1.1", req.ProtoMajor, req.ProtoMinor)
	}

	// Test case 2: Apply POST HTTP/2.0 JA4H
	ja4hString = "po20_ab123cd_ef456gh"
	err = cycletls.ApplyJA4HToRequest(req, ja4hString)
	if err != nil {
		t.Errorf("ApplyJA4HToRequest failed: %v", err)
	}

	if req.Method != "POST" {
		t.Errorf("Method not applied correctly: got %s, want POST", req.Method)
	}

	if req.Proto != "HTTP/2.0" {
		t.Errorf("Protocol not applied correctly: got %s, want HTTP/2.0", req.Proto)
	}

	if req.ProtoMajor != 2 || req.ProtoMinor != 0 {
		t.Errorf("Protocol version not applied correctly: got %d.%d, want 2.0", req.ProtoMajor, req.ProtoMinor)
	}

	// Test case 3: Test other HTTP methods
	testCases := []struct {
		ja4h           string
		expectedMethod string
	}{
		{"pu11_123456_789abc", "PUT"},
		{"he11_123456_789abc", "HEAD"},
		{"de11_123456_789abc", "DELETE"},
		{"pa11_123456_789abc", "PATCH"},
		{"op11_123456_789abc", "OPTIONS"},
		{"xx11_123456_789abc", "GET"}, // Unknown method should default to GET
	}

	for _, tc := range testCases {
		err = cycletls.ApplyJA4HToRequest(req, tc.ja4h)
		if err != nil {
			t.Errorf("ApplyJA4HToRequest failed for %s: %v", tc.ja4h, err)
		}
		if req.Method != tc.expectedMethod {
			t.Errorf("Method not applied correctly for %s: got %s, want %s", tc.ja4h, req.Method, tc.expectedMethod)
		}
	}

	// Test case 4: Invalid JA4H string
	err = cycletls.ApplyJA4HToRequest(req, "invalid")
	if err == nil {
		t.Error("Expected error for invalid JA4H string")
	}
}
