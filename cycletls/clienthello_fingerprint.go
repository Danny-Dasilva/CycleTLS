package cycletls

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// TLS constants used when reading a ClientHello.
const (
	tlsRecordTypeHandshake   = 0x16
	tlsHandshakeClientHello  = 0x01
	tlsRecordHeaderLen       = 5
	tlsHandshakeHeaderLen    = 4
	extensionIDServerName    = 0x0000
	extensionIDGroups        = 0x000a
	extensionIDPointFormats  = 0x000b
	extensionIDSigAlgs       = 0x000d
	extensionIDALPN          = 0x0010
	extensionIDSupportedVers = 0x002b
)

// ClientHelloInfo holds the fields of a TLS ClientHello that JA3 and JA4 are
// computed from. Lists are in wire order and still contain GREASE values; the
// JA3 and JA4 methods remove them.
type ClientHelloInfo struct {
	// Version is the legacy_version field of the ClientHello (0x0303 for
	// TLS 1.2 and TLS 1.3 hellos).
	Version uint16
	// CipherSuites is the cipher suite list.
	CipherSuites []uint16
	// Extensions is the list of extension IDs.
	Extensions []uint16
	// ExtensionData holds the body of each extension, in the same order as
	// Extensions.
	ExtensionData [][]byte
	// SupportedGroups is the body of supported_groups (extension 10).
	SupportedGroups []uint16
	// PointFormats is the body of ec_point_formats (extension 11).
	PointFormats []uint8
	// SignatureAlgorithms is the body of signature_algorithms (extension 13).
	SignatureAlgorithms []uint16
	// SupportedVersions is the body of supported_versions (extension 43).
	SupportedVersions []uint16
	// ALPN is the protocol list of extension 16.
	ALPN []string
	// HasSNI reports whether the server_name extension is present.
	HasSNI bool
	// ServerName is the host name in the server_name extension, if any.
	ServerName string
}

// clientHelloMessage returns the ClientHello handshake message (type byte,
// 3-byte length, body) found in raw. raw is either that message itself or one
// or more TLS records that carry it.
func clientHelloMessage(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("client hello: empty input")
	}
	msg := raw
	if raw[0] == tlsRecordTypeHandshake {
		// Join the handshake payload of every record. A ClientHello larger
		// than one record is split across several.
		msg = nil
		rest := raw
		for len(rest) > 0 {
			if len(rest) < tlsRecordHeaderLen {
				return nil, errors.New("client hello: truncated TLS record header")
			}
			if rest[0] != tlsRecordTypeHandshake {
				break
			}
			n := int(rest[3])<<8 | int(rest[4])
			if len(rest) < tlsRecordHeaderLen+n {
				return nil, errors.New("client hello: truncated TLS record")
			}
			msg = append(msg, rest[tlsRecordHeaderLen:tlsRecordHeaderLen+n]...)
			rest = rest[tlsRecordHeaderLen+n:]
			if len(msg) >= tlsHandshakeHeaderLen {
				want := int(msg[1])<<16 | int(msg[2])<<8 | int(msg[3])
				if len(msg) >= tlsHandshakeHeaderLen+want {
					break
				}
			}
		}
	}
	if len(msg) < tlsHandshakeHeaderLen {
		return nil, errors.New("client hello: truncated handshake header")
	}
	if msg[0] != tlsHandshakeClientHello {
		return nil, fmt.Errorf("client hello: handshake message type %d is not a ClientHello", msg[0])
	}
	n := int(msg[1])<<16 | int(msg[2])<<8 | int(msg[3])
	if len(msg) < tlsHandshakeHeaderLen+n {
		return nil, errors.New("client hello: truncated handshake message")
	}
	return msg[:tlsHandshakeHeaderLen+n], nil
}

// byteReader reads big-endian fields from a ClientHello and remembers whether
// any read ran past the end.
type byteReader struct {
	b   []byte
	bad bool
}

func (r *byteReader) take(n int) []byte {
	if r.bad || n < 0 || len(r.b) < n {
		r.bad = true
		return nil
	}
	out := r.b[:n]
	r.b = r.b[n:]
	return out
}

func (r *byteReader) u8() int {
	b := r.take(1)
	if b == nil {
		return 0
	}
	return int(b[0])
}

func (r *byteReader) u16() int {
	b := r.take(2)
	if b == nil {
		return 0
	}
	return int(b[0])<<8 | int(b[1])
}

func (r *byteReader) vec8() []byte  { return r.take(r.u8()) }
func (r *byteReader) vec16() []byte { return r.take(r.u16()) }

