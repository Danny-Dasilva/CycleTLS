package cycletls

import (
	"encoding/binary"
	"strconv"

	utls "github.com/refraction-networking/utls"
)

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
func CreateExtensionFromID(extID uint16, tlsVersion uint16, components *JA4RComponents, disableGrease bool, serverName string) utls.TLSExtension {
	switch extID {
	case 0x0000: // Server Name Indication
		return &utls.SNIExtension{
			ServerName: serverName,
		}
	case 0x0005: // Status Request
		return &utls.StatusRequestExtension{}
	case 0x000a: // Supported Groups (Elliptic Curves)
		curves := []utls.CurveID{utls.X25519, utls.CurveP256, utls.CurveP384}
		// Add post-quantum curves if supported
		if tlsVersion == utls.VersionTLS13 {
			// Add X25519MLKEM768 if supported by uTLS version
			curves = append([]utls.CurveID{utls.X25519MLKEM768}, curves...)
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
				sigSchemes = append(sigSchemes, utls.SignatureScheme(rawSig))
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
	case 0x0015: // Padding
		return &utls.UtlsPaddingExtension{GetPaddingLen: ja4rPaddingStyle}
	case 0x0017: // Extended Master Secret
		return &utls.ExtendedMasterSecretExtension{}
	case extensionTrustAnchors: // Trust Anchor Identifiers - 51764
		return newTrustAnchorsExtension()
	case 0x001b: // Compress Certificate
		// Use the uTLS type, not a GenericExtension: uTLS only accepts the
		// CompressedCertificate message that servers send in reply (Cloudflare
		// does) when this extension has registered its algorithms on the conn.
		// The bytes on the wire are the same.
		return &utls.UtlsCompressCertExtension{Algorithms: []utls.CertCompressionAlgo{
			utls.CertCompressionBrotli,
		}}
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
				{Group: utls.X25519MLKEM768},
				{Group: utls.X25519},
			}
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
		// Sent even when disableGrease is set. 0xfe0d is a real extension ID
		// that JA4 counts and hashes, so dropping it would change the
		// fingerprint the caller asked for. The JA3 path (genMap) also sends it
		// unconditionally.
		return utls.BoringGREASEECH()
	case 0xff01: // Renegotiation Info - 65281
		return &utls.RenegotiationInfoExtension{
			Renegotiation: utls.RenegotiateOnceAsClient,
		}
	default:
		// Handle GREASE values
		if IsGREASEValue(extID) && !disableGrease {
			return NewCustomGREASEExtension(extID)
		}
		// IDs the JA3 path knows a valid body for (status_request_v2,
		// token_binding, pre_shared_key, cookie, signature_algorithms_cert,
		// channel_id, ...) use that body. A zero-length body is not a valid
		// encoding for them.
		if ext, ok := genMap(disableGrease)[strconv.Itoa(int(extID))]; ok {
			return ext
		}
		// Unknown extensions: preserve as generic with original ID
		return &utls.GenericExtension{
			Id:   extID,
			Data: []byte{}, // Empty data for unknown extensions
		}
	}
}

// ja4rPaddingStyle is the padding rule for a JA4_r string that lists the
// padding extension (0x0015). BoringSSL pads only when the unpadded hello is
// 256 to 511 bytes long and sends no padding extension otherwise. A JA4_r that
// lists 0x0015 counts it and hashes it, so the extension is always sent: with
// the BoringSSL length when that rule applies, and with one zero byte when it
// does not.
func ja4rPaddingStyle(unpaddedLen int) (int, bool) {
	if n, ok := utls.BoringPaddingStyle(unpaddedLen); ok && n > 0 {
		return n, true
	}
	return 1, true
}

// movePreSharedKeyLast moves the pre_shared_key extension to the end of the
// list, where RFC 8446 section 4.2.11 requires it.
func movePreSharedKeyLast(exts []utls.TLSExtension) []utls.TLSExtension {
	out := make([]utls.TLSExtension, 0, len(exts))
	var psk []utls.TLSExtension
	for _, e := range exts {
		if _, ok := e.(utls.PreSharedKeyExtension); ok {
			psk = append(psk, e)
			continue
		}
		out = append(out, e)
	}
	return append(out, psk...)
}
