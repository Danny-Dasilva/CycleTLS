package cycletls

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/textproto"
	"sort"
	"strings"

	http2 "github.com/Danny-Dasilva/fhttp/http2"
)

// GenerateJA4 returns a JA4 fingerprint in the FoxIO format
// (for example "t13d1516h2_8daaf6152771_e5627efa2ab1") from a TLS version, a
// cipher suite list and an extension list.
//
// The arguments do not carry the ALPN list or the signature algorithms, so the
// ALPN part is always "00" and the last hash covers the extension IDs only.
// headers and userAgent are not part of JA4 and are ignored; they stay in the
// signature so existing callers keep compiling. Before 3.0.1 this function
// returned a string that was not JA4 (no counts, no ALPN, hashes cut to 1 and 4
// characters, HTTP headers mixed in).
//
// Deprecated: use JA4FromClientHello, which reads every JA4 input from the
// ClientHello bytes.
func GenerateJA4(tlsVersion uint16, cipherSuites []uint16, extensions []uint16, headers http.Header, userAgent string) string {
	info := &ClientHelloInfo{
		Version:      tlsVersion,
		CipherSuites: cipherSuites,
		Extensions:   extensions,
	}
	for _, e := range extensions {
		if e == extensionIDServerName {
			info.HasSNI = true
		}
	}
	return info.JA4()
}

// GenerateJA4HTTP generates a JA4 HTTP fingerprint from HTTP headers only
func GenerateJA4HTTP(headers http.Header, userAgent string) string {
	// Step 1: Headers hash (first 4 characters)
	headersHash := hashHeaders(headers)

	// Step 2: User Agent hash (first 4 characters)
	uaHash := hashUserAgent(userAgent)

	// Format: <Headers hash>_<UA hash>
	return fmt.Sprintf("%s_%s", headersHash[:4], uaHash[:4])
}

// GenerateJA4H2 generates a JA4 HTTP/2 fingerprint from HTTP/2 settings
func GenerateJA4H2(settings []http2.Setting, streamDependency uint32, exclusive bool, priorityOrder []string) string {
	// Format settings as key:value pairs
	settingsStrs := make([]string, 0, len(settings))
	for _, s := range settings {
		settingsStrs = append(settingsStrs, fmt.Sprintf("%d:%d", s.ID, s.Val))
	}

	// Join settings with commas
	settingsStr := strings.Join(settingsStrs, ",")

	// Calculate priority details
	exclusiveFlag := 0
	if exclusive {
		exclusiveFlag = 1
	}

	// Combine priority elements with a comma
	priorityStr := strings.Join(priorityOrder, ",")

	// Final format: settings|streamDependency|exclusive|priorityOrder
	return fmt.Sprintf("%s|%d|%d|%s", settingsStr, streamDependency, exclusiveFlag, priorityStr)
}

// Helper functions

func hashHeaders(headers http.Header) string {
	// Get header names and sort them
	headerNames := make([]string, 0, len(headers))
	for name := range headers {
		headerNames = append(headerNames, textproto.CanonicalMIMEHeaderKey(name))
	}
	sort.Strings(headerNames)

	// Join header names
	h := sha256.New()
	h.Write([]byte(strings.Join(headerNames, ",")))
	return hex.EncodeToString(h.Sum(nil))
}

func hashUserAgent(userAgent string) string {
	h := sha256.New()
	h.Write([]byte(userAgent))
	return hex.EncodeToString(h.Sum(nil))
}
