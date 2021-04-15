package cycletls

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"crypto/sha256"
	"golang.org/x/net/http2"
	"golang.org/x/net/proxy"
	"strconv"

	utls "github.com/refraction-networking/utls"
)

var errProtocolNegotiated = errors.New("protocol negotiated")
type ErrExtensionNotExist string
func (e ErrExtensionNotExist) Error() string {
	return fmt.Sprintf("Extension does not exist: %s\n", e)
}

type roundTripper struct {
	sync.Mutex
	// fix typing
	JA3 			  string
	UserAgent 		  string

	cachedConnections map[string]net.Conn
	cachedTransports  map[string]http.RoundTripper

	dialer proxy.ContextDialer
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Fix there may be a better place to put this header
	
	req.Header.Set("User-Agent", rt.UserAgent)
	addr := rt.getDialTLSAddr(req)
	if _, ok := rt.cachedTransports[addr]; !ok {
		if err := rt.getTransport(req, addr); err != nil {
			return nil, err
		}
	}
	return rt.cachedTransports[addr].RoundTrip(req)
}

func (rt *roundTripper) getTransport(req *http.Request, addr string) error {
	switch strings.ToLower(req.URL.Scheme) {
	case "http":
		rt.cachedTransports[addr] = &http.Transport{DialContext: rt.dialer.DialContext, DisableKeepAlives: true}
		return nil
	case "https":
	default:
		return fmt.Errorf("invalid URL scheme: [%v]", req.URL.Scheme)
	}

	_, err := rt.dialTLS(context.Background(), "tcp", addr)
	switch err {
	case errProtocolNegotiated:
	case nil:
		// Should never happen.
		panic("dialTLS returned no error when determining cachedTransports")
	default:
		return err
	}

	return nil
}

func (rt *roundTripper) dialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	rt.Lock()
	defer rt.Unlock()

	// If we have the connection from when we determined the HTTPS
	// cachedTransports to use, return that.
	if conn := rt.cachedConnections[addr]; conn != nil {
		delete(rt.cachedConnections, addr)
		return conn, nil
	}

	rawConn, err := rt.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	var host string
	if host, _, err = net.SplitHostPort(addr); err != nil {
		host = addr
	}

	// conn := utls.UClient(rawConn, &utls.Config{ServerName: host}, rt.clientHelloId)
	// if err = conn.Handshake(); err != nil {
	// 	_ = conn.Close()
	// 	return nil, err
	// }

	//////////////////test code inserted

	spec, err := stringToSpec(rt.JA3)
	if err != nil {
		return nil, err
	}

	conn := utls.UClient(rawConn, &utls.Config{ServerName: host, 
		// MinVersion:         tls.VersionTLS10,
		// MaxVersion:         tls.VersionTLS12, // Default is TLS13
		}, utls.HelloCustom)
	if err := conn.ApplyPreset(spec); err != nil {
		return nil, err
	}

	if err = conn.Handshake(); err != nil {
		_ = conn.Close()
		return nil, err
	}


	//////////
	if rt.cachedTransports[addr] != nil {
		return conn, nil
	}

	// No http.Transport constructed yet, create one based on the results
	// of ALPN.
	switch conn.ConnectionState().NegotiatedProtocol {
	case http2.NextProtoTLS:
		// The remote peer is speaking HTTP 2 + TLS.
		rt.cachedTransports[addr] = &http2.Transport{DialTLS: rt.dialTLSHTTP2}
	default:
		// Assume the remote peer is speaking HTTP 1.x + TLS.
		rt.cachedTransports[addr] = &http.Transport{DialTLSContext: rt.dialTLS}
		
	}

	// Stash the connection just established for use servicing the
	// actual request (should be near-immediate).
	rt.cachedConnections[addr] = conn

	return nil, errProtocolNegotiated
}

func (rt *roundTripper) dialTLSHTTP2(network, addr string, _ *tls.Config) (net.Conn, error) {
	return rt.dialTLS(context.Background(), network, addr)
}

func (rt *roundTripper) getDialTLSAddr(req *http.Request) string {
	host, port, err := net.SplitHostPort(req.URL.Host)
	if err == nil {
		return net.JoinHostPort(host, port)
	}
	return net.JoinHostPort(req.URL.Host, "443") // we can assume port is 443 at this point
}

