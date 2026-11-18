package cycletls

import (
	"fmt"
	"sort"
	"strings"

	utls "github.com/refraction-networking/utls"
)

// tlsProfiles maps a profile name to the utls ClientHello it stands for. The
// names are the utls ClientHelloID names in lower case with "_" as the only
// separator. The entries without a version follow the utls "Auto" IDs, so they
// move when utls is upgraded; the versioned ones do not.
var tlsProfiles = map[string]utls.ClientHelloID{
	"chrome":        utls.HelloChrome_Auto,
	"chrome_58":     utls.HelloChrome_58,
	"chrome_62":     utls.HelloChrome_62,
	"chrome_70":     utls.HelloChrome_70,
	"chrome_72":     utls.HelloChrome_72,
	"chrome_83":     utls.HelloChrome_83,
	"chrome_87":     utls.HelloChrome_87,
	"chrome_96":     utls.HelloChrome_96,
	"chrome_100":    utls.HelloChrome_100,
	"chrome_102":    utls.HelloChrome_102,
	"chrome_106":    utls.HelloChrome_106_Shuffle,
	"chrome_115_pq": utls.HelloChrome_115_PQ,
	"chrome_120":    utls.HelloChrome_120,
	"chrome_120_pq": utls.HelloChrome_120_PQ,
	"chrome_131":    utls.HelloChrome_131,
	"chrome_133":    utls.HelloChrome_133,

	"firefox":     utls.HelloFirefox_Auto,
	"firefox_55":  utls.HelloFirefox_55,
	"firefox_56":  utls.HelloFirefox_56,
	"firefox_63":  utls.HelloFirefox_63,
	"firefox_65":  utls.HelloFirefox_65,
	"firefox_99":  utls.HelloFirefox_99,
	"firefox_102": utls.HelloFirefox_102,
	"firefox_105": utls.HelloFirefox_105,
	"firefox_120": utls.HelloFirefox_120,

	"safari":      utls.HelloSafari_Auto,
	"safari_16_0": utls.HelloSafari_16_0,

	"ios":      utls.HelloIOS_Auto,
	"ios_11_1": utls.HelloIOS_11_1,
	"ios_12_1": utls.HelloIOS_12_1,
	"ios_13":   utls.HelloIOS_13,
	"ios_14":   utls.HelloIOS_14,

	"edge":     utls.HelloEdge_Auto,
	"edge_85":  utls.HelloEdge_85,
	"edge_106": utls.HelloEdge_106,

	"android_11_okhttp": utls.HelloAndroid_11_OkHttp,

	"360_7_5":  utls.Hello360_7_5,
	"360_11_0": utls.Hello360_11_0,
	"qq_11_1":  utls.HelloQQ_11_1,
}

// TLSProfileNames returns the names the TLSProfile option accepts, sorted.
func TLSProfileNames() []string {
	names := make([]string, 0, len(tlsProfiles))
	for name := range tlsProfiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// normalizeTLSProfileName lower-cases a profile name and turns ".", "-" and
// spaces into "_", so "Safari_16.0" and "chrome-133" are accepted.
func normalizeTLSProfileName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.NewReplacer(".", "_", "-", "_", " ", "_").Replace(name)
}

// TLSProfileToSpec returns the ClientHello spec of a named TLS profile, for
// example "chrome_133", "firefox_120" or "safari_16_0". TLSProfileNames lists
// the names. With forceHTTP1 the ALPN list is cut to http/1.1 and ALPS is
// dropped. The spec is new on every call, so callers may change it.
//
// A profile covers the TLS ClientHello only. HTTP/2 settings, header order and
// the User-Agent still come from the other options.
func TLSProfileToSpec(name string, forceHTTP1 bool) (*utls.ClientHelloSpec, error) {
	id, ok := tlsProfiles[normalizeTLSProfileName(name)]
	if !ok {
		return nil, fmt.Errorf("unknown tlsProfile %q; valid names: %s", name, strings.Join(TLSProfileNames(), ", "))
	}
	spec, err := utls.UTLSIdToSpec(id)
	if err != nil {
		return nil, fmt.Errorf("tlsProfile %q: %w", name, err)
	}
	if forceHTTP1 {
		spec.Extensions = applyForceHTTP1(spec.Extensions)
	}
	return &spec, nil
}

// applyForceHTTP1 cuts the ALPN list to http/1.1 and drops ALPS, which is an
// HTTP/2-only extension. It finds the extensions by type, so it works on any
// extension order.
func applyForceHTTP1(exts []utls.TLSExtension) []utls.TLSExtension {
	filtered := make([]utls.TLSExtension, 0, len(exts))
	for _, ext := range exts {
		switch ext.(type) {
		case *utls.ALPNExtension:
			filtered = append(filtered, &utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}})
		case *utls.ApplicationSettingsExtensionNew, *utls.ApplicationSettingsExtension:
			// ALPS is HTTP/2-only; drop it for HTTP/1.1
		default:
			filtered = append(filtered, ext)
		}
	}
	return filtered
}

// shuffleSpecExtensions puts the extensions of spec in a random order, the way
// Chrome has done on every connection since version 106. GREASE, padding and
// pre_shared_key keep their positions.
func shuffleSpecExtensions(spec *utls.ClientHelloSpec) {
	if spec == nil {
		return
	}
	spec.Extensions = utls.ShuffleChromeTLSExtensions(spec.Extensions)
}

// optInSpec returns the ClientHello spec chosen by the 3.0.1 options, or nil
// when neither is set and the older options (quicFingerprint, ja3, ja4r, the
// User-Agent default) decide. ClientHello wins over TLSProfile, and both win
// over the older options.
//
// ShuffleExtensions applies to a profile. It does not apply to an imported
// ClientHello, where the captured order is the fingerprint.
func (rt *roundTripper) optInSpec() (*utls.ClientHelloSpec, error) {
	if rt.ClientHello != "" {
		return ClientHelloToSpec(rt.ClientHello, rt.ForceHTTP1)
	}
	if rt.TLSProfile != "" {
		spec, err := TLSProfileToSpec(rt.TLSProfile, rt.ForceHTTP1)
		if err != nil {
			return nil, err
		}
		if rt.ShuffleExtensions {
			shuffleSpecExtensions(spec)
		}
		return spec, nil
	}
	return nil, nil
}
