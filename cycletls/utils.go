package cycletls

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
	fhttp "github.com/Danny-Dasilva/fhttp"
	utls "github.com/refraction-networking/utls"
	uquic "github.com/refraction-networking/uquic"
)

const (
	chrome  = "chrome"  //chrome User agent enum
	firefox = "firefox" //firefox User agent enum
)

// Cipher suite mappings from hex to uTLS constants
var cipherSuiteMap = map[uint16]uint16{
	// TLS 1.3 cipher suites
	0x1301: utls.TLS_AES_128_GCM_SHA256,
	0x1302: utls.TLS_AES_256_GCM_SHA384,
	0x1303: utls.TLS_CHACHA20_POLY1305_SHA256,

	// TLS 1.2 and below cipher suites
	0x002f: utls.TLS_RSA_WITH_AES_128_CBC_SHA,
	0x0035: utls.TLS_RSA_WITH_AES_256_CBC_SHA,
	0x009c: utls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	0x009d: utls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	0xc009: utls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	0xc00a: utls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	0xc013: utls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	0xc014: utls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	0xc02b: utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	0xc02c: utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	0xc02f: utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	0xc030: utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	0xcca8: utls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	0xcca9: utls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	0x000a: utls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	0xc023: utls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	0xc027: utls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
}

type UserAgent struct {
	UserAgent   string
	HeaderOrder []string
}

// ParseUserAgent returns the pseudo header order and user agent string for chrome/firefox
func parseUserAgent(userAgent string) UserAgent {
	switch {
	case strings.Contains(strings.ToLower(userAgent), "chrome"):
		return UserAgent{chrome, []string{":method", ":authority", ":scheme", ":path"}}
	case strings.Contains(strings.ToLower(userAgent), "firefox"):
		return UserAgent{firefox, []string{":method", ":path", ":authority", ":scheme"}}
	default:
		return UserAgent{chrome, []string{":method", ":authority", ":scheme", ":path"}}
	}

}

// DecompressBody unzips compressed data following axios-style automatic decompression
func DecompressBody(Body []byte, encoding []string, content []string) (parsedBody []byte) {
	// If no encoding specified, return original body
	if len(encoding) == 0 {
		return Body
	}

	// Handle multiple encodings (e.g., "gzip, deflate") - process first encoding
	encodingType := strings.ToLower(strings.TrimSpace(encoding[0]))

	switch encodingType {
	case "gzip":
		unz, err := gUnzipData(Body)
		if err != nil {
			// Return original body on decompression failure (axios behavior)
			return Body
		}
		return unz
	case "deflate":
		unz, err := enflateData(Body)
		if err != nil {
			// Return original body on decompression failure (axios behavior)
			return Body
		}
		return unz
	case "br", "brotli":
		unz, err := unBrotliData(Body)
		if err != nil {
			// Return original body on decompression failure (axios behavior)
			return Body
		}
		return unz
	default:
		// Unknown encoding, return original body
		return Body
	}
}

func gUnzipData(data []byte) (resData []byte, err error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return []byte{}, err
	}
	defer gz.Close()
	respBody, err := io.ReadAll(gz)
	return respBody, err
}
func enflateData(data []byte) (resData []byte, err error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return []byte{}, err
	}
	defer zr.Close()
	enflated, err := io.ReadAll(zr)
	return enflated, err
}
func unBrotliData(data []byte) (resData []byte, err error) {
	br := brotli.NewReader(bytes.NewReader(data))
	respBody, err := io.ReadAll(br)
	return respBody, err
}



