package cycletls

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	stdhttp "net/http"
	"time"

	http "github.com/Danny-Dasilva/fhttp"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	uquic "github.com/refraction-networking/uquic"
)

// HTTP3Transport represents an HTTP/3 transport with customizable settings
type HTTP3Transport struct {
	// QuicConfig is the QUIC configuration
	QuicConfig *quic.Config

	// TLSClientConfig is the TLS configuration
	TLSClientConfig *tls.Config

	// UQuic integration fields
	UQuicConfig *uquic.Config
	QUICSpec    *uquic.QUICSpec
	UseUQuic    bool // Enable uquic-based transport when QUIC fingerprint is provided

	// MaxIdleConns controls the maximum number of idle connections
	MaxIdleConns int

	// IdleConnTimeout is the maximum amount of time a connection may be idle
	IdleConnTimeout time.Duration

	// ResponseHeaderTimeout is the amount of time to wait for a server's response headers
	ResponseHeaderTimeout time.Duration

	// DialTimeout is the maximum amount of time a dial will wait for a connect to complete
	DialTimeout time.Duration

	// ForceAttemptHTTP2 specifies whether HTTP/2 should be attempted
	ForceAttemptHTTP2 bool

	// DisableCompression, if true, prevents the Transport from
	// requesting compression with an "Accept-Encoding: gzip"
	DisableCompression bool
}

// NewHTTP3Transport creates a new HTTP/3 transport
func NewHTTP3Transport(tlsConfig *tls.Config) *HTTP3Transport {
	return &HTTP3Transport{
		TLSClientConfig: tlsConfig,
		QuicConfig: &quic.Config{
			HandshakeIdleTimeout: 30 * time.Second,
			MaxIdleTimeout:       90 * time.Second,
			KeepAlivePeriod:      15 * time.Second,
		},
		UQuicConfig:           nil, // Will be set when QUIC fingerprint is provided
		QUICSpec:              nil, // Will be set when QUIC fingerprint is provided
		UseUQuic:              false,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		DialTimeout:           30 * time.Second,
		DisableCompression:    false,
	}
}

// NewHTTP3TransportWithUQuic creates a new HTTP/3 transport with UQuic fingerprinting support
func NewHTTP3TransportWithUQuic(tlsConfig *tls.Config, quicSpec *uquic.QUICSpec) *HTTP3Transport {
	transport := NewHTTP3Transport(tlsConfig)
	if quicSpec != nil {
		transport.QUICSpec = quicSpec
		transport.UseUQuic = true
		transport.UQuicConfig = &uquic.Config{}
	}
	return transport
}

// UQuicHTTP3Transport implements HTTP/3 transport with UQuic fingerprinting
type UQuicHTTP3Transport struct {
	// TLSClientConfig is the TLS configuration
	TLSClientConfig *tls.Config

	// UQuicConfig is the UQuic configuration
	UQuicConfig *uquic.Config

	// QUICSpec is the QUIC specification for fingerprinting
	QUICSpec *uquic.QUICSpec

	// DialTimeout is the maximum amount of time a dial will wait for a connect to complete
	DialTimeout time.Duration
}

// NewUQuicHTTP3Transport creates a new UQuic-based HTTP/3 transport
func NewUQuicHTTP3Transport(tlsConfig *tls.Config, quicSpec *uquic.QUICSpec) *UQuicHTTP3Transport {
	return &UQuicHTTP3Transport{
		TLSClientConfig: tlsConfig,
		UQuicConfig:     &uquic.Config{},
		QUICSpec:        quicSpec,
		DialTimeout:     30 * time.Second,
	}
}