func u16List(b []byte) []uint16 {
	out := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		out = append(out, uint16(b[i])<<8|uint16(b[i+1]))
	}
	return out
}

// ParseClientHello reads a TLS ClientHello. raw is either the TLS record(s)
// as captured on the wire (first byte 0x16) or the bare handshake message
// (first byte 0x01), which is what utls exposes as HandshakeState.Hello.Raw.
func ParseClientHello(raw []byte) (*ClientHelloInfo, error) {
	msg, err := clientHelloMessage(raw)
	if err != nil {
		return nil, err
	}
	r := &byteReader{b: msg[tlsHandshakeHeaderLen:]}
	info := &ClientHelloInfo{}
	info.Version = uint16(r.u16())
	r.take(32) // random
	r.vec8()   // session id
	info.CipherSuites = u16List(r.vec16())
	r.vec8() // compression methods
	if r.bad {
		return nil, errors.New("client hello: truncated before extensions")
	}
	if len(r.b) == 0 {
		return info, nil // extensions are optional
	}
	exts := &byteReader{b: r.vec16()}
	if r.bad {
		return nil, errors.New("client hello: truncated extension block")
	}
	for len(exts.b) > 0 {
		id := uint16(exts.u16())
		body := exts.vec16()
		if exts.bad {
			return nil, errors.New("client hello: truncated extension")
		}
		info.Extensions = append(info.Extensions, id)
		info.ExtensionData = append(info.ExtensionData, body)
		br := &byteReader{b: body}
		switch id {
		case extensionIDServerName:
			info.HasSNI = true
			list := &byteReader{b: br.vec16()}
			for len(list.b) > 0 && !list.bad {
				nameType := list.u8()
				name := list.vec16()
				if nameType == 0 && info.ServerName == "" {
					info.ServerName = string(name)
				}
			}
		case extensionIDGroups:
			info.SupportedGroups = u16List(br.vec16())
		case extensionIDPointFormats:
			info.PointFormats = append([]uint8(nil), br.vec8()...)
		case extensionIDSigAlgs:
			info.SignatureAlgorithms = u16List(br.vec16())
		case extensionIDSupportedVers:
			info.SupportedVersions = u16List(br.vec8())
		case extensionIDALPN:
			list := &byteReader{b: br.vec16()}
			for len(list.b) > 0 && !list.bad {
				info.ALPN = append(info.ALPN, string(list.vec8()))
			}
		}
	}
	return info, nil
}

// ExtensionBody returns the body of the first extension with the given ID and
// whether the extension is present.
func (c *ClientHelloInfo) ExtensionBody(id uint16) ([]byte, bool) {
	for i, e := range c.Extensions {
		if e == id && i < len(c.ExtensionData) {
			return c.ExtensionData[i], true
		}
	}
	for _, e := range c.Extensions {
		if e == id {
			return nil, true
		}
	}
	return nil, false
}

func withoutGREASE(in []uint16) []uint16 {
	out := make([]uint16, 0, len(in))
	for _, v := range in {
		if !IsGREASEValue(v) {
			out = append(out, v)
		}
	}
	return out
}

func joinDecimal(in []uint16) string {
	parts := make([]string, len(in))
	for i, v := range in {
		parts[i] = strconv.Itoa(int(v))
	}
	return strings.Join(parts, "-")
}

func joinHex(in []uint16) string {
	parts := make([]string, len(in))
	for i, v := range in {
		parts[i] = fmt.Sprintf("%04x", v)
	}
	return strings.Join(parts, ",")
}

// JA3 returns the JA3 string of the hello:
// version,ciphers,extensions,groups,point formats, each list joined with "-"
// and GREASE values removed.
func (c *ClientHelloInfo) JA3() string {
	formats := make([]string, len(c.PointFormats))
	for i, f := range c.PointFormats {
		formats[i] = strconv.Itoa(int(f))
	}
	return strings.Join([]string{
		strconv.Itoa(int(c.Version)),
		joinDecimal(withoutGREASE(c.CipherSuites)),
		joinDecimal(withoutGREASE(c.Extensions)),
		joinDecimal(withoutGREASE(c.SupportedGroups)),
		strings.Join(formats, "-"),
	}, ",")
}

// JA3Hash returns the MD5 of the JA3 string in hex, which is the value usually
// called "the JA3".
func (c *ClientHelloInfo) JA3Hash() string {
	sum := md5.Sum([]byte(c.JA3()))
	return hex.EncodeToString(sum[:])
}