// StringToSpec creates a ClientHelloSpec based on a JA3 string
func StringToSpec(ja3 string, userAgent string, forceHTTP1 bool) (*utls.ClientHelloSpec, error) {
	parsedUserAgent := parseUserAgent(userAgent)
	// if tlsExtensions == nil {
	// 	tlsExtensions = &TLSExtensions{}
	// }
	// ext := tlsExtensions
	extMap := genMap(false)
	tokens := strings.Split(ja3, ",")

	version := tokens[0]
	ciphers := strings.Split(tokens[1], "-")
	extensions := strings.Split(tokens[2], "-")
	curves := strings.Split(tokens[3], "-")
	if len(curves) == 1 && curves[0] == "" {
		curves = []string{}
	}
	pointFormats := strings.Split(tokens[4], "-")
	if len(pointFormats) == 1 && pointFormats[0] == "" {
		pointFormats = []string{}
	}
	// parse curves
	var targetCurves []utls.CurveID
	// if parsedUserAgent == chrome && !tlsExtensions.UseGREASE {
	if parsedUserAgent.UserAgent == chrome {
		targetCurves = append(targetCurves, utls.CurveID(utls.GREASE_PLACEHOLDER)) //append grease for Chrome browsers
		if supportedVersionsExt, ok := extMap["43"]; ok {
			if supportedVersions, ok := supportedVersionsExt.(*utls.SupportedVersionsExtension); ok {
				supportedVersions.Versions = append([]uint16{utls.GREASE_PLACEHOLDER}, supportedVersions.Versions...)
			}
		}
		if keyShareExt, ok := extMap["51"]; ok {
			if keyShare, ok := keyShareExt.(*utls.KeyShareExtension); ok {
				keyShare.KeyShares = append([]utls.KeyShare{{Group: utls.CurveID(utls.GREASE_PLACEHOLDER), Data: []byte{0}}}, keyShare.KeyShares...)
			}
		}
	} else {
		if keyShareExt, ok := extMap["51"]; ok {
			if keyShare, ok := keyShareExt.(*utls.KeyShareExtension); ok {
				keyShare.KeyShares = append(keyShare.KeyShares, utls.KeyShare{Group: utls.CurveP256})
			}
		}
	}
	for _, c := range curves {
		cid, err := strconv.ParseUint(c, 10, 16)
		if err != nil {
			return nil, err
		}
		targetCurves = append(targetCurves, utls.CurveID(cid))
	}
	extMap["10"] = &utls.SupportedCurvesExtension{Curves: targetCurves}

	// parse point formats
	var targetPointFormats []byte
	for _, p := range pointFormats {
		pid, err := strconv.ParseUint(p, 10, 8)
		if err != nil {
			return nil, err
		}
		targetPointFormats = append(targetPointFormats, byte(pid))
	}
	extMap["11"] = &utls.SupportedPointsExtension{SupportedPoints: targetPointFormats}

	// force http1
	if forceHTTP1 {
		extMap["16"] = &utls.ALPNExtension{
			AlpnProtocols: []string{"http/1.1"},
		}
	}

	// set extension 43
	ver, err := strconv.ParseUint(version, 10, 16)
	if err != nil {
		return nil, err
	}
	tlsMaxVersion, tlsMinVersion, tlsExtension, err := createTlsVersion(uint16(ver), false)
	extMap["43"] = tlsExtension

	// build extenions list
	var exts []utls.TLSExtension
	//Optionally Add Chrome Grease Extension
	// if parsedUserAgent == chrome && !tlsExtensions.UseGREASE {
	if parsedUserAgent.UserAgent == chrome {
		exts = append(exts, &utls.UtlsGREASEExtension{})
	}
	for _, e := range extensions {
		te, ok := extMap[e]
		if !ok {
			return nil, raiseExtensionError(e)
		}
		// //Optionally add Chrome Grease Extension
		// if e == "21" && parsedUserAgent == chrome && !tlsExtensions.UseGREASE {
		if e == "21" && parsedUserAgent.UserAgent == chrome {
			exts = append(exts, &utls.UtlsGREASEExtension{})
		}
		exts = append(exts, te)
	}

	// build CipherSuites
	var suites []uint16
	//Optionally Add Chrome Grease Extension
	// if parsedUserAgent == chrome && !tlsExtensions.UseGREASE {
	if parsedUserAgent.UserAgent == chrome {
		suites = append(suites, utls.GREASE_PLACEHOLDER)
	}
	for _, c := range ciphers {
		cid, err := strconv.ParseUint(c, 10, 16)
		if err != nil {
			return nil, err
		}
		suites = append(suites, uint16(cid))
	}
	return &utls.ClientHelloSpec{
		TLSVersMin:         tlsMinVersion,
		TLSVersMax:         tlsMaxVersion,
		CipherSuites:       suites,
		CompressionMethods: []byte{0},
		Extensions:         exts,
		GetSessionID:       sha256.Sum256,
	}, nil
}

// JA4Components represents the parsed components of a JA4 string
type JA4Components struct {
	TLSVersion     string
	SNI            string // "d" for domain, "i" for IP
	CipherCount    int    // Number of cipher suites
	ExtensionCount int    // Number of extensions
	ALPN           string // ALPN value (e.g., "h2", "h1")
	CipherHash     string
	ExtensionsHash string
	HeadersHash    string // Legacy field
	UserAgentHash  string // Legacy field
}

// JA4HComponents represents the parsed components of a JA4H (HTTP Client) string
type JA4HComponents struct {
	HTTPMethodVersion string
	HeadersHash       string
	CookiesHash       string
}