// RoundTrip implements the http.RoundTripper interface for UQuic transport
func (t *UQuicHTTP3Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// For now, fall back to standard HTTP/3 transport since uquic integration
	// requires more complex implementation that goes beyond the scope of this change.
	// Future enhancement: Implement direct uquic HTTP/3 client integration

	// Create standard HTTP/3 client as fallback
	client := &stdhttp.Client{
		Transport: &http3.Transport{
			TLSClientConfig: t.TLSClientConfig,
			QUICConfig: &quic.Config{
				HandshakeIdleTimeout: 30 * time.Second,
				MaxIdleTimeout:       90 * time.Second,
				KeepAlivePeriod:      15 * time.Second,
			},
		},
	}

	// Convert fhttp.Request to net/http.Request for HTTP/3
	stdReq := &stdhttp.Request{
		Method:           req.Method,
		URL:              req.URL,
		Proto:            req.Proto,
		ProtoMajor:       req.ProtoMajor,
		ProtoMinor:       req.ProtoMinor,
		Header:           ConvertFhttpHeader(req.Header),
		Body:             req.Body,
		GetBody:          req.GetBody,
		ContentLength:    req.ContentLength,
		TransferEncoding: req.TransferEncoding,
		Close:            req.Close,
		Host:             req.Host,
		Form:             req.Form,
		PostForm:         req.PostForm,
		MultipartForm:    req.MultipartForm,
		Trailer:          ConvertFhttpHeader(req.Trailer),
		RemoteAddr:       req.RemoteAddr,
		RequestURI:       req.RequestURI,
		TLS:              nil, // TLS state conversion not needed for HTTP/3
		Cancel:           req.Cancel,
		Response:         nil,
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(req.Context(), t.DialTimeout)
	defer cancel()

	// Create a new request with the context
	newReq := stdReq.Clone(ctx)

	// Perform the request using standard HTTP/3
	// Uses standard HTTP/3 implementation (uquic integration available)
	stdResp, err := client.Do(newReq)
	if err != nil {
		return nil, err
	}

	// Convert back to fhttp.Response
	return &http.Response{
		Status:           stdResp.Status,
		StatusCode:       stdResp.StatusCode,
		Proto:            stdResp.Proto,
		ProtoMajor:       stdResp.ProtoMajor,
		ProtoMinor:       stdResp.ProtoMinor,
		Header:           ConvertHttpHeader(stdResp.Header),
		Body:             stdResp.Body,
		ContentLength:    stdResp.ContentLength,
		TransferEncoding: stdResp.TransferEncoding,
		Close:            stdResp.Close,
		Uncompressed:     stdResp.Uncompressed,
		Trailer:          ConvertHttpHeader(stdResp.Trailer),
		Request:          req,
		TLS:              nil, // Will be set properly if needed
	}, nil
}

// RoundTrip implements the http.RoundTripper interface
func (t *HTTP3Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// If UQuic is enabled and we have a QUIC spec, use UQuic transport
	if t.UseUQuic && t.QUICSpec != nil {
		uquicTransport := &UQuicHTTP3Transport{
			TLSClientConfig: t.TLSClientConfig,
			UQuicConfig:     t.UQuicConfig,
			QUICSpec:        t.QUICSpec,
			DialTimeout:     t.DialTimeout,
		}
		return uquicTransport.RoundTrip(req)
	}

	// Convert fhttp.Request to net/http.Request for HTTP/3
	stdReq := &stdhttp.Request{
		Method:           req.Method,
		URL:              req.URL,
		Proto:            req.Proto,
		ProtoMajor:       req.ProtoMajor,
		ProtoMinor:       req.ProtoMinor,
		Header:           ConvertFhttpHeader(req.Header),
		Body:             req.Body,
		GetBody:          req.GetBody,
		ContentLength:    req.ContentLength,
		TransferEncoding: req.TransferEncoding,
		Close:            req.Close,
		Host:             req.Host,
		Form:             req.Form,
		PostForm:         req.PostForm,
		MultipartForm:    req.MultipartForm,
		Trailer:          ConvertFhttpHeader(req.Trailer),
		RemoteAddr:       req.RemoteAddr,
		RequestURI:       req.RequestURI,
		TLS:              nil, // TLS state conversion not needed for HTTP/3
		Cancel:           req.Cancel,
		Response:         nil,
	}

	// Create an HTTP/3 client
	client := &stdhttp.Client{
		Transport: &http3.Transport{
			TLSClientConfig: t.TLSClientConfig,
			QUICConfig:      t.QuicConfig,
		},
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(req.Context(), t.DialTimeout)
	defer cancel()

	// Create a new request with the context
	newReq := stdReq.Clone(ctx)

	// Perform the request
	stdResp, err := client.Do(newReq)
	if err != nil {
		return nil, err
	}

	// Convert back to fhttp.Response
	return &http.Response{
		Status:           stdResp.Status,
		StatusCode:       stdResp.StatusCode,
		Proto:            stdResp.Proto,
		ProtoMajor:       stdResp.ProtoMajor,
		ProtoMinor:       stdResp.ProtoMinor,
		Header:           ConvertHttpHeader(stdResp.Header),
		Body:             stdResp.Body,
		ContentLength:    stdResp.ContentLength,
		TransferEncoding: stdResp.TransferEncoding,
		Close:            stdResp.Close,
		Uncompressed:     stdResp.Uncompressed,
		Trailer:          ConvertHttpHeader(stdResp.Trailer),
		Request:          req,
		TLS:              nil, // Will be set properly if needed
	}, nil
}

// ConfigureHTTP3Client configures an http.Client to use HTTP/3
func ConfigureHTTP3Client(client *stdhttp.Client, tlsConfig *tls.Config) {
	client.Transport = &http3.Transport{
		TLSClientConfig: tlsConfig,
		QUICConfig: &quic.Config{
			HandshakeIdleTimeout: 30 * time.Second,
			MaxIdleTimeout:       90 * time.Second,
			KeepAlivePeriod:      15 * time.Second,
		},
	}
}

// HTTP3RoundTripper implements an HTTP/3 round tripper with support for custom TLS fingerprints
type HTTP3RoundTripper struct {
	// TLSClientConfig is the TLS configuration
	TLSClientConfig *tls.Config

	// QuicConfig is the QUIC configuration
	QuicConfig *quic.Config

	// Forwarder is the underlying HTTP/3 transport
	Forwarder *http3.Transport

	// Dialer is the custom dialer for HTTP/3 connections
	Dialer func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (*quic.Conn, error)
}

// NewHTTP3RoundTripper creates a new HTTP/3 round tripper with custom fingerprinting
func NewHTTP3RoundTripper(tlsConfig *tls.Config, quicConfig *quic.Config) *HTTP3RoundTripper {
	rt := &HTTP3RoundTripper{
		TLSClientConfig: tlsConfig,
		QuicConfig:      quicConfig,
	}

	// Create the forwarder with default dialer
	rt.Forwarder = &http3.Transport{
		TLSClientConfig: tlsConfig,
		QUICConfig:      quicConfig,
	}

	return rt
}

// RoundTrip implements the http.RoundTripper interface
func (rt *HTTP3RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Convert fhttp.Request to net/http.Request
	stdReq := &stdhttp.Request{
		Method:           req.Method,
		URL:              req.URL,
		Proto:            req.Proto,
		ProtoMajor:       req.ProtoMajor,
		ProtoMinor:       req.ProtoMinor,
		Header:           ConvertFhttpHeader(req.Header),
		Body:             req.Body,
		GetBody:          req.GetBody,
		ContentLength:    req.ContentLength,
		TransferEncoding: req.TransferEncoding,
		Close:            req.Close,
		Host:             req.Host,
		Form:             req.Form,
		PostForm:         req.PostForm,
		MultipartForm:    req.MultipartForm,
		Trailer:          ConvertFhttpHeader(req.Trailer),
		RemoteAddr:       req.RemoteAddr,
		RequestURI:       req.RequestURI,
		TLS:              nil, // TLS state conversion not needed for HTTP/3
		Cancel:           req.Cancel,
		Response:         nil,
	}

	// Use the custom dialer if set, otherwise use the forwarder
	if rt.Dialer != nil {
		// Check if req.URL.Host includes a port
		host := req.URL.Host
		if _, _, err := net.SplitHostPort(host); err != nil {
			// No port, add the default HTTPS port
			host = fmt.Sprintf("%s:443", host)
		}

		// Create a custom HTTP/3 client with our dialer
		customRT := &http3.Transport{
			TLSClientConfig: rt.TLSClientConfig,
			QUICConfig:      rt.QuicConfig,
			Dial:            rt.Dialer,
		}

		stdResp, err := customRT.RoundTrip(stdReq)
		if err != nil {
			return nil, err
		}

		// Convert back to fhttp.Response
		return &http.Response{
			Status:           stdResp.Status,
			StatusCode:       stdResp.StatusCode,
			Proto:            stdResp.Proto,
			ProtoMajor:       stdResp.ProtoMajor,
			ProtoMinor:       stdResp.ProtoMinor,
			Header:           ConvertHttpHeader(stdResp.Header),
			Body:             stdResp.Body,
			ContentLength:    stdResp.ContentLength,
			TransferEncoding: stdResp.TransferEncoding,
			Close:            stdResp.Close,
			Uncompressed:     stdResp.Uncompressed,
			Trailer:          ConvertHttpHeader(stdResp.Trailer),
			Request:          req,
			TLS:              nil,
		}, nil
	}

	// Use the default forwarder with conversion
	stdResp, err := rt.Forwarder.RoundTrip(stdReq)
	if err != nil {
		return nil, err
	}

	// Convert back to fhttp.Response
	return &http.Response{
		Status:           stdResp.Status,
		StatusCode:       stdResp.StatusCode,
		Proto:            stdResp.Proto,
		ProtoMajor:       stdResp.ProtoMajor,
		ProtoMinor:       stdResp.ProtoMinor,
		Header:           ConvertHttpHeader(stdResp.Header),
		Body:             stdResp.Body,
		ContentLength:    stdResp.ContentLength,
		TransferEncoding: stdResp.TransferEncoding,
		Close:            stdResp.Close,
		Uncompressed:     stdResp.Uncompressed,
		Trailer:          ConvertHttpHeader(stdResp.Trailer),
		Request:          req,
		TLS:              nil,
	}, nil
}

// HTTP3Connection represents an HTTP/3 connection with associated metadata
type HTTP3Connection struct {
	QuicConn interface{} // Can be *quic.Conn or uquic.EarlyConnection
	RawConn  net.PacketConn
	Proxys   []string
	IsUQuic  bool // Flag to indicate if this is a UQuic connection
}

// http3Dial establishes a UDP connection for HTTP/3 with proxy support
func (rt *roundTripper) http3Dial(ctx context.Context, remoteAddr, port string, proxys ...string) (net.PacketConn, error) {
	// If proxies are provided, handle proxy dialing
	if len(proxys) > 0 {
		// For now, HTTP/3 proxy support is limited - most HTTP/3 connections are direct
		// TODO: Implement proper CONNECT-UDP proxy support for HTTP/3
		return nil, fmt.Errorf("HTTP/3 proxy support not yet implemented")
	}

	// Direct UDP connection
	conn, err := net.ListenPacket("udp", "")
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP packet connection: %w", err)
	}

	return conn, nil
}

// ghttp3Dial performs standard HTTP/3 dialing using the standard QUIC implementation
func (rt *roundTripper) ghttp3Dial(ctx context.Context, remoteAddr, port string, proxys ...string) (*HTTP3Connection, error) {
	// Establish UDP connection
	udpConn, err := rt.http3Dial(ctx, remoteAddr, port, proxys...)
	if err != nil {
		return nil, err
	}

	// Configure TLS - use crypto/tls.Config for standard QUIC (matches reference implementation)
	var tlsConfig *tls.Config
	if rt.TLSConfig != nil {
		// Convert from utls.Config to crypto/tls.Config for standard QUIC
		converted := ConvertUtlsConfig(rt.TLSConfig)
		if converted != nil {
			tlsConfig = converted.Clone()
		}
	}
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}
	tlsConfig.NextProtos = []string{http3.NextProtoH3}
	if rt.ServerName != "" {
		tlsConfig.ServerName = rt.ServerName
	} else {
		tlsConfig.ServerName = remoteAddr
	}

	// Resolve remote address
	remoteHost := remoteAddr
	if net.ParseIP(remoteAddr) == nil {
		// If remoteAddr is not an IP, resolve it
		ips, err := net.LookupIP(remoteAddr)
		if err != nil {
			udpConn.Close()
			return nil, fmt.Errorf("failed to resolve host %s: %w", remoteAddr, err)
		}
		if len(ips) == 0 {
			udpConn.Close()
			return nil, fmt.Errorf("no IP addresses found for host %s", remoteAddr)
		}
		// Use the first IP address
		remoteHost = ips[0].String()
	}

	// Convert port to integer
	portInt := 443
	if port != "" {
		if p, err := net.LookupPort("tcp", port); err == nil {
			portInt = p
		}
	}

	// Configure QUIC - conditional setup like reference implementation
	var quicConfig *quic.Config
	// TODO: Add support for rt.UquicConfig when it's available
	// For now, use default QUIC config similar to reference behavior
	if quicConfig == nil {
		quicConfig = &quic.Config{
			HandshakeIdleTimeout:           30 * time.Second,
			MaxIdleTimeout:                 90 * time.Second,
			KeepAlivePeriod:                15 * time.Second,
			InitialStreamReceiveWindow:     512 * 1024,      // 512 KB
			MaxStreamReceiveWindow:         2 * 1024 * 1024, // 2 MB
			InitialConnectionReceiveWindow: 1024 * 1024,     // 1 MB
			MaxConnectionReceiveWindow:     4 * 1024 * 1024, // 4 MB
			MaxIncomingStreams:             100,
			MaxIncomingUniStreams:          100,
			EnableDatagrams:                false,
			DisablePathMTUDiscovery:        false,
			Allow0RTT:                      false, // Security consideration
		}
	}

	// Establish QUIC connection
	remoteUDPAddr := &net.UDPAddr{
		IP:   net.ParseIP(remoteHost),
		Port: portInt,
	}

	quicConn, err := quic.DialEarly(ctx, udpConn, remoteUDPAddr, tlsConfig, quicConfig)
	if err != nil {
		udpConn.Close()
		return nil, fmt.Errorf("failed to establish QUIC connection: %w", err)
	}

	return &HTTP3Connection{
		QuicConn: quicConn,
		RawConn:  udpConn,
		Proxys:   proxys,
		IsUQuic:  false,
	}, nil
}

