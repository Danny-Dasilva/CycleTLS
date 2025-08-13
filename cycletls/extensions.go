package cycletls

import (
	"encoding/binary"
	"fmt"
	utls "github.com/refraction-networking/utls"
)

var supportedSignatureAlgorithmsExtensions = map[string]utls.SignatureScheme{
	"PKCS1WithSHA256":                     utls.PKCS1WithSHA256,
	"PKCS1WithSHA384":                     utls.PKCS1WithSHA384,
	"PKCS1WithSHA512":                     utls.PKCS1WithSHA512,
	"PSSWithSHA256":                       utls.PSSWithSHA256,
	"PSSWithSHA384":                       utls.PSSWithSHA384,
	"PSSWithSHA512":                       utls.PSSWithSHA512,
	"ECDSAWithP256AndSHA256":              utls.ECDSAWithP256AndSHA256,
	"ECDSAWithP384AndSHA384":              utls.ECDSAWithP384AndSHA384,
	"ECDSAWithP521AndSHA512":              utls.ECDSAWithP521AndSHA512,
	"Ed25519":                             utls.Ed25519,
	"PKCS1WithSHA1":                       utls.PKCS1WithSHA1,
	"ECDSAWithSHA1":                       utls.ECDSAWithSHA1,
	"rsa_pkcs1_sha1":                      utls.SignatureScheme(0x0201),
	"Reserved for backward compatibility": utls.SignatureScheme(0x0202),
	"ecdsa_sha1":                          utls.SignatureScheme(0x0203),
	"rsa_pkcs1_sha256":                    utls.SignatureScheme(0x0401),
	"ecdsa_secp256r1_sha256":              utls.SignatureScheme(0x0403),
	"rsa_pkcs1_sha256_legacy":             utls.SignatureScheme(0x0420),
	"rsa_pkcs1_sha384":                    utls.SignatureScheme(0x0501),
	"ecdsa_secp384r1_sha384":              utls.SignatureScheme(0x0503),
	"rsa_pkcs1_sha384_legacy":             utls.SignatureScheme(0x0520),
	"rsa_pkcs1_sha512":                    utls.SignatureScheme(0x0601),
	"ecdsa_secp521r1_sha512":              utls.SignatureScheme(0x0603),
	"rsa_pkcs1_sha512_legacy":             utls.SignatureScheme(0x0620),
	"eccsi_sha256":                        utls.SignatureScheme(0x0704),
	"iso_ibs1":                            utls.SignatureScheme(0x0705),
	"iso_ibs2":                            utls.SignatureScheme(0x0706),
	"iso_chinese_ibs":                     utls.SignatureScheme(0x0707),
	"sm2sig_sm3":                          utls.SignatureScheme(0x0708),
	"gostr34102012_256a":                  utls.SignatureScheme(0x0709),
	"gostr34102012_256b":                  utls.SignatureScheme(0x070A),
	"gostr34102012_256c":                  utls.SignatureScheme(0x070B),
	"gostr34102012_256d":                  utls.SignatureScheme(0x070C),
	"gostr34102012_512a":                  utls.SignatureScheme(0x070D),
	"gostr34102012_512b":                  utls.SignatureScheme(0x070E),
	"gostr34102012_512c":                  utls.SignatureScheme(0x070F),
	"rsa_pss_rsae_sha256":                 utls.SignatureScheme(0x0804),
	"rsa_pss_rsae_sha384":                 utls.SignatureScheme(0x0805),
	"rsa_pss_rsae_sha512":                 utls.SignatureScheme(0x0806),
	"ed25519":                             utls.SignatureScheme(0x0807),
	"ed448":                               utls.SignatureScheme(0x0808),
	"rsa_pss_pss_sha256":                  utls.SignatureScheme(0x0809),
	"rsa_pss_pss_sha384":                  utls.SignatureScheme(0x080A),
	"rsa_pss_pss_sha512":                  utls.SignatureScheme(0x080B),
	"ecdsa_brainpoolP256r1tls13_sha256":   utls.SignatureScheme(0x081A),
	"ecdsa_brainpoolP384r1tls13_sha384":   utls.SignatureScheme(0x081B),
	"ecdsa_brainpoolP512r1tls13_sha512":   utls.SignatureScheme(0x081C),
}

// PreserveIDExtension is an interface for extensions that preserve their original extension ID
type PreserveIDExtension interface {
	utls.TLSExtension
	GetPreservedID() uint16
}