// ParseJA4String parses a JA4 string into its components
// JA4 format: <TLS version><cipher hash>_<extensions hash>_<headers hash>_<UA hash>
// Example: t13d_cd89_1952_bb99
func ParseJA4String(ja4 string) (*JA4Components, error) {
	if len(ja4) < 18 { // minimum reasonable length for JA4: t12d1209h2_123456789012_123456789012
		return nil, errors.New("invalid JA4 string: too short")
	}

	// Split by underscores
	parts := strings.Split(ja4, "_")
	if len(parts) != 3 {
		return nil, errors.New("invalid JA4 string: incorrect format - expected 3 parts separated by underscores")
	}

	// Parse first part: [protocol][version][sni][cipher_count][extension_count][alpn]
	// Example: t12d1209h2 = t + 12 + d + 12 + 09 + h2
	ja4a := parts[0]
	if len(ja4a) < 7 {
		return nil, errors.New("invalid JA4 string: first part too short")
	}

	// Extract protocol (should be 't' for TLS)
	if ja4a[0] != 't' {
		return nil, errors.New("invalid JA4 string: must start with 't' for TLS")
	}

	// Extract TLS version (2 digits)
	if len(ja4a) < 3 {
		return nil, errors.New("invalid JA4 string: missing TLS version")
	}
	tlsVersion := ja4a[:3] // t10, t11, t12, t13
	if !(ja4a[1:3] == "10" || ja4a[1:3] == "11" || ja4a[1:3] == "12" || ja4a[1:3] == "13") {
		return nil, errors.New("invalid JA4 string: invalid TLS version: " + ja4a[1:3])
	}

	// Extract SNI indicator (1 character: 'd' for domain, 'i' for IP)
	if len(ja4a) < 4 {
		return nil, errors.New("invalid JA4 string: missing SNI indicator")
	}
	sni := string(ja4a[3])
	if sni != "d" && sni != "i" {
		return nil, errors.New("invalid JA4 string: invalid SNI indicator: " + sni)
	}

	// Find ALPN at the end (variable length, 2+ characters)
	// Work backwards to find where ALPN starts
	if len(ja4a) < 7 {
		return nil, errors.New("invalid JA4 string: too short for cipher/extension counts and ALPN")
	}

	// ALPN is at least 2 characters, cipher and extension counts are 2 digits each
	// So minimum structure: t12d1209h2 (10 chars total)
	if len(ja4a) < 8 {
		return nil, errors.New("invalid JA4 string: insufficient length for all components")
	}

	// ALPN is usually 2 characters for common values (h2, h1, etc.)
	// Work backwards from the end to find ALPN
	// For t12d1209h2, we want: t(1) + 12(2) + d(1) + 1209(4) + h2(2) = 10 total
	// So ALPN starts at position 8 for this case
	if len(ja4a) < 8 {
		return nil, errors.New("invalid JA4 string: too short for minimum components")
	}

	// Most common ALPN values are 2 characters: h2, h1
	// But could be longer: h11, http1, etc.
	// For now, assume 2 characters and validate
	alpn := ja4a[len(ja4a)-2:]

	// Extract cipher count and extension count (4 digits total between SNI and ALPN)
	countsStr := ja4a[4 : len(ja4a)-2] // From position 4 to 2 chars before end
	if len(countsStr) != 4 {
		return nil, fmt.Errorf("invalid JA4 string: expected 4 digits for cipher/extension counts, got %d", len(countsStr))
	}

	cipherCount, err := strconv.Atoi(countsStr[:2])
	if err != nil {
		return nil, fmt.Errorf("invalid JA4 string: invalid cipher count: %s", countsStr[:2])
	}

	extensionCount, err := strconv.Atoi(countsStr[2:])
	if err != nil {
		return nil, fmt.Errorf("invalid JA4 string: invalid extension count: %s", countsStr[2:])
	}

	// JA4_b is the cipher suites hash (12 characters)
	cipherHash := parts[1]
	if len(cipherHash) != 12 {
		return nil, fmt.Errorf("invalid JA4 string: cipher hash must be 12 characters, got %d", len(cipherHash))
	}

	// JA4_c is the extensions hash (12 characters)
	extensionsHash := parts[2]
	if len(extensionsHash) != 12 {
		return nil, fmt.Errorf("invalid JA4 string: extension hash must be 12 characters, got %d", len(extensionsHash))
	}

	return &JA4Components{
		TLSVersion:     tlsVersion,
		SNI:            sni,
		CipherCount:    cipherCount,
		ExtensionCount: extensionCount,
		ALPN:           alpn,
		CipherHash:     cipherHash,
		ExtensionsHash: extensionsHash,
		HeadersHash:    "", // Not used in 3-part format
		UserAgentHash:  "", // Not used in 3-part format
	}, nil
}

