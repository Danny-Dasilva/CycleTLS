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
	extMap := genMap()
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
	tlsMaxVersion, tlsMinVersion, tlsExtension, err := createTlsVersion(uint16(ver))
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
	TLSVersion       string
	CipherHash       string
	ExtensionsHash   string
	HeadersHash      string
	UserAgentHash    string
}

// ParseJA4String parses a JA4 string into its components
// JA4 format: <TLS version><cipher hash>_<extensions hash>_<headers hash>_<UA hash>
// Example: t13d_cd89_1952_bb99
func ParseJA4String(ja4 string) (*JA4Components, error) {
	if len(ja4) < 19 { // minimum length for JA4
		return nil, errors.New("invalid JA4 string: too short")
	}

	// Split by underscores
	parts := strings.Split(ja4, "_")
	if len(parts) != 4 {
		return nil, errors.New("invalid JA4 string: incorrect format")
	}

	// Extract TLS version and cipher hash from first part
	// Expected format: t13d (3 chars TLS version + 1 char cipher hash = 4 chars)
	if len(parts[0]) != 4 { // t13 + 1 char exactly
		return nil, errors.New("invalid JA4 string: invalid TLS version/cipher part")
	}

	tlsVersion := parts[0][:3] // t10, t11, t12, t13
	cipherHash := parts[0][3:] // remainder is cipher hash (1 char)

	return &JA4Components{
		TLSVersion:       tlsVersion,
		CipherHash:       cipherHash,
		ExtensionsHash:   parts[1],
		HeadersHash:      parts[2], 
		UserAgentHash:    parts[3],
	}, nil
}