func newRoundTripper(browser Browser, dialer ...proxy.ContextDialer) http.RoundTripper {
	if len(dialer) > 0 {
	
		return &roundTripper{
			dialer: dialer[0],

			JA3: browser.JA3,
			UserAgent: browser.UserAgent,

			cachedTransports:  make(map[string]http.RoundTripper),
			cachedConnections: make(map[string]net.Conn),
		}
	} else {

		return &roundTripper{
			dialer: proxy.Direct,

			JA3: browser.JA3,
			UserAgent: browser.UserAgent,

			cachedTransports:  make(map[string]http.RoundTripper),
			cachedConnections: make(map[string]net.Conn),
		}
	}
}
///////////////////////// test code
// stringToSpec creates a ClientHelloSpec based on a JA3 string
func stringToSpec(ja3 string) (*utls.ClientHelloSpec, error) {
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
	for _, c := range curves {
		cid, err := strconv.ParseUint(c, 10, 16)
		if err != nil {
			return nil, err
		}
		targetCurves = append(targetCurves, utls.CurveID(cid))
	}
	extMap["10"] = &utls.SupportedCurvesExtension{targetCurves}

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

	// build extenions list
	var exts []utls.TLSExtension
	for _, e := range extensions {
		te, ok := extMap[e]
		if !ok {
			return nil, ErrExtensionNotExist(e)
		}
		exts = append(exts, te)
	}
	// build SSLVersion
	vid64, err := strconv.ParseUint(version, 10, 16)
	if err != nil {
		return nil, err
	}
	vid := uint16(vid64)

	// build CipherSuites
	var suites []uint16
	for _, c := range ciphers {
		cid, err := strconv.ParseUint(c, 10, 16)
		if err != nil {
			return nil, err
		}
		suites = append(suites, uint16(cid))
	}

	return &utls.ClientHelloSpec{
		TLSVersMin:         vid,
		TLSVersMax:         vid,
		CipherSuites:       suites,
		CompressionMethods: []byte{0},
		Extensions:         exts,
		GetSessionID:       sha256.Sum256,
	}, nil
}



var extMap = map[string]utls.TLSExtension{
	"0": &utls.SNIExtension{},
	"5": &utls.StatusRequestExtension{},
	// These are applied later
	// "10": &tls.SupportedCurvesExtension{...}
	// "11": &tls.SupportedPointsExtension{...}
	"13": &utls.SignatureAlgorithmsExtension{
		SupportedSignatureAlgorithms: []utls.SignatureScheme{
			utls.ECDSAWithP256AndSHA256,
			utls.PSSWithSHA256,
			utls.PKCS1WithSHA256,
			utls.ECDSAWithP384AndSHA384,
			utls.PSSWithSHA384,
			utls.PKCS1WithSHA384,
			utls.PSSWithSHA512,
			utls.PKCS1WithSHA512,
			utls.PKCS1WithSHA1,
		},
	},
	"16": &utls.ALPNExtension{
		AlpnProtocols: []string{"h2", "http/1.1"},
	},
	"18": &utls.SCTExtension{},
	"21": &utls.UtlsPaddingExtension{GetPaddingLen: utls.BoringPaddingStyle},
	"23": &utls.UtlsExtendedMasterSecretExtension{},
	"27": &utls.FakeCertCompressionAlgsExtension{},
	"28": &utls.FakeRecordSizeLimitExtension{},
	"35": &utls.SessionTicketExtension{},
	"43": &utls.SupportedVersionsExtension{[]uint16{
		utls.GREASE_PLACEHOLDER,
		utls.VersionTLS13,
		utls.VersionTLS12,
		utls.VersionTLS11,
		utls.VersionTLS10}},
	"44": &utls.CookieExtension{},
	"45": &utls.PSKKeyExchangeModesExtension{[]uint8{
		utls.PskModeDHE,
	}},
	"51":    &utls.KeyShareExtension{[]utls.KeyShare{}},
	"13172": &utls.NPNExtension{},
	"65281": &utls.RenegotiationInfoExtension{
		Renegotiation: utls.RenegotiateOnceAsClient,
	},
}