// CustomApplicationSettingsExtension preserves the original extension ID for ALPS
type CustomApplicationSettingsExtension struct {
	*utls.GenericExtension
	OriginalID         uint16
	SupportedProtocols []string
}

// NewCustomApplicationSettingsExtension creates a new ALPS extension with preserved ID
func NewCustomApplicationSettingsExtension(extID uint16, protocols []string) *CustomApplicationSettingsExtension {
	// Build the extension data according to ALPS specification
	// Format: length (2 bytes) + protocol_list
	var data []byte

	// Build protocol list: each protocol is length-prefixed
	var protocolData []byte
	for _, protocol := range protocols {
		protocolData = append(protocolData, byte(len(protocol)))
		protocolData = append(protocolData, []byte(protocol)...)
	}

	// Add total length prefix (2 bytes)
	lengthBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(lengthBytes, uint16(len(protocolData)))
	data = append(data, lengthBytes...)
	data = append(data, protocolData...)

	return &CustomApplicationSettingsExtension{
		GenericExtension: &utls.GenericExtension{
			Id:   extID,
			Data: data,
		},
		OriginalID:         extID,
		SupportedProtocols: protocols,
	}
}

// GetPreservedID returns the original extension ID
func (c *CustomApplicationSettingsExtension) GetPreservedID() uint16 {
	return c.OriginalID
}

// CustomECHExtension preserves the original extension ID for Encrypted Client Hello
type CustomECHExtension struct {
	*utls.GenericExtension
	OriginalID uint16
}

// NewCustomECHExtension creates a new ECH extension with preserved ID
func NewCustomECHExtension(extID uint16) *CustomECHExtension {
	// ECH extension data - using a simple placeholder for now
	// In a full implementation, this would contain actual ECH configuration
	data := []byte{
		0x00, 0x00, // config_id
		0x00, 0x00, // kem_id placeholder
		0x00, 0x00, // public_key length
		// public_key would follow
		0x00, 0x00, // cipher_suites length
		// cipher_suites would follow
	}

	return &CustomECHExtension{
		GenericExtension: &utls.GenericExtension{
			Id:   extID,
			Data: data,
		},
		OriginalID: extID,
	}
}

// GetPreservedID returns the original extension ID
func (c *CustomECHExtension) GetPreservedID() uint16 {
	return c.OriginalID
}

// CustomCompressCertificateExtension preserves the original extension ID
type CustomCompressCertificateExtension struct {
	*utls.GenericExtension
	OriginalID uint16
	Algorithms []utls.CertCompressionAlgo
}

// NewCustomCompressCertificateExtension creates a new compress certificate extension
func NewCustomCompressCertificateExtension(extID uint16, algorithms []utls.CertCompressionAlgo) *CustomCompressCertificateExtension {
	// Build extension data: algorithm list
	var data []byte
	data = append(data, byte(len(algorithms)*2)) // length of algorithms list

	for _, algo := range algorithms {
		algoBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(algoBytes, uint16(algo))
		data = append(data, algoBytes...)
	}

	return &CustomCompressCertificateExtension{
		GenericExtension: &utls.GenericExtension{
			Id:   extID,
			Data: data,
		},
		OriginalID: extID,
		Algorithms: algorithms,
	}
}

// GetPreservedID returns the original extension ID
func (c *CustomCompressCertificateExtension) GetPreservedID() uint16 {
	return c.OriginalID
}

// CustomRecordSizeLimitExtension preserves the original extension ID
type CustomRecordSizeLimitExtension struct {
	*utls.GenericExtension
	OriginalID uint16
	Limit      uint16
}

// NewCustomRecordSizeLimitExtension creates a new record size limit extension
func NewCustomRecordSizeLimitExtension(extID uint16, limit uint16) *CustomRecordSizeLimitExtension {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, limit)

	return &CustomRecordSizeLimitExtension{
		GenericExtension: &utls.GenericExtension{
			Id:   extID,
			Data: data,
		},
		OriginalID: extID,
		Limit:      limit,
	}
}

// CustomDelegatedCredentialsExtension preserves the original extension ID
type CustomDelegatedCredentialsExtension struct {
	*utls.GenericExtension
	OriginalID          uint16
	SignatureAlgorithms []utls.SignatureScheme
}

