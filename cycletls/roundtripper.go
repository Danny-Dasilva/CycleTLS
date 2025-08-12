package cycletls

import (
	"context"
	"errors"
	"fmt"
	http "github.com/Danny-Dasilva/fhttp"
	http2 "github.com/Danny-Dasilva/fhttp/http2"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/proxy"
	"net"
	"strings"
	"sync"
)

var errProtocolNegotiated = errors.New("protocol negotiated")

type roundTripper struct {
	sync.Mutex

	// TLS fingerprinting options
	JA3              string
	JA4r             string // JA4 raw format with explicit cipher/extension values
	HTTP2Fingerprint string
	QUICFingerprint  string
	DisableGrease    bool

	// Browser identification
	UserAgent   string
	HeaderOrder []string

	// Connection options
	TLSConfig          *utls.Config
	InsecureSkipVerify bool
	Cookies            []Cookie
	ForceHTTP1         bool
	ForceHTTP3         bool

	// Caching
	cachedConnections map[string]net.Conn
	cachedTransports  map[string]http.RoundTripper

	dialer proxy.ContextDialer
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Apply cookies to the request
	for _, properties := range rt.Cookies {
		cookie := &http.Cookie{
			Name:       properties.Name,
			Value:      properties.Value,
			Path:       properties.Path,
			Domain:     properties.Domain,
			Expires:    properties.JSONExpires.Time,
			RawExpires: properties.RawExpires,
			MaxAge:     properties.MaxAge,
			HttpOnly:   properties.HTTPOnly,
			Secure:     properties.Secure,
			Raw:        properties.Raw,
			Unparsed:   properties.Unparsed,
		}
		req.AddCookie(cookie)
	}

	// Apply user agent
	req.Header.Set("User-Agent", rt.UserAgent)

	// Apply header order if specified (for regular headers, not pseudo-headers)
	if len(rt.HeaderOrder) > 0 {
		req.Header = ConvertHttpHeader(MarshalHeader(req.Header, rt.HeaderOrder))

		// Note: rt.HeaderOrder contains regular headers like "cache-control", "accept", etc.
		// Do NOT overwrite http.PHeaderOrderKey which contains pseudo-headers like ":method", ":path"
		// The pseudo-header order is already set correctly in index.go based on UserAgent parsing
	}

	// Get address for dialing
	addr := rt.getDialTLSAddr(req)

	// Check if we need HTTP/3
	if rt.ForceHTTP3 {
		// Use HTTP/3 transport with UQuic if QUIC fingerprint is provided
		tlsConfig := ConvertUtlsConfig(rt.TLSConfig)

		if rt.QUICFingerprint != "" {
			// Create UQuic spec from fingerprint
			quicSpec, err := CreateUQuicSpecFromFingerprint(rt.QUICFingerprint)
			if err != nil {
				// Fallback to standard HTTP/3 transport if UQuic spec creation fails
				transport := NewHTTP3Transport(tlsConfig)
				return transport.RoundTrip(req)
			}

			// Use UQuic transport
			transport := NewHTTP3TransportWithUQuic(tlsConfig, quicSpec)
			return transport.RoundTrip(req)
		} else {
			// Use standard HTTP/3 transport
			transport := NewHTTP3Transport(tlsConfig)
			return transport.RoundTrip(req)
		}
	}

	// Use cached transport if available, otherwise create a new one
	if _, ok := rt.cachedTransports[addr]; !ok {
		if err := rt.getTransport(req, addr); err != nil {
			return nil, err
		}
	}

	// Perform the request
	return rt.cachedTransports[addr].RoundTrip(req)
}

func (rt *roundTripper) getTransport(req *http.Request, addr string) error {
	switch strings.ToLower(req.URL.Scheme) {
	case "http":
		// Allow connection reuse by removing DisableKeepAlives
		rt.cachedTransports[addr] = &http.Transport{
			DialContext: rt.dialer.DialContext,
		}
		return nil
	case "https":
	default:
		return fmt.Errorf("invalid URL scheme: [%v]", req.URL.Scheme)
	}

	// Establish TLS connection
	_, err := rt.dialTLS(req.Context(), "tcp", addr)
	switch err {
	case errProtocolNegotiated:
		// Expected behavior - transport has been cached
	case nil:
		// Should never happen
		panic("dialTLS returned no error when determining cached transports")
	default:
		return err
	}

	return nil
}

