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

// JA4 is a TLS client fingerprinting technique that captures both TLS and HTTP fingerprints
// Format: <TLS version><TLS ciphers hash>_<TLS extensions hash>_<HTTP headers hash>_<User-Agent hash>
// Example: t13d_cd89_1952_bb99

// GenerateJA4 generates a JA4 fingerprint from the TLS handshake and HTTP headers
func GenerateJA4(tlsVersion uint16, cipherSuites []uint16, extensions []uint16, headers http.Header, userAgent string) string {
	// Step 1: TLS version
	tlsVersionStr := getTLSVersionString(tlsVersion)

	// Step 2: Cipher suites hash (first 1 character)
	cipherHash := hashCipherSuites(cipherSuites)

	// Step 3: Extensions hash (first 4 characters)
	extensionsHash := hashExtensions(extensions)

	// Step 4: HTTP headers hash (first 4 characters)
	headersHash := hashHeaders(headers)

	// Step 5: User Agent hash (first 4 characters)
	uaHash := hashUserAgent(userAgent)

	// Format: <TLS version><Cipher hash>_<Extensions hash>_<Headers hash>_<UA hash>
	// JA4 format: t13d_cd89_1952_bb99 (19 chars total)
	return fmt.Sprintf("%s%s_%s_%s_%s", tlsVersionStr, cipherHash[:1], extensionsHash[:4], headersHash[:4], uaHash[:4])
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

func getTLSVersionString(version uint16) string {
	switch version {
	case 0x0301:
		return "t10"
	case 0x0302:
		return "t11"
	case 0x0303:
		return "t12"
	case 0x0304:
		return "t13"
	default:
		return "t??"
	}
}

func hashCipherSuites(ciphers []uint16) string {
	// Sort and join cipher suites
	cipherStrings := make([]string, len(ciphers))
	for i, cipher := range ciphers {
		cipherStrings[i] = fmt.Sprintf("%04x", cipher)
	}
	sort.Strings(cipherStrings)

	// Hash the joined string
	h := sha256.New()
	h.Write([]byte(strings.Join(cipherStrings, "")))
	return hex.EncodeToString(h.Sum(nil))
}

func hashExtensions(extensions []uint16) string {
	// Sort and join extensions
	extStrings := make([]string, len(extensions))
	for i, ext := range extensions {
		extStrings[i] = fmt.Sprintf("%04x", ext)
	}
	sort.Strings(extStrings)

	// Hash the joined string
	h := sha256.New()
	h.Write([]byte(strings.Join(extStrings, "")))
	return hex.EncodeToString(h.Sum(nil))
}

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
