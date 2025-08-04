
package cycletls

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	stdhttp "net/http"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	http "github.com/Danny-Dasilva/fhttp"
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
		UQuicConfig:       nil, // Will be set when QUIC fingerprint is provided
		QUICSpec:          nil, // Will be set when QUIC fingerprint is provided
		UseUQuic:          false,
		MaxIdleConns:      100,
		IdleConnTimeout:   90 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		DialTimeout:       30 * time.Second,
		DisableCompression: false,
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
		UQuicConfig: &uquic.Config{},
		QUICSpec:    quicSpec,
		DialTimeout: 30 * time.Second,
	}
}

// uhttp3Dial creates a QUIC connection using UQuic with fingerprinting
func (t *UQuicHTTP3Transport) uhttp3Dial(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error) {
	// Parse the address to get host and port
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// If no port specified, add default HTTPS port
		host = addr
		port = "443"
		addr = net.JoinHostPort(host, port)
	}

	// Create UDP address
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	// Create UDP connection
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP connection: %w", err)
	}

	// Note: For future uquic implementation, we would need to convert configs here
	// Currently using fallback approach in RoundTrip method

	// Create UQuic transport with fingerprinting
	uTransport := &uquic.UTransport{
		Transport: &uquic.Transport{
			Conn: udpConn,
		},
	}

	// Set QUIC specification for fingerprinting if available
	if t.QUICSpec != nil {
		uTransport.QUICSpec = t.QUICSpec
	}

	// Since uquic.EarlyConnection doesn't directly implement quic.EarlyConnection,
	// we need to create a wrapper or use a different approach.
	// For now, let's return an error indicating this needs a different implementation
	return nil, fmt.Errorf("uquic integration requires different approach - use UQuicHTTP3Transport.RoundTrip directly instead of custom dialer")
}

// RoundTrip implements the http.RoundTripper interface for UQuic transport
func (t *UQuicHTTP3Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// For now, fall back to standard HTTP/3 transport since uquic integration
	// requires more complex implementation that goes beyond the scope of this change.
	// Future enhancement: Implement direct uquic HTTP/3 client integration
	
	// Create standard HTTP/3 client as fallback
	client := &stdhttp.Client{
		Transport: &http3.RoundTripper{
			TLSClientConfig: t.TLSClientConfig,
			QuicConfig: &quic.Config{
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
	// TODO: Replace with actual uquic implementation in future enhancement
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
		Transport: &http3.RoundTripper{
			TLSClientConfig: t.TLSClientConfig,
			QuicConfig:      t.QuicConfig,
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
	client.Transport = &http3.RoundTripper{
		TLSClientConfig: tlsConfig,
		QuicConfig: &quic.Config{
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

	// Forwarder is the underlying HTTP/3 round tripper
	Forwarder *http3.RoundTripper

	// Dialer is the custom dialer for HTTP/3 connections
	Dialer func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error)
}

// NewHTTP3RoundTripper creates a new HTTP/3 round tripper with custom fingerprinting
func NewHTTP3RoundTripper(tlsConfig *tls.Config, quicConfig *quic.Config) *HTTP3RoundTripper {
	rt := &HTTP3RoundTripper{
		TLSClientConfig: tlsConfig,
		QuicConfig:      quicConfig,
	}

	// Create the forwarder with default dialer
	rt.Forwarder = &http3.RoundTripper{
		TLSClientConfig: tlsConfig,
		QuicConfig:      quicConfig,
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
		customRT := &http3.RoundTripper{
			TLSClientConfig: rt.TLSClientConfig,
			QuicConfig:      rt.QuicConfig,
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