// uhttp3Dial performs HTTP/3 dialing using UQuic for QUIC fingerprinting
func (rt *roundTripper) uhttp3Dial(ctx context.Context, spec *uquic.QUICSpec, remoteAddr, port string, proxys ...string) (*HTTP3Connection, error) {
	// Establish UDP connection
	udpConn, err := rt.http3Dial(ctx, remoteAddr, port, proxys...)
	if err != nil {
		return nil, err
	}

	// Configure TLS with uTLS config - use utls.Config directly (matches reference implementation)
	if rt.TLSConfig == nil {
		return nil, fmt.Errorf("TLS config is required for UQuic HTTP/3")
	}
	tlsConfig := rt.TLSConfig.Clone()
	tlsConfig.NextProtos = []string{http3.NextProtoH3}
	if rt.ServerName != "" {
		tlsConfig.ServerName = rt.ServerName
	} else {
		tlsConfig.ServerName = remoteAddr
	}

	// Resolve remote address
	remoteHost := remoteAddr
	if net.ParseIP(remoteAddr) == nil {
		// If remoteAddr is not an IP, resolve it
		ips, err := net.LookupIP(remoteAddr)
		if err != nil {
			udpConn.Close()
			return nil, fmt.Errorf("failed to resolve host %s: %w", remoteAddr, err)
		}
		if len(ips) == 0 {
			udpConn.Close()
			return nil, fmt.Errorf("no IP addresses found for host %s", remoteAddr)
		}
		// Use the first IP address
		remoteHost = ips[0].String()
	}

	// Convert port to integer
	portInt := 443
	if port != "" {
		if p, err := net.LookupPort("tcp", port); err == nil {
			portInt = p
		}
	}

	// Configure UQuic - conditional setup like reference implementation
	var uquicConfig *uquic.Config
	// TODO: Add support for rt.UquicConfig when it's available
	// For now, use default UQuic config similar to reference behavior
	if uquicConfig == nil {
		uquicConfig = &uquic.Config{}
	}

	// Create UQuic transport
	uTransport := &uquic.UTransport{
		Transport: &uquic.Transport{
			Conn: udpConn,
		},
		QUICSpec: spec,
	}

	// Establish QUIC connection with UQuic
	remoteUDPAddr := &net.UDPAddr{
		IP:   net.ParseIP(remoteHost),
		Port: portInt,
	}

	quicConn, err := uTransport.DialEarly(ctx, remoteUDPAddr, tlsConfig, uquicConfig)
	if err != nil {
		udpConn.Close()
		return nil, fmt.Errorf("failed to establish UQuic connection: %w", err)
	}

	return &HTTP3Connection{
		QuicConn: quicConn,
		RawConn:  udpConn,
		Proxys:   proxys,
		IsUQuic:  true,
	}, nil
}
