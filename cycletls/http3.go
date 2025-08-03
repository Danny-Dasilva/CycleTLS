package cycletls

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// HTTP3Transport represents an HTTP/3 transport with customizable settings
type HTTP3Transport struct {
	// QuicConfig is the QUIC configuration
	QuicConfig *quic.Config

	// TLSClientConfig is the TLS configuration
	TLSClientConfig *tls.Config

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
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		DialTimeout:         30 * time.Second,
		DisableCompression:  false,
	}
}

// RoundTrip implements the http.RoundTripper interface
func (t *HTTP3Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create an HTTP/3 client
	client := &http.Client{
		Transport: &http3.RoundTripper{
			TLSClientConfig: t.TLSClientConfig,
			QuicConfig:      t.QuicConfig,
		},
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(req.Context(), t.DialTimeout)
	defer cancel()

	// Create a new request with the context
	newReq := req.Clone(ctx)

	// Perform the request
	return client.Do(newReq)
}

// ConfigureHTTP3Client configures an http.Client to use HTTP/3
func ConfigureHTTP3Client(client *http.Client, tlsConfig *tls.Config) {
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

		return customRT.RoundTrip(req)
	}

	// Use the default forwarder
	return rt.Forwarder.RoundTrip(req)
}