package cycletls

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"io"
	"errors"
	"github.com/andybalholm/brotli"
	utls "github.com/refraction-networking/utls"
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

// DecompressBody unzips compressed data
func DecompressBody(Body []byte, encoding []string, content []string) (parsedBody string) {
	if len(encoding) > 0 {
		if encoding[0] == "gzip" {
			unz, err := gUnzipData(Body)
			if err != nil {
				return string(Body)
			}
			return string(unz)
		} else if encoding[0] == "deflate" {
			unz, err := enflateData(Body)
			if err != nil {
				return string(Body)
			}
			return string(unz)
		} else if encoding[0] == "br" {
			unz, err := unBrotliData(Body)
			if err != nil {
				return string(Body)
			}
			return string(unz)
		}
	} else if len(content) > 0 {
		decodingTypes := map[string]bool{
			"image/svg+xml":   true,
			"image/webp":      true,
			"image/jpeg":      true,
			"image/png":       true,
			"image/gif":       true,
			"image/avif":      true,
			"application/pdf": true,
		}
		if decodingTypes[content[0]] {
			return base64.StdEncoding.EncodeToString(Body)
		}
	}
	parsedBody = string(Body)
	return parsedBody

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
		"30032": &utls.GenericExtension{Id: 0x7550, Data: []byte{0}}, //FIXME
		"65281": &utls.RenegotiationInfoExtension{
			Renegotiation: utls.RenegotiateOnceAsClient,
		},
		"65037": utls.BoringGREASEECH(),
	}
	return

}

// PrettyStruct formats json
func PrettyStruct(data interface{}) (string, error) {
	val, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "", err
	}
	return string(val), nil
}
