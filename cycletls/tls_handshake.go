package cycletls

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	http "github.com/Danny-Dasilva/fhttp"
	utls "github.com/refraction-networking/utls"
)

// This file holds the 3.0.1 TLS handshake options: session resumption
// (sessionResumption), certificate pinning (certPins) and extra root CAs
// (caCerts). dialTLS and its two retry dials in roundtripper.go call into it.

// errCertPinsPlainHTTP is returned when certPins is set and the request (or a
// redirect hop) is plain http. There is no certificate to check there, and
// sending the request anyway would drop the pinning the caller asked for.
var errCertPinsPlainHTTP = errors.New("certPins is set but the URL is http://: there is no TLS certificate to check, so the request was not sent")

// tlsHandshakeKeyPart is the part of the client pool key (generateClientKey)
// that covers the 3.0.1 TLS handshake options. Clients that differ in any of
// them must not share a pooled client or a session cache. It is spliced into
// a format string, so it is a hex digest: caller-supplied pins must not be
// able to put a '%' there.
func tlsHandshakeKeyPart(browser Browser) string {
	h := sha256.New()
	fmt.Fprintf(h, "resume:%t|pins:%d", browser.SessionResumption, len(browser.CertPins))
	for _, pin := range browser.CertPins {
		fmt.Fprintf(h, "|%d:%s", len(pin), pin)
	}
	fmt.Fprintf(h, "|cas:%d", len(browser.CACerts))
	for _, ca := range browser.CACerts {
		fmt.Fprintf(h, "|%d:%s", len(ca), ca)
	}
	return "tls301:" + hex.EncodeToString(h.Sum(nil))
}

// buildRootCAs returns the system roots plus the given PEM certificates. A
// string that holds no certificate, or holds anything that is not a valid
// certificate, is an error.
func buildRootCAs(caCerts []string) (*x509.CertPool, error) {
	if len(caCerts) == 0 {
		return nil, nil
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	for i, pemData := range caCerts {
		if err := appendPEMCerts(pool, []byte(pemData)); err != nil {
			return nil, fmt.Errorf("invalid caCerts[%d]: %w", i, err)
		}
	}
	return pool, nil
}

// appendPEMCerts adds every certificate in pemData to the pool. Unlike
// CertPool.AppendCertsFromPEM it does not skip what it cannot read: text that
// is not PEM, a block that is not a certificate, or a certificate that does
// not parse is an error, as is input with no certificate at all.
func appendPEMCerts(pool *x509.CertPool, pemData []byte) error {
	added := 0
	rest := pemData
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return fmt.Errorf("PEM block %d is %q, want CERTIFICATE", added+1, block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("PEM block %d is not a valid certificate: %w", added+1, err)
		}
		pool.AddCert(cert)
		added++
	}
	if strings.TrimSpace(string(rest)) != "" {
		return errors.New("text that is not PEM follows the certificates")
	}
	if added == 0 {
		return errors.New("no PEM certificate found")
	}
	return nil
}

// initTLSHandshakeOptions copies the 3.0.1 TLS handshake options from the
// Browser and prepares what the dials need. Bad pins or CAs are remembered in
// tlsOptionsErr and fail every request (newRoundTripper has no error return).
func (rt *roundTripper) initTLSHandshakeOptions(browser Browser) {
	rt.SessionResumption = browser.SessionResumption
	rt.CertPins = browser.CertPins
	rt.CACerts = browser.CACerts

	if rt.SessionResumption {
		rt.sessionCache = browser.sessionCache
		if rt.sessionCache == nil {
			// Built without createNewClient (NewTransport and friends): the
			// cache lives as long as this roundTripper.
			rt.sessionCache = newSessionCache()
		}
	}

	pins, err := parseCertPins(rt.CertPins)
	if err != nil {
		rt.tlsOptionsErr = err
		return
	}
	rt.certPins = pins

	pool, err := buildRootCAs(rt.CACerts)
	if err != nil {
		rt.tlsOptionsErr = err
		return
	}
	rt.rootCAs = pool
	if pool != nil {
		// The HTTP/3 dials build their TLS config from rt.TLSConfig, so put
		// the roots there as well unless the caller supplied its own.
		cfg := &utls.Config{}
		if rt.TLSConfig != nil {
			cfg = rt.TLSConfig.Clone()
		}
		if cfg.RootCAs == nil {
			cfg.RootCAs = pool
		}
		rt.TLSConfig = cfg
	}
}