// JA4RComponents represents the parsed components of a JA4_r (raw) string
type JA4RComponents struct {
	TLSVersion       string
	SNI              string   // "d" for domain, "i" for IP
	CipherCount      int      // Number of cipher suites
	ExtensionCount   int      // Number of extensions
	ALPN             string   // ALPN value (e.g., "h2" for HTTP/2)
	CipherSuites     []uint16 // Raw cipher suite values
	Extensions       []uint16 // Raw extension values
	SignatureSchemes []uint16 // Raw signature scheme values (optional, from 4th part)
}

// ParseJA4RString parses a JA4_r (raw) string into its components
// JA4_r format: <prefix>_<ciphers>_<extensions>_<signatures>
// Example: t13d1717h2_002f,0035,009c,009d,1301,1302,1303_0000,0005,000a,000b,000d_0403,0503,0603,0804,0805,0806
func ParseJA4RString(ja4r string) (*JA4RComponents, error) {
	// Split by underscores
	parts := strings.Split(ja4r, "_")
	if len(parts) < 3 {
		return nil, errors.New("invalid JA4_r string: incorrect format - expected at least 3 parts")
	}

	// Parse first part: [protocol][version][sni][cipher_count][extension_count][alpn]
	// Example: t13d1717h2 = t + 13 + d + 17 + 17 + h2
	// Example: t12d128h2 = t + 12 + d + 12 + 8 + h2 (note: extension count can be 1 or 2 digits)
	ja4a := parts[0]
	if len(ja4a) < 6 {
		return nil, errors.New("invalid JA4_r string: first part too short")
	}

	// Extract protocol (should be 't' for TLS)
	if ja4a[0] != 't' {
		return nil, errors.New("invalid JA4_r string: must start with 't' for TLS")
	}

	// Extract TLS version (2 digits)
	tlsVersion := "t" + ja4a[1:3]

	// Extract SNI indicator (1 character: 'd' for domain, 'i' for IP)
	sni := string(ja4a[3])
	if sni != "d" && sni != "i" {
		return nil, errors.New("invalid JA4_r string: SNI indicator must be 'd' or 'i'")
	}

	// Extract cipher count (2 digits)
	cipherCountStr := ja4a[4:6]
	cipherCount, err := strconv.Atoi(cipherCountStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JA4_r string: invalid cipher count: %w", err)
	}

	// Extract extension count and ALPN (variable length parsing)
	// We need to find where the extension count ends and ALPN begins
	remainder := ja4a[6:] // Everything after cipher count

	// Parse extension count - can be 1 or 2 digits
	var extensionCount int
	var alpn string

	// Try parsing different lengths for extension count
	if len(remainder) >= 2 {
		// Try 2-digit extension count first
		if twoDigitCount, err := strconv.Atoi(remainder[0:2]); err == nil {
			// Check if the remaining characters after 2-digit number form a valid ALPN
			remainingAfter2 := remainder[2:]
			if remainingAfter2 == "" || remainingAfter2 == "h1" || remainingAfter2 == "h2" ||
				remainingAfter2 == "h3" || strings.HasPrefix(remainingAfter2, "h") {
				extensionCount = twoDigitCount
				alpn = remainingAfter2
			} else {
				// 2-digit doesn't work, try 1-digit
				if oneDigitCount, err := strconv.Atoi(remainder[0:1]); err == nil {
					extensionCount = oneDigitCount
					alpn = remainder[1:]
				} else {
					return nil, fmt.Errorf("invalid JA4_r string: cannot parse extension count from '%s'", remainder)
				}
			}
		} else {
			// 2-digit failed, try 1-digit
			if oneDigitCount, err := strconv.Atoi(remainder[0:1]); err == nil {
				extensionCount = oneDigitCount
				alpn = remainder[1:]
			} else {
				return nil, fmt.Errorf("invalid JA4_r string: cannot parse extension count from '%s'", remainder)
			}
		}
	} else if len(remainder) >= 1 {
		// Only 1 character left, must be extension count with no ALPN
		if oneDigitCount, err := strconv.Atoi(remainder[0:1]); err == nil {
			extensionCount = oneDigitCount
			alpn = ""
		} else {
			return nil, fmt.Errorf("invalid JA4_r string: cannot parse extension count from '%s'", remainder)
		}
	} else {
		return nil, errors.New("invalid JA4_r string: missing extension count")
	}

	// Parse cipher suites from part 2
	cipherSuites := []uint16{}
	if parts[1] != "" {
		cipherStrs := strings.Split(parts[1], ",")
		for _, cipherStr := range cipherStrs {
			// Parse hex string (may or may not have 0x prefix)
			cipherStr = strings.TrimPrefix(cipherStr, "0x")
			cipher, err := strconv.ParseUint(cipherStr, 16, 16)
			if err != nil {
				return nil, fmt.Errorf("invalid cipher suite hex: %s: %w", cipherStr, err)
			}
			cipherSuites = append(cipherSuites, uint16(cipher))
		}
	}

	// Parse extensions from part 3
	extensions := []uint16{}
	if parts[2] != "" {
		extStrs := strings.Split(parts[2], ",")
		for _, extStr := range extStrs {
			// Parse hex string (may or may not have 0x prefix)
			extStr = strings.TrimPrefix(extStr, "0x")
			ext, err := strconv.ParseUint(extStr, 16, 16)
			if err != nil {
				return nil, fmt.Errorf("invalid extension hex: %s: %w", extStr, err)
			}
			extensions = append(extensions, uint16(ext))
		}
	}

	// Parse signature schemes from part 4 (optional)
	signatureSchemes := []uint16{}
	if len(parts) > 3 && parts[3] != "" {
		sigStrs := strings.Split(parts[3], ",")
		for _, sigStr := range sigStrs {
			// Parse hex string (may or may not have 0x prefix)
			sigStr = strings.TrimPrefix(sigStr, "0x")
			sig, err := strconv.ParseUint(sigStr, 16, 16)
			if err != nil {
				return nil, fmt.Errorf("invalid signature scheme hex: %s: %w", sigStr, err)
			}
			signatureSchemes = append(signatureSchemes, uint16(sig))
		}
	}

	return &JA4RComponents{
		TLSVersion:       tlsVersion,
		SNI:              sni,
		CipherCount:      cipherCount,
		ExtensionCount:   extensionCount,
		ALPN:             alpn,
		CipherSuites:     cipherSuites,
		Extensions:       extensions,
		SignatureSchemes: signatureSchemes,
	}, nil
}