// NewCustomDelegatedCredentialsExtension creates a new delegated credentials extension
func NewCustomDelegatedCredentialsExtension(extID uint16, algorithms []utls.SignatureScheme) *CustomDelegatedCredentialsExtension {
	// Build extension data: signature algorithms list
	var data []byte
	data = append(data, byte(len(algorithms)*2)) // length of algorithms list

	for _, algo := range algorithms {
		algoBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(algoBytes, uint16(algo))
		data = append(data, algoBytes...)
	}

	return &CustomDelegatedCredentialsExtension{
		GenericExtension: &utls.GenericExtension{
			Id:   extID,
			Data: data,
		},
		OriginalID:          extID,
		SignatureAlgorithms: algorithms,
	}
}

// GetPreservedID returns the original extension ID
func (c *CustomDelegatedCredentialsExtension) GetPreservedID() uint16 {
	return c.OriginalID
}

// GetPreservedID returns the original extension ID
func (c *CustomRecordSizeLimitExtension) GetPreservedID() uint16 {
	return c.OriginalID
}

// CustomPostQuantumExtension for post-quantum key exchange extensions
type CustomPostQuantumExtension struct {
	*utls.GenericExtension
	OriginalID       uint16
	KeyExchangeGroup uint16
}

// NewCustomPostQuantumExtension creates a new post-quantum extension
func NewCustomPostQuantumExtension(extID uint16, group uint16) *CustomPostQuantumExtension {
	// Simple placeholder data for post-quantum extension
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[0:2], group)  // key exchange group
	binary.BigEndian.PutUint16(data[2:4], 0x0000) // placeholder for additional data

	return &CustomPostQuantumExtension{
		GenericExtension: &utls.GenericExtension{
			Id:   extID,
			Data: data,
		},
		OriginalID:       extID,
		KeyExchangeGroup: group,
	}
}

// GetPreservedID returns the original extension ID
func (c *CustomPostQuantumExtension) GetPreservedID() uint16 {
	return c.OriginalID
}

// CustomGREASEExtension preserves GREASE extension IDs
type CustomGREASEExtension struct {
	*utls.GenericExtension
	OriginalID uint16
}

// NewCustomGREASEExtension creates a new GREASE extension with preserved ID
func NewCustomGREASEExtension(extID uint16) *CustomGREASEExtension {
	// GREASE extensions typically have empty or minimal data
	data := []byte{}

	return &CustomGREASEExtension{
		GenericExtension: &utls.GenericExtension{
			Id:   extID,
			Data: data,
		},
		OriginalID: extID,
	}
}

// GetPreservedID returns the original extension ID
func (c *CustomGREASEExtension) GetPreservedID() uint16 {
	return c.OriginalID
}

// IsGREASEValue checks if an extension ID is a GREASE value
func IsGREASEValue(extID uint16) bool {
	// GREASE values follow the pattern 0x?A?A where ? can be any hex digit
	return (extID & 0x0f0f) == 0x0a0a
}

