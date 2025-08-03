package unit

import (
	"net/http"
	"testing"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
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
	settings := []struct {
		ID  uint32
		Val uint32
	}{
		{ID: 1, Val: 65536},       // HEADER_TABLE_SIZE
		{ID: 2, Val: 0},           // ENABLE_PUSH
		{ID: 4, Val: 6291456},     // INITIAL_WINDOW_SIZE
		{ID: 6, Val: 262144},      // MAX_HEADER_LIST_SIZE
	}

	// Convert to proper type
	h2Settings := make([]cycletls.HTTP2Setting, len(settings))
	for i, s := range settings {
		h2Settings = append(h2Settings, cycletls.HTTP2Setting{ID: s.ID, Val: s.Val})
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