// checkTLSHandshakeOptions runs at the top of RoundTrip, before anything is
// dialed. It fails the request when the options are unusable or when the
// pins could not be enforced on the path the request is about to take.
func (rt *roundTripper) checkTLSHandshakeOptions(req *http.Request) error {
	if rt.tlsOptionsErr != nil {
		return rt.tlsOptionsErr
	}
	if len(rt.certPins) == 0 {
		return nil
	}
	if rt.ForceHTTP3 {
		return errCertPinsHTTP3
	}
	if req != nil && req.URL != nil && strings.EqualFold(req.URL.Scheme, "http") {
		return errCertPinsPlainHTTP
	}
	return nil
}

// handshakeConfig is the utls config for one TLS dial: the first dial and both
// retry dials use it, so SNI, roots and the session cache cannot drift apart.
func (rt *roundTripper) handshakeConfig(serverName string) *utls.Config {
	cfg := &utls.Config{
		ServerName:         serverName,
		OmitEmptyPsk:       true,
		InsecureSkipVerify: rt.InsecureSkipVerify,
		RootCAs:            rt.rootCAs, // nil means the system roots
	}
	if rt.sessionCache != nil {
		cfg.ClientSessionCache = rt.sessionCache
		// A spec with neither a session ticket nor a pre_shared_key extension
		// (a TLS 1.2 JA3 without 35, say) cannot resume. Without this utls
		// panics on such a spec once a session cache is set.
		cfg.PreferSkipResumptionOnNilExtension = true
	}
	return cfg
}

// finishHandshakeSpec applies the options that change the ClientHello after
// the fingerprint has been chosen. It returns the spec to apply with
// utls.HelloCustom, or nil when the built-in helloID should be used as is.
//
//   - A built-in parrot (the Firefox default) carries its own ALPN list. When
//     HTTP/1.1 is forced, or when a pre_shared_key extension has to be added,
//     the parrot is expanded into its spec so that it can be changed.
//   - With sessionResumption on, the spec gets a pre_shared_key extension as
//     its last extension unless it has one. utls leaves it off the wire while
//     there is no session to offer (OmitEmptyPsk), so the first hello to a
//     host is unchanged and only a resumed hello carries extension 41.
func (rt *roundTripper) finishHandshakeSpec(spec *utls.ClientHelloSpec, helloID utls.ClientHelloID) (*utls.ClientHelloSpec, error) {
	if spec == nil {
		if helloID.Client == "" || !(rt.ForceHTTP1 || rt.SessionResumption) {
			return nil, nil
		}
		expanded, err := utls.UTLSIdToSpec(helloID)
		if err != nil {
			return nil, fmt.Errorf("failed to expand ClientHello preset %s: %w", helloID.Str(), err)
		}
		spec = &expanded
		if rt.ForceHTTP1 {
			forceHTTP1ALPN(spec)
		}
	}
	if rt.SessionResumption {
		ensurePreSharedKeyLast(spec)
	}
	return spec, nil
}

// forceHTTP1ALPN makes a spec offer http/1.1 only and drops ALPS, which is
// HTTP/2 only. Same rule as ModernChromeSpec(forceHTTP1=true).
func forceHTTP1ALPN(spec *utls.ClientHelloSpec) {
	filtered := make([]utls.TLSExtension, 0, len(spec.Extensions))
	for _, ext := range spec.Extensions {
		switch ext.(type) {
		case *utls.ALPNExtension:
			filtered = append(filtered, &utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}})
		case *utls.ApplicationSettingsExtension, *utls.ApplicationSettingsExtensionNew:
		default:
			filtered = append(filtered, ext)
		}
	}
	spec.Extensions = filtered
}

// ensurePreSharedKeyLast puts a pre_shared_key extension at the end of the
// spec, where TLS 1.3 requires it. One that is already in the spec is moved
// rather than duplicated.
func ensurePreSharedKeyLast(spec *utls.ClientHelloSpec) {
	var psk utls.TLSExtension
	kept := make([]utls.TLSExtension, 0, len(spec.Extensions)+1)
	for _, ext := range spec.Extensions {
		if _, ok := ext.(utls.PreSharedKeyExtension); ok {
			if psk == nil {
				psk = ext
			}
			continue
		}
		kept = append(kept, ext)
	}
	if psk == nil {
		psk = &utls.UtlsPreSharedKeyExtension{}
	}
	spec.Extensions = append(kept, psk)
}

// verifyHandshake runs the checks that follow a finished handshake. On failure
// it closes the connection, so nothing can be written to a server that did not
// match the pins.
func (rt *roundTripper) verifyHandshake(conn *utls.UConn, serverName string) error {
	if len(rt.certPins) == 0 {
		return nil
	}
	if err := verifyCertPins(rt.certPins, serverName, conn.ConnectionState().PeerCertificates); err != nil {
		_ = conn.Close()
		return err
	}
	return nil
}