// ja4Version maps a TLS version to the two characters JA4 uses.
func ja4Version(v uint16) string {
	switch v {
	case 0x0304:
		return "13"
	case 0x0303:
		return "12"
	case 0x0302:
		return "11"
	case 0x0301:
		return "10"
	case 0x0300:
		return "s3"
	case 0x0002:
		return "s2"
	default:
		return "00"
	}
}

func isASCIIAlnum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// ja4ALPN returns the two ALPN characters of JA4: the first and last character
// of the first ALPN value, "00" when there is none, and the first and last hex
// digit of the value when either end is not alphanumeric.
func ja4ALPN(alpn []string) string {
	if len(alpn) == 0 || alpn[0] == "" {
		return "00"
	}
	v := alpn[0]
	first, last := v[0], v[len(v)-1]
	if isASCIIAlnum(first) && isASCIIAlnum(last) {
		return string([]byte{first, last})
	}
	h := hex.EncodeToString([]byte(v))
	return string([]byte{h[0], h[len(h)-1]})
}

func ja4Count(n int) string {
	if n > 99 {
		n = 99
	}
	return fmt.Sprintf("%02d", n)
}

func sortedCopy(in []uint16) []uint16 {
	out := append([]uint16(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func ja4Hash(s string) string {
	if s == "" {
		return "000000000000"
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:12]
}

// ja4Parts returns the three sections of JA4 before hashing: the prefix
// (JA4_a), the sorted cipher list, and the sorted extension list followed by
// "_" and the signature algorithms in wire order.
func (c *ClientHelloInfo) ja4Parts() (a, ciphers, exts string) {
	version := c.Version
	if sv := withoutGREASE(c.SupportedVersions); len(sv) > 0 {
		version = 0
		for _, v := range sv {
			if v > version {
				version = v
			}
		}
	}
	sni := "i"
	if c.HasSNI {
		sni = "d"
	}
	cs := withoutGREASE(c.CipherSuites)
	es := withoutGREASE(c.Extensions)
	a = "t" + ja4Version(version) + sni + ja4Count(len(cs)) + ja4Count(len(es)) + ja4ALPN(c.ALPN)

	ciphers = joinHex(sortedCopy(cs))

	// SNI and ALPN are counted in the prefix but left out of the hashed list.
	hashed := make([]uint16, 0, len(es))
	for _, e := range es {
		if e != extensionIDServerName && e != extensionIDALPN {
			hashed = append(hashed, e)
		}
	}
	exts = joinHex(sortedCopy(hashed))
	if sig := withoutGREASE(c.SignatureAlgorithms); len(sig) > 0 {
		exts += "_" + joinHex(sig)
	}
	return a, ciphers, exts
}

// JA4 returns the JA4 TLS client fingerprint as FoxIO defines it, for example
// "t13d1516h2_8daaf6152771_e5627efa2ab1". The protocol character is always "t"
// (TLS over TCP).
func (c *ClientHelloInfo) JA4() string {
	a, ciphers, exts := c.ja4Parts()
	return a + "_" + ja4Hash(ciphers) + "_" + ja4Hash(exts)
}

// JA4R returns the raw form of JA4 (JA4_r): the same sorted lists as JA4,
// written out instead of hashed. JA4RStringToSpec accepts this format.
func (c *ClientHelloInfo) JA4R() string {
	a, ciphers, exts := c.ja4Parts()
	return a + "_" + ciphers + "_" + exts
}

// JA3FromClientHello returns the JA3 string and its MD5 hash for a ClientHello.
// See ParseClientHello for the accepted input.
func JA3FromClientHello(raw []byte) (ja3 string, hash string, err error) {
	info, err := ParseClientHello(raw)
	if err != nil {
		return "", "", err
	}
	return info.JA3(), info.JA3Hash(), nil
}

// JA4FromClientHello returns the FoxIO JA4 fingerprint of a ClientHello.
// See ParseClientHello for the accepted input.
func JA4FromClientHello(raw []byte) (string, error) {
	info, err := ParseClientHello(raw)
	if err != nil {
		return "", err
	}
	return info.JA4(), nil
}

// JA4RFromClientHello returns the raw JA4 (JA4_r) of a ClientHello.
// See ParseClientHello for the accepted input.
func JA4RFromClientHello(raw []byte) (string, error) {
	info, err := ParseClientHello(raw)
	if err != nil {
		return "", err
	}
	return info.JA4R(), nil
}