// ParseJA4HString parses a JA4H (HTTP Client) string into its components
// JA4H format: <method_version>_<headers_hash>_<cookies_hash>
// Example: po11_73a4f1e_8b3fce7
func ParseJA4HString(ja4h string) (*JA4HComponents, error) {
	if len(ja4h) < 8 { // minimum reasonable length for JA4H
		return nil, errors.New("invalid JA4H string: too short")
	}

	// Split by underscores
	parts := strings.Split(ja4h, "_")
	if len(parts) != 3 {
		return nil, errors.New("invalid JA4H string: incorrect format - expected 3 parts separated by underscores")
	}

	// Validate method version format (e.g., "po11" for POST HTTP/1.1, "ge20" for GET HTTP/2.0)
	httpMethodVersion := parts[0]
	if len(httpMethodVersion) < 4 {
		return nil, errors.New("invalid JA4H string: HTTP method/version too short")
	}

	headersHash := parts[1]
	cookiesHash := parts[2]

	return &JA4HComponents{
		HTTPMethodVersion: httpMethodVersion,
		HeadersHash:       headersHash,
		CookiesHash:       cookiesHash,
	}, nil
}

// JA4StringToSpec creates a ClientHelloSpec based on a JA4 string
// Since JA4 uses hashes, we create a spec with common TLS parameters
// that would produce a similar fingerprint

// JA4RStringToSpec creates a ClientHelloSpec from a JA4_r (raw) string
func JA4RStringToSpec(ja4r string, userAgent string, forceHTTP1 bool, disableGrease bool) (*utls.ClientHelloSpec, error) {
	components, err := ParseJA4RString(ja4r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JA4_r: %w", err)
	}

	// Map TLS version string to actual version
	var tlsVersion uint16
	var tlsMinVersion uint16
	var tlsMaxVersion uint16

	switch components.TLSVersion {
	case "t10":
		tlsVersion = utls.VersionTLS10
		tlsMinVersion = utls.VersionTLS10
		tlsMaxVersion = utls.VersionTLS12
	case "t11":
		tlsVersion = utls.VersionTLS11
		tlsMinVersion = utls.VersionTLS11
		tlsMaxVersion = utls.VersionTLS12
	case "t12":
		tlsVersion = utls.VersionTLS12
		tlsMinVersion = utls.VersionTLS12
		tlsMaxVersion = utls.VersionTLS12
	case "t13":
		tlsVersion = utls.VersionTLS13
		tlsMinVersion = utls.VersionTLS12
		tlsMaxVersion = utls.VersionTLS13
	default:
		return nil, errors.New("unsupported TLS version in JA4_r: " + components.TLSVersion)
	}

	// Map cipher suites from raw values
	cipherSuites := []uint16{}
	for _, rawCipher := range components.CipherSuites {
		// Check if we have a mapping for this cipher
		if mappedCipher, exists := cipherSuiteMap[rawCipher]; exists {
			cipherSuites = append(cipherSuites, mappedCipher)
		} else {
			// Use raw value if no mapping exists
			cipherSuites = append(cipherSuites, rawCipher)
		}
	}

	// Check if ALPN extension (0x0010) is present in the extensions list
	hasALPNExtension := false
	for _, extCode := range components.Extensions {
		if extCode == 0x0010 {
			hasALPNExtension = true
			break
		}
	}

	// Handle forceHTTP1 by modifying ALPN to use HTTP/1.1 only
	if forceHTTP1 && components.ALPN != "" {
		components.ALPN = "h1"
	}

	// Build extensions based on raw values using the new extension framework
	extensions := []utls.TLSExtension{}

	// Process extensions from JA4_r using CreateExtensionFromID
	for _, extCode := range components.Extensions {
		if ext := CreateExtensionFromID(extCode, tlsVersion, components, disableGrease); ext != nil {
			extensions = append(extensions, ext)
		}
	}

	// Add ALPN extension manually if:
	// 1. ALPN is specified in the header (components.ALPN != "")
	// 2. AND 0x0010 was NOT in the extensions list
	if components.ALPN != "" && !hasALPNExtension {
		var alpnProtocols []string
		switch components.ALPN {
		case "h2":
			alpnProtocols = []string{"h2", "http/1.1"}
		case "h1":
			alpnProtocols = []string{"http/1.1"}
		case "h3":
			alpnProtocols = []string{"h3", "h2", "http/1.1"}
		default:
			// For other ALPN values, use them directly
			alpnProtocols = []string{components.ALPN}
		}

		alpnExt := &utls.ALPNExtension{
			AlpnProtocols: alpnProtocols,
		}
		extensions = append(extensions, alpnExt)
	}


	return &utls.ClientHelloSpec{
		TLSVersMin:         tlsMinVersion,
		TLSVersMax:         tlsMaxVersion,
		CipherSuites:       cipherSuites,
		CompressionMethods: []byte{0}, // no compression
		Extensions:         extensions,
		GetSessionID:       sha256.Sum256,
	}, nil
}