// CreateExtensionFromID creates an appropriate extension for the given ID
func CreateExtensionFromID(extID uint16, tlsVersion uint16, components *JA4RComponents, disableGrease bool) utls.TLSExtension {
	switch extID {
	case 0x0000: // Server Name Indication
		return &utls.SNIExtension{}
	case 0x0005: // Status Request
		return &utls.StatusRequestExtension{}
	case 0x000a: // Supported Groups (Elliptic Curves)
		curves := []utls.CurveID{utls.X25519, utls.CurveP256, utls.CurveP384}
		// Add post-quantum curves if supported
		if tlsVersion == utls.VersionTLS13 {
			// Add X25519MLKEM768 if supported by uTLS version
			curves = append([]utls.CurveID{utls.X25519}, curves...)
		}
		return &utls.SupportedCurvesExtension{Curves: curves}
	case 0x000b: // EC Point Formats
		return &utls.SupportedPointsExtension{
			SupportedPoints: []byte{0}, // uncompressed
		}
	case 0x000d: // Signature Algorithms
		sigSchemes := []utls.SignatureScheme{}
		if components != nil {
			for _, rawSig := range components.SignatureSchemes {
				if mappedSig, exists := supportedSignatureAlgorithmsExtensions[fmt.Sprintf("0x%04x", rawSig)]; exists {
					sigSchemes = append(sigSchemes, mappedSig)
				} else {
					// Use the raw value as a signature scheme
					sigSchemes = append(sigSchemes, utls.SignatureScheme(rawSig))
				}
			}
		}
		if len(sigSchemes) == 0 {
			// Default signature algorithms if none provided
			sigSchemes = []utls.SignatureScheme{
				utls.ECDSAWithP256AndSHA256,
				utls.ECDSAWithP384AndSHA384,
				utls.ECDSAWithP521AndSHA512,
				utls.PSSWithSHA256,
				utls.PSSWithSHA384,
				utls.PSSWithSHA512,
				utls.PKCS1WithSHA256,
				utls.PKCS1WithSHA384,
				utls.PKCS1WithSHA512,
			}
		}
		return &utls.SignatureAlgorithmsExtension{
			SupportedSignatureAlgorithms: sigSchemes,
		}
	case 0x0010: // ALPN
		alpnProtocols := []string{"h2", "http/1.1"}
		if components != nil {
			
			switch components.ALPN {
			case "h2":
				alpnProtocols = []string{"h2", "http/1.1"}
			case "h1":
				alpnProtocols = []string{"http/1.1"}
			case "h3":
				alpnProtocols = []string{"h3", "h2", "http/1.1"}
			}
		}
		
		return &utls.ALPNExtension{
			AlpnProtocols: alpnProtocols,
		}
	case 0x0012: // Signed Certificate Timestamp
		return &utls.SCTExtension{}
	case 0x0017: // Extended Master Secret
		return &utls.ExtendedMasterSecretExtension{}
	case 0x001b: // Compress Certificate
		return NewCustomCompressCertificateExtension(extID, []utls.CertCompressionAlgo{
			utls.CertCompressionBrotli,
		})
	case 0x001c: // Record Size Limit
		return NewCustomRecordSizeLimitExtension(extID, 0x4001)
	case 0x0022: // Delegated Credentials - PROBLEMATIC EXTENSION
		// This extension causes connection resets with some servers (like peet.ws)
		// Instead of implementing the complex RFC format, use a simpler fallback
		// that maintains compatibility but avoids server-side rejections
		return &utls.GenericExtension{
			Id:   extID,
			Data: []byte{0x00, 0x04, 0x04, 0x03, 0x08, 0x04}, // Minimal valid data
		}
	case 0x0023: // Session Ticket
		return &utls.SessionTicketExtension{}
	case 0x002b: // Supported Versions
		if tlsVersion == utls.VersionTLS13 {
			return &utls.SupportedVersionsExtension{
				Versions: []uint16{utls.VersionTLS13, utls.VersionTLS12},
			}
		} else if tlsVersion == utls.VersionTLS12 {
			return &utls.SupportedVersionsExtension{
				Versions: []uint16{utls.VersionTLS12, utls.VersionTLS11},
			}
		}
		return nil
	case 0x002d: // PSK Key Exchange Modes
		return &utls.PSKKeyExchangeModesExtension{
			Modes: []uint8{utls.PskModeDHE},
		}
	case 0x0033: // Key Share
		if tlsVersion == utls.VersionTLS13 {
			keyShares := []utls.KeyShare{
				{Group: utls.X25519},
				{Group: utls.CurveP256},
			}
			// Add post-quantum key share if supported
			return &utls.KeyShareExtension{
				KeyShares: keyShares,
			}
		}
		return nil
	case 0x4469: // Old ALPS (ApplicationSettings) - 17513
		return NewCustomApplicationSettingsExtension(extID, []string{"h2"})
	case 0x44cd: // New ALPS (ApplicationSettings) - 17613
		return NewCustomApplicationSettingsExtension(extID, []string{"h2"})
	case 0x6399: // X25519Kyber768Draft00 (Post-Quantum) - 25497
		return NewCustomPostQuantumExtension(extID, 0x6399)
	case 0xfe0d: // Encrypted Client Hello (ECH) - 65037
		return NewCustomECHExtension(extID)
	case 0xff01: // Renegotiation Info - 65281
		return &utls.RenegotiationInfoExtension{
			Renegotiation: utls.RenegotiateOnceAsClient,
		}
	default:
		// Handle GREASE values
		if IsGREASEValue(extID) && !disableGrease {
			return NewCustomGREASEExtension(extID)
		}
		// Unknown extensions: preserve as generic with original ID
		return &utls.GenericExtension{
			Id:   extID,
			Data: []byte{}, // Empty data for unknown extensions
		}
	}
}