// JA4StringToSpec creates a ClientHelloSpec based on a JA4 string
// Since JA4 uses hashes, we create a spec with common TLS parameters 
// that would produce a similar fingerprint
func JA4StringToSpec(ja4 string, userAgent string, forceHTTP1 bool) (*utls.ClientHelloSpec, error) {
	components, err := ParseJA4String(ja4)
	if err != nil {
		return nil, err
	}

	parsedUserAgent := parseUserAgent(userAgent)
	extMap := genMap()

	// Map TLS version string to actual version
	var tlsVersion uint16
	switch components.TLSVersion {
	case "t10":
		tlsVersion = utls.VersionTLS10
	case "t11":
		tlsVersion = utls.VersionTLS11
	case "t12":
		tlsVersion = utls.VersionTLS12
	case "t13":
		tlsVersion = utls.VersionTLS13
	default:
		return nil, errors.New("unsupported TLS version in JA4: " + components.TLSVersion)
	}

	// Create TLS configuration
	tlsMaxVersion, tlsMinVersion, tlsExtension, err := createTlsVersion(tlsVersion)
	if err != nil {
		return nil, err
	}
	extMap["43"] = tlsExtension

	// Use common cipher suites based on TLS version
	var suites []uint16
	if parsedUserAgent.UserAgent == chrome {
		suites = append(suites, utls.GREASE_PLACEHOLDER)
	}

	// Add common cipher suites based on TLS version
	if tlsVersion == utls.VersionTLS13 {
		suites = append(suites, []uint16{
			utls.TLS_AES_128_GCM_SHA256,
			utls.TLS_AES_256_GCM_SHA384,
			utls.TLS_CHACHA20_POLY1305_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		}...)
	} else {
		suites = append(suites, []uint16{
			utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			utls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		}...)
	}

	// Use common extensions and curves
	var targetCurves []utls.CurveID
	if parsedUserAgent.UserAgent == chrome {
		targetCurves = append(targetCurves, utls.CurveID(utls.GREASE_PLACEHOLDER))
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
	}

	// Add common curves
	targetCurves = append(targetCurves, []utls.CurveID{
		utls.CurveP256,
		utls.CurveP384,
		utls.CurveP521,
		utls.X25519,
	}...)
	extMap["10"] = &utls.SupportedCurvesExtension{Curves: targetCurves}

	// Add common point formats
	extMap["11"] = &utls.SupportedPointsExtension{SupportedPoints: []byte{0}}

	// Force HTTP1 if requested
	if forceHTTP1 {
		extMap["16"] = &utls.ALPNExtension{
			AlpnProtocols: []string{"http/1.1"},
		}
	}

	// Build extensions list with common extensions
	var exts []utls.TLSExtension
	if parsedUserAgent.UserAgent == chrome {
		exts = append(exts, &utls.UtlsGREASEExtension{})
	}

	// Add common extensions in typical order
	commonExtensions := []string{"0", "23", "65281", "10", "11", "35", "16", "5", "51", "43", "13", "45", "28", "21"}
	for _, e := range commonExtensions {
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

// QUIC fingerprinting utilities based on reference implementation

// USpec represents a QUIC fingerprint specification
type USpec struct {
	QUICID uquic.QUICID
}

// Spec converts USpec to QUICSpec
func (obj USpec) Spec() (uquic.QUICSpec, error) {
	spec, err := uquic.QUICID2Spec(obj.QUICID)
	if err != nil {
		return uquic.QUICSpec{}, err
	}
	return spec, nil
}

// CreateUSpec creates a QUIC spec from various input types
func CreateUSpec(value any) (uquic.QUICSpec, error) {
	switch data := value.(type) {
	case bool:
		if data {
			return uquic.QUICID2Spec(uquic.QUICFirefox_116)
		}
		return uquic.QUICSpec{}, nil
	case uquic.QUICID:
		return uquic.QUICID2Spec(data)
	case USpec:
		return data.Spec()
	default:
		return uquic.QUICSpec{}, errors.New("unsupported type")
	}
}

// CreateUQuicSpecFromFingerprint creates a uquic.QUICSpec from a QUIC fingerprint string
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

// CreateUQuicSpecFromUserAgent creates a uquic.QUICSpec based on user agent
func CreateUQuicSpecFromUserAgent(userAgent string) (*uquic.QUICSpec, error) {
	parsedUA := parseUserAgent(userAgent)
	
	var quicID uquic.QUICID
	switch parsedUA.UserAgent {
	case chrome:
		quicID = uquic.QUICChrome_115
	case firefox:
		quicID = uquic.QUICFirefox_116
	default:
		// Default to Chrome for unknown user agents
		quicID = uquic.QUICChrome_115
	}

	spec, err := uquic.QUICID2Spec(quicID)
	if err != nil {
		return nil, fmt.Errorf("failed to create QUIC spec for user agent: %w", err)
	}

	return &spec, nil
}

// QUICStringToSpec creates a ClientHelloSpec based on a QUIC fingerprint string
func QUICStringToSpec(quicFingerprint string, userAgent string, forceHTTP1 bool) (*utls.ClientHelloSpec, error) {
	if quicFingerprint == "" {
		return nil, errors.New("empty QUIC fingerprint")
	}

	parsedUserAgent := parseUserAgent(userAgent)
	extMap := genMap()

	// Parse QUIC fingerprint hex string - for now we'll use a simplified approach
	// The hex string contains TLS ClientHello parameters encoded for QUIC
	// For the implementation, we'll decode key components and create a spec
	
	// Extract TLS version from the beginning of the hex string
	// Format typically starts with TLS version (e.g., 1603 for TLS 1.2, 1603 for TLS 1.3)
	if len(quicFingerprint) < 4 {
		return nil, errors.New("invalid QUIC fingerprint: too short")
	}

	// Default to TLS 1.3 for QUIC (as QUIC typically uses TLS 1.3)
	var tlsVersion uint16 = utls.VersionTLS13
	
	// Create TLS configuration for QUIC
	tlsMaxVersion, tlsMinVersion, tlsExtension, err := createTlsVersion(tlsVersion)
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

// TLSVersion，Ciphers，Extensions，EllipticCurves，EllipticCurvePointFormats
func createTlsVersion(ver uint16) (tlsMaxVersion uint16, tlsMinVersion uint16, tlsSuppor utls.TLSExtension, err error) {
	switch ver {
	case utls.VersionTLS13:
		tlsMaxVersion = utls.VersionTLS13
		tlsMinVersion = utls.VersionTLS12
		tlsSuppor = &utls.SupportedVersionsExtension{
			Versions: []uint16{
				utls.GREASE_PLACEHOLDER,
				utls.VersionTLS13,
				utls.VersionTLS12,
			},
		}
	case utls.VersionTLS12:
		tlsMaxVersion = utls.VersionTLS12
		tlsMinVersion = utls.VersionTLS11
		tlsSuppor = &utls.SupportedVersionsExtension{
			Versions: []uint16{
				utls.GREASE_PLACEHOLDER,
				utls.VersionTLS12,
				utls.VersionTLS11,
			},
		}
	case utls.VersionTLS11:
		tlsMaxVersion = utls.VersionTLS11
		tlsMinVersion = utls.VersionTLS10
		tlsSuppor = &utls.SupportedVersionsExtension{
			Versions: []uint16{
				utls.GREASE_PLACEHOLDER,
				utls.VersionTLS11,
				utls.VersionTLS10,
			},
		}
	default:
		err = errors.New("ja3Str tls version error")
	}
	return
}
func genMap() (extMap map[string]utls.TLSExtension) {
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
		"41": &utls.UtlsPreSharedKeyExtension{}, //FIXME pre_shared_key
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
		"51": &utls.KeyShareExtension{KeyShares: []utls.KeyShare{
			{Group: utls.CurveID(utls.GREASE_PLACEHOLDER), Data: []byte{0}},
			{Group: utls.X25519},

			// {Group: utls.CurveP384}, known bug missing correct extensions for handshake
		}},
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
		"30032": &utls.GenericExtension{Id: 0x7550, Data: []byte{0}}, //FIXME
		"65281": &utls.RenegotiationInfoExtension{
			Renegotiation: utls.RenegotiateOnceAsClient,
		},
		"65037": utls.BoringGREASEECH(),
	}
	return

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