// ConvertFhttpHeader converts fhttp.Header to http.Header
func ConvertFhttpHeader(fh fhttp.Header) http.Header {
	h := make(http.Header)
	for k, v := range fh {
		h[k] = v
	}
	return h
}

// ConvertHttpHeader converts http.Header to fhttp.Header
func ConvertHttpHeader(h http.Header) fhttp.Header {
	fh := make(fhttp.Header)
	for k, v := range h {
		fh[k] = v
	}
	return fh
}

// ConvertUtlsConfig converts utls.Config to tls.Config
func ConvertUtlsConfig(utlsConfig *utls.Config) *tls.Config {
	if utlsConfig == nil {
		return nil
	}

	return &tls.Config{
		Rand:               utlsConfig.Rand,
		Time:               utlsConfig.Time,
		RootCAs:            utlsConfig.RootCAs,
		NextProtos:         utlsConfig.NextProtos,
		ServerName:         utlsConfig.ServerName,
		InsecureSkipVerify: utlsConfig.InsecureSkipVerify,
		CipherSuites:       utlsConfig.CipherSuites,
		MinVersion:         utlsConfig.MinVersion,
		MaxVersion:         utlsConfig.MaxVersion,
	}
}

// MarshalHeader preserves header order while converting to http.Header
func MarshalHeader(h fhttp.Header, order []string) http.Header {
	result := make(http.Header)

	// Add ordered headers first
	for _, key := range order {
		if values, ok := h[key]; ok {
			result[key] = values
		}
	}

	// Add remaining headers
	for key, values := range h {
		if _, exists := result[key]; !exists {
			result[key] = values
		}
	}

	return result
}

