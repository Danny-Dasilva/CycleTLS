package cycletls

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	utls "github.com/refraction-networking/utls"
)

// decodeClientHelloHex decodes the hex form of a captured ClientHello. It
// accepts upper or lower case, an optional "0x" prefix, and white space or
// ":" between bytes, which is how packet tools print them.
func decodeClientHelloHex(s string) ([]byte, error) {
	s = strings.NewReplacer(" ", "", "\n", "", "\r", "", "\t", "", ":", "").Replace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if s == "" {
		return nil, errors.New("clientHello: empty value")
	}
	raw, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("clientHello: not valid hex: %w", err)
	}
	return raw, nil
}

// ClientHelloToSpec turns a captured ClientHello into a spec that sends the
// same hello: same cipher suites, extensions, extension order and extension
// bodies. clientHelloHex is the hex of the TLS record as captured on the wire
// (it starts with 16 03 01), or of the bare handshake message (it starts with
// 01). A hello split across several records is joined first.
//
// Three things are not copied from the capture:
//   - the server name: the SNI extension stays, and utls fills in the host of
//     each connection;
//   - key shares and the client random: utls makes new ones per connection;
//   - pre_shared_key: a captured ticket and binder belong to someone else's
//     session, so the extension is dropped.
//
// Extensions utls has no type for are replayed byte for byte. With forceHTTP1
// the ALPN list is cut to http/1.1 and ALPS is dropped.
func ClientHelloToSpec(clientHelloHex string, forceHTTP1 bool) (*utls.ClientHelloSpec, error) {
	raw, err := decodeClientHelloHex(clientHelloHex)
	if err != nil {
		return nil, err
	}
	msg, err := clientHelloMessage(raw)
	if err != nil {
		return nil, fmt.Errorf("clientHello: %w", err)
	}

	// utls wants one TLS record. Rebuild it around the handshake message so a
	// bare message and a hello split across records both work. utls does not
	// read the record length, which cannot hold more than 65535 anyway.
	n := len(msg)
	if n > 0xffff {
		n = 0xffff
	}
	record := append([]byte{tlsRecordTypeHandshake, 0x03, 0x01, byte(n >> 8), byte(n)}, msg...)

	spec, err := (&utls.Fingerprinter{AllowBluntMimicry: true}).FingerprintClientHello(record)
	if err != nil {
		return nil, fmt.Errorf("clientHello: %w", err)
	}

	exts := make([]utls.TLSExtension, 0, len(spec.Extensions))
	for _, ext := range spec.Extensions {
		switch ext.(type) {
		case utls.PreSharedKeyExtension:
			continue
		case *utls.SNIExtension:
			exts = append(exts, &utls.SNIExtension{})
		default:
			exts = append(exts, ext)
		}
	}
	if forceHTTP1 {
		exts = applyForceHTTP1(exts)
	}
	spec.Extensions = exts
	return spec, nil
}