func (rt *roundTripper) dialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	rt.Lock()
	defer rt.Unlock()

	// Return cached connection if available
	if conn := rt.cachedConnections[addr]; conn != nil {
		return conn, nil
	}

	// Establish raw connection
	rawConn, err := rt.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	// Extract host from address
	var host string
	if host, _, err = net.SplitHostPort(addr); err != nil {
		host = addr
	}

	var spec *utls.ClientHelloSpec

	// Determine which fingerprint to use
	if rt.QUICFingerprint != "" {
		// Use QUIC fingerprint
		spec, err = QUICStringToSpec(rt.QUICFingerprint, rt.UserAgent, rt.ForceHTTP1)
		if err != nil {
			return nil, err
		}
	} else if rt.JA3 != "" {
		// Use JA3 fingerprint
		spec, err = StringToSpec(rt.JA3, rt.UserAgent, rt.ForceHTTP1)
		if err != nil {
			return nil, err
		}
	} else if rt.JA4r != "" {
		// Use JA4r (raw) fingerprint
		spec, err = JA4RStringToSpec(rt.JA4r, rt.UserAgent, rt.ForceHTTP1, rt.DisableGrease)
		if err != nil {
			return nil, err
		}
	} else {
		// Default to Chrome fingerprint
		spec, err = StringToSpec(DefaultChrome_JA3, rt.UserAgent, rt.ForceHTTP1)
		if err != nil {
			return nil, err
		}
	}

	// Create TLS client
	conn := utls.UClient(rawConn, &utls.Config{
		ServerName:         host,
		OmitEmptyPsk:       true,
		InsecureSkipVerify: rt.InsecureSkipVerify,
	}, utls.HelloCustom)

	// Apply TLS fingerprint
	if err := conn.ApplyPreset(spec); err != nil {
		return nil, err
	}

	// Perform TLS handshake
	if err = conn.Handshake(); err != nil {
		_ = conn.Close()

		if err.Error() == "tls: CurvePreferences includes unsupported curve" {
			return nil, fmt.Errorf("conn.Handshake() error for TLS 1.3 (please retry request): %+v", err)
		}
		return nil, fmt.Errorf("uTlsConn.Handshake() error: %+v", err)
	}

	// If transport already exists, return connection
	if rt.cachedTransports[addr] != nil {
		return conn, nil
	}

	// Create appropriate transport based on negotiated protocol
	switch conn.ConnectionState().NegotiatedProtocol {
	case http2.NextProtoTLS:
		// HTTP/2 transport
		parsedUserAgent := parseUserAgent(rt.UserAgent)

		// Use HTTP/2 fingerprint if specified
		var http2Transport http2.Transport
		if rt.HTTP2Fingerprint != "" {
			// Parse and apply HTTP/2 fingerprint
			h2Fingerprint, err := NewHTTP2Fingerprint(rt.HTTP2Fingerprint)
			if err != nil {
				return nil, fmt.Errorf("failed to parse HTTP/2 fingerprint: %v", err)
			}

			http2Transport = http2.Transport{
				DialTLS:     rt.dialTLSHTTP2,
				PushHandler: &http2.DefaultPushHandler{},
				Navigator:   parsedUserAgent.UserAgent,
			}

			// Apply HTTP/2 fingerprint settings
			h2Fingerprint.Apply(&http2Transport)
		} else {
			http2Transport = http2.Transport{
				DialTLS:     rt.dialTLSHTTP2,
				PushHandler: &http2.DefaultPushHandler{},
				Navigator:   parsedUserAgent.UserAgent,
			}
		}

		rt.cachedTransports[addr] = &http2Transport
	default:
		// HTTP/1.x transport - configure to avoid idle channel errors
		rt.cachedTransports[addr] = &http.Transport{
			DialTLSContext:    rt.dialTLS,
			DisableKeepAlives: true, // Disable keep-alives to prevent idle channel errors
		}
	}

	// Cache the connection for future use
	rt.cachedConnections[addr] = conn

	return nil, errProtocolNegotiated
}

func (rt *roundTripper) dialTLSHTTP2(network, addr string, _ *utls.Config) (net.Conn, error) {
	return rt.dialTLS(context.Background(), network, addr)
}

func (rt *roundTripper) getDialTLSAddr(req *http.Request) string {
	host, port, err := net.SplitHostPort(req.URL.Host)
	if err == nil {
		return net.JoinHostPort(host, port)
	}
	return net.JoinHostPort(req.URL.Host, "443") // Default HTTPS port
}

// CloseIdleConnections closes connections that have been idle for too long
// If selectedAddr is provided, only close connections not matching this address
func (rt *roundTripper) CloseIdleConnections(selectedAddr ...string) {
	rt.Lock()
	defer rt.Unlock()

	// If we have a specific address to keep, only close other connections
	if len(selectedAddr) > 0 && selectedAddr[0] != "" {
		addr := selectedAddr[0]
		// Keep the connection for the provided address, close others
		for connAddr, conn := range rt.cachedConnections {
			if connAddr != addr {
				_ = conn.Close()
				delete(rt.cachedConnections, connAddr)
			}
		}
	} else {
		// No address specified, close all connections (original behavior)
		for addr, conn := range rt.cachedConnections {
			_ = conn.Close()
			delete(rt.cachedConnections, addr)
		}
	}
}

func newRoundTripper(browser Browser, dialer ...proxy.ContextDialer) http.RoundTripper {
	var contextDialer proxy.ContextDialer
	if len(dialer) > 0 {
		contextDialer = dialer[0]
	} else {
		contextDialer = proxy.Direct
	}

	return &roundTripper{
		dialer:             contextDialer,
		JA3:                browser.JA3,
		JA4r:               browser.JA4r,
		HTTP2Fingerprint:   browser.HTTP2Fingerprint,
		QUICFingerprint:    browser.QUICFingerprint,
		DisableGrease:      browser.DisableGrease,
		UserAgent:          browser.UserAgent,
		HeaderOrder:        browser.HeaderOrder,
		TLSConfig:          browser.TLSConfig,
		Cookies:            browser.Cookies,
		cachedTransports:   make(map[string]http.RoundTripper),
		cachedConnections:  make(map[string]net.Conn),
		InsecureSkipVerify: browser.InsecureSkipVerify,
		ForceHTTP1:         browser.ForceHTTP1,
		ForceHTTP3:         browser.ForceHTTP3,
	}
}

// Default JA3 fingerprint for Chrome
const DefaultChrome_JA3 = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0"