// PrettyStruct formats json
func PrettyStruct(data interface{}) (string, error) {
	val, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// TLSVersion，Ciphers，Extensions，EllipticCurves，EllipticCurvePointFormats
func createTlsVersion(ver uint16, disableGrease bool) (tlsMaxVersion uint16, tlsMinVersion uint16, tlsSuppor utls.TLSExtension, err error) {
	switch ver {
	case utls.VersionTLS13:
		tlsMaxVersion = utls.VersionTLS13
		tlsMinVersion = utls.VersionTLS12
		versions := []uint16{}
		if !disableGrease {
			versions = append(versions, utls.GREASE_PLACEHOLDER)
		}
		versions = append(versions, utls.VersionTLS13, utls.VersionTLS12)
		tlsSuppor = &utls.SupportedVersionsExtension{
			Versions: versions,
		}
	case utls.VersionTLS12:
		tlsMaxVersion = utls.VersionTLS12
		tlsMinVersion = utls.VersionTLS11
		versions := []uint16{}
		if !disableGrease {
			versions = append(versions, utls.GREASE_PLACEHOLDER)
		}
		versions = append(versions, utls.VersionTLS12, utls.VersionTLS11)
		tlsSuppor = &utls.SupportedVersionsExtension{
			Versions: versions,
		}
	case utls.VersionTLS11:
		tlsMaxVersion = utls.VersionTLS11
		tlsMinVersion = utls.VersionTLS10
		versions := []uint16{}
		if !disableGrease {
			versions = append(versions, utls.GREASE_PLACEHOLDER)
		}
		versions = append(versions, utls.VersionTLS11, utls.VersionTLS10)
		tlsSuppor = &utls.SupportedVersionsExtension{
			Versions: versions,
		}
	default:
		err = errors.New("ja3Str tls version error")
	}
	return
}

func genMap(disableGrease bool) (extMap map[string]utls.TLSExtension) {
	extMap = map[string]utls.TLSExtension{
		"0": &utls.SNIExtension{},
		"5": &utls.StatusRequestExtension{},
		// These are applied later
		// "10": &tls.SupportedCurvesExtension{...}
		// "11": &tls.SupportedPointsExtension{...}
		"13": &utls.SignatureAlgorithmsExtension{
			SupportedSignatureAlgorithms: []utls.SignatureScheme{
				utls.ECDSAWithP256AndSHA256,
				utls.ECDSAWithP384AndSHA384,
				utls.ECDSAWithP521AndSHA512,
				utls.PSSWithSHA256,
				utls.PSSWithSHA384,
				utls.PSSWithSHA512,
				utls.PKCS1WithSHA256,
				utls.PKCS1WithSHA384,
				utls.PKCS1WithSHA512,
				utls.ECDSAWithSHA1,
				utls.PKCS1WithSHA1,
			},
		},
		"16": &utls.ALPNExtension{
			AlpnProtocols: []string{"h2", "http/1.1"},
		},
		"17": &utls.GenericExtension{Id: 17}, // status_request_v2
		"18": &utls.SCTExtension{},
		"21": &utls.UtlsPaddingExtension{GetPaddingLen: utls.BoringPaddingStyle},
		"22": &utls.GenericExtension{Id: 22}, // encrypt_then_mac
		"23": &utls.ExtendedMasterSecretExtension{},
		"24": &utls.FakeTokenBindingExtension{},
		"27": &utls.UtlsCompressCertExtension{
			Algorithms: []utls.CertCompressionAlgo{utls.CertCompressionBrotli},
		},
		"28": &utls.FakeRecordSizeLimitExtension{
			Limit: 0x4001,
		}, //Limit: 0x4001
		"34": &utls.DelegatedCredentialsExtension{
			SupportedSignatureAlgorithms: []utls.SignatureScheme{
				utls.ECDSAWithP256AndSHA256,
				utls.ECDSAWithP384AndSHA384,
				utls.ECDSAWithP521AndSHA512,
				utls.ECDSAWithSHA1,
			},
		},
		"35": &utls.SessionTicketExtension{},
		"41": &utls.UtlsPreSharedKeyExtension{}, // PSK extension
		// "43": &utls.SupportedVersionsExtension{Versions: []uint16{ this gets set above
		// 	utls.VersionTLS13,
		// 	utls.VersionTLS12,
		// }},
		"44": &utls.CookieExtension{},
		"45": &utls.PSKKeyExchangeModesExtension{Modes: []uint8{
			utls.PskModeDHE,
		}},
		"49": &utls.GenericExtension{Id: 49}, // post_handshake_auth
		"50": &utls.SignatureAlgorithmsCertExtension{
			SupportedSignatureAlgorithms: []utls.SignatureScheme{
				utls.ECDSAWithP256AndSHA256,
				utls.ECDSAWithP384AndSHA384,
				utls.ECDSAWithP521AndSHA512,
				utls.PSSWithSHA256,
				utls.PSSWithSHA384,
				utls.PSSWithSHA512,
				utls.PKCS1WithSHA256,
				utls.PKCS1WithSHA384,
				utls.SignatureScheme(0x0806),
				utls.SignatureScheme(0x0601),
			},
		}, // signature_algorithms_cert
		"51": func() *utls.KeyShareExtension {
			if disableGrease {
				return &utls.KeyShareExtension{KeyShares: []utls.KeyShare{
					{Group: utls.X25519},
					// {Group: utls.CurveP384}, known bug missing correct extensions for handshake
				}}
			} else {
				return &utls.KeyShareExtension{KeyShares: []utls.KeyShare{
					{Group: utls.CurveID(utls.GREASE_PLACEHOLDER), Data: []byte{0}},
					{Group: utls.X25519},
					// {Group: utls.CurveP384}, known bug missing correct extensions for handshake
				}}
			}
		}(),
		"57":    &utls.QUICTransportParametersExtension{},
		"13172": &utls.NPNExtension{},
		"17513": &utls.ApplicationSettingsExtension{
			SupportedProtocols: []string{
				"h2",
			},
		},
		"17613": &utls.GenericExtension{
			Id:   17613,
			Data: []byte{0x00, 0x03, 0x02, 0x68, 0x32},
		},
		"30032": &utls.GenericExtension{Id: 0x7550, Data: []byte{0}}, // Channel ID extension
		"65281": &utls.RenegotiationInfoExtension{
			Renegotiation: utls.RenegotiateOnceAsClient,
		},
		"65037": utls.BoringGREASEECH(),
	}
	return
}

// QUIC fingerprinting utilities
func CreateUQuicSpecFromFingerprint(quicFingerprint string) (*uquic.QUICSpec, error) {
	if quicFingerprint == "" {
		return nil, errors.New("empty QUIC fingerprint")
	}

	// Todo: we are using a default QUIC specification based on Chrome
	// In the future, this could be enhanced to parse the actual fingerprint
	// and create a custom specification once I find a route to test against
	spec, err := uquic.QUICID2Spec(uquic.QUICChrome_115)
	if err != nil {
		return nil, fmt.Errorf("failed to create QUIC spec: %w", err)
	}

	return &spec, nil
}

// QUICStringToSpec creates a ClientHelloSpec based on a QUIC fingerprint string
func QUICStringToSpec(quicFingerprint string, userAgent string, forceHTTP1 bool) (*utls.ClientHelloSpec, error) {
	if quicFingerprint == "" {
		return nil, errors.New("empty QUIC fingerprint")
	}

	parsedUserAgent := parseUserAgent(userAgent)
	extMap := genMap(false)

	// Default to TLS 1.3 for QUIC (as QUIC typically uses TLS 1.3)
	var tlsVersion uint16 = utls.VersionTLS13

	// Create TLS configuration for QUIC
	tlsMaxVersion, tlsMinVersion, tlsExtension, err := createTlsVersion(tlsVersion, false)
	if err != nil {
		return nil, err
	}
	extMap["43"] = tlsExtension

	// QUIC-specific cipher suites (TLS 1.3 only)
	var suites []uint16
	if parsedUserAgent.UserAgent == chrome {
		suites = append(suites, utls.GREASE_PLACEHOLDER)
	}

	// Add TLS 1.3 cipher suites commonly used with QUIC
	suites = append(suites, []uint16{
		utls.TLS_AES_128_GCM_SHA256,
		utls.TLS_AES_256_GCM_SHA384,
		utls.TLS_CHACHA20_POLY1305_SHA256,
		utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	}...)

	// Set up curves for QUIC
	var targetCurves []utls.CurveID
	if parsedUserAgent.UserAgent == chrome {
		targetCurves = append(targetCurves, utls.CurveID(utls.GREASE_PLACEHOLDER))
	}

	// Add common curves for QUIC
	targetCurves = append(targetCurves, []utls.CurveID{
		utls.X25519,
		utls.CurveP256,
		utls.CurveP384,
		utls.CurveP521,
	}...)
	extMap["10"] = &utls.SupportedCurvesExtension{Curves: targetCurves}

	// Set point formats
	extMap["11"] = &utls.SupportedPointsExtension{SupportedPoints: []byte{0}}

	// Force HTTP/1.1 if requested (though this is unusual for QUIC)
	if forceHTTP1 {
		extMap["16"] = &utls.ALPNExtension{
			AlpnProtocols: []string{"http/1.1"},
		}
	} else {
		// Default ALPN protocols for QUIC/HTTP3
		extMap["16"] = &utls.ALPNExtension{
			AlpnProtocols: []string{"h3", "h3-29", "h3-28", "h3-27"},
		}
	}

	// Add QUIC transport parameters extension (critical for QUIC)
	extMap["57"] = &utls.QUICTransportParametersExtension{}

	// Build extensions list with QUIC-appropriate extensions
	var exts []utls.TLSExtension
	if parsedUserAgent.UserAgent == chrome {
		exts = append(exts, &utls.UtlsGREASEExtension{})
	}

	// QUIC-specific extension order
	quicExtensions := []string{"0", "23", "65281", "10", "11", "35", "16", "5", "51", "43", "13", "45", "28", "57", "21"}
	for _, e := range quicExtensions {
		if te, ok := extMap[e]; ok {
			if e == "21" && parsedUserAgent.UserAgent == chrome {
				exts = append(exts, &utls.UtlsGREASEExtension{})
			}
			exts = append(exts, te)
		}
	}

	return &utls.ClientHelloSpec{
		TLSVersMin:         tlsMinVersion,
		TLSVersMax:         tlsMaxVersion,
		CipherSuites:       suites,
		CompressionMethods: []byte{0},
		Extensions:         exts,
		GetSessionID:       sha256.Sum256,
	}, nil
}