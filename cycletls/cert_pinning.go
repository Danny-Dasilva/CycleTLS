package cycletls

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrCertPinMismatch is returned (wrapped) when certPins is set and no
// certificate the server presented matches a pin. Test with errors.Is.
var ErrCertPinMismatch = errors.New("certificate pin mismatch")

// errCertPinsHTTP3 is returned when certPins is set on an HTTP/3 request. The
// QUIC handshake does not go through dialTLS, so the pins could not be checked
// there; failing is better than sending the request unpinned.
var errCertPinsHTTP3 = errors.New("certPins is not supported with HTTP/3: the pins cannot be checked on a QUIC connection, so the request was not sent")

// errCertPinsWebSocket is the same refusal for WebSocket requests, which are
// dialed by gorilla/websocket with crypto/tls and never reach dialTLS.
var errCertPinsWebSocket = errors.New("certPins is not supported with WebSocket: the pins cannot be checked on a WebSocket connection, so it was not opened")

// spkiPinPrefix marks a pin as the hash of a SubjectPublicKeyInfo (HPKP form).
const spkiPinPrefix = "sha256/"

// certPin is one entry of certPins: a SHA-256 digest and what it is a digest of.
type certPin struct {
	digest [sha256.Size]byte
	// spki is true for a "sha256/<base64>" pin, which is matched against the
	// certificate's SubjectPublicKeyInfo. A hex pin is matched against the
	// whole certificate.
	spki bool
}

// parseCertPins decodes the certPins option. Any pin that cannot be decoded is
// an error: a pin list that is silently shortened would pin less than the
// caller asked for.
func parseCertPins(pins []string) ([]certPin, error) {
	if len(pins) == 0 {
		return nil, nil
	}
	parsed := make([]certPin, 0, len(pins))
	for i, raw := range pins {
		pin, err := parseCertPin(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid certPins[%d] %q: %w", i, raw, err)
		}
		parsed = append(parsed, pin)
	}
	return parsed, nil
}

// parseCertPin decodes one pin. There are two forms, and the form says what
// was hashed:
//
//	64 hex characters   SHA-256 of the whole certificate (DER), the meaning
//	                    certPins has had since it was added (CertPinConfig)
//	"sha256/<base64>"   SHA-256 of the certificate's SubjectPublicKeyInfo,
//	                    the HPKP pin-sha256 value
//
// Anything else is an error.
func parseCertPin(raw string) (certPin, error) {
	var pin certPin
	s := strings.TrimSpace(raw)

	if len(s) >= len(spkiPinPrefix) && strings.EqualFold(s[:len(spkiPinPrefix)], spkiPinPrefix) {
		decoded, err := base64.StdEncoding.DecodeString(s[len(spkiPinPrefix):])
		if err != nil {
			return pin, fmt.Errorf("the value after %q is not base64: %w", spkiPinPrefix, err)
		}
		if len(decoded) != sha256.Size {
			return pin, fmt.Errorf("the value after %q decodes to %d bytes, want %d (a SHA-256)", spkiPinPrefix, len(decoded), sha256.Size)
		}
		copy(pin.digest[:], decoded)
		pin.spki = true
		return pin, nil
	}

	if err := (&CertPinConfig{SHA256Pins: []string{s}}).Validate(); err != nil {
		return pin, fmt.Errorf("%w; want 64 hex characters (SHA-256 of the certificate) or %s<base64> (SHA-256 of its SubjectPublicKeyInfo)", err, spkiPinPrefix)
	}
	decoded, _ := hex.DecodeString(s) // Validate checked it
	copy(pin.digest[:], decoded)
	return pin, nil
}

// spkiPin returns the "sha256/<base64>" pin of a certificate's key: the
// SHA-256 of its SubjectPublicKeyInfo.
func spkiPin(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return spkiPinPrefix + base64.StdEncoding.EncodeToString(sum[:])
}

// certFingerprint returns the hex pin of a certificate: the SHA-256 of its DER.
func certFingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

// verifyCertPins reports whether any certificate in the chain the server
// presented matches a pin. Pinning an intermediate or root that the server
// sends works the same way as pinning the leaf.
//
// It reads the chain as presented, so it also holds when insecureSkipVerify is
// set and on resumed sessions (the session keeps the original chain).
func verifyCertPins(pins []certPin, serverName string, certs []*x509.Certificate) error {
	if len(pins) == 0 {
		return nil
	}
	if len(certs) == 0 {
		return fmt.Errorf("%w for %s: the server presented no certificate", ErrCertPinMismatch, serverName)
	}
	for _, cert := range certs {
		spki := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
		whole := sha256.Sum256(cert.Raw)
		for _, pin := range pins {
			if pin.spki && pin.digest == spki || !pin.spki && pin.digest == whole {
				return nil
			}
		}
	}
	return fmt.Errorf("%w for %s: none of the %d certificate(s) the server presented matches the %d configured pin(s); the leaf certificate is %s (certificate %s)",
		ErrCertPinMismatch, serverName, len(certs), len(pins), spkiPin(certs[0]), certFingerprint(certs[0]))
}
