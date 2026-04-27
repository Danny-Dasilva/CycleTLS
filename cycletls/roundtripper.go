package cycletls

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	http "github.com/Danny-Dasilva/fhttp"
	http2 "github.com/Danny-Dasilva/fhttp/http2"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	uquic "github.com/refraction-networking/uquic"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/proxy"
	"net"
	stdhttp "net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var errProtocolNegotiated = errors.New("protocol negotiated")

// Cache configuration constants
const (
	maxCachedConnections = 100
	maxCachedTransports  = 100
	cacheCleanupInterval = 5 * time.Minute
	connectionMaxAge     = 10 * time.Minute
)

// cachedConn wraps a net.Conn with LRU tracking
type cachedConn struct {
	conn     net.Conn
	lastUsed time.Time
}

// cachedTransport wraps an http.RoundTripper with LRU tracking
type cachedTransport struct {
	transport http.RoundTripper
	lastUsed  time.Time
}

// cachedHTTP3Transport wraps an http3.Transport with LRU tracking and connection info
type cachedHTTP3Transport struct {
	transport *http3.Transport
	conn      *HTTP3Connection // Keep reference for cleanup
	lastUsed  time.Time
}

type roundTripper struct {
	sync.Mutex

	// Per-address mutexes for preventing concurrent transport creation
	addressMutexes   map[string]*sync.Mutex
	addressMutexLock sync.Mutex

	// TLS fingerprinting options
	JA3              string
	JA4r             string // JA4 raw format with explicit cipher/extension values
	HTTP2Fingerprint string
	QUICFingerprint  string
	USpec            *uquic.QUICSpec // UQuic QUIC specification for HTTP3 fingerprinting
	DisableGrease    bool

	// Browser identification
	UserAgent   string
	HeaderOrder []string

	// Connection options
	TLSConfig          *utls.Config
	InsecureSkipVerify bool
	ServerName         string
	Cookies            []Cookie
	ForceHTTP1         bool
	ForceHTTP3         bool

	// TLS 1.3 specific options
	TLS13AutoRetry bool

	// Caching with LRU eviction
	cachedConnections     map[string]*cachedConn
	cachedTransports      map[string]*cachedTransport
	cachedHTTP3Transports map[string]*cachedHTTP3Transport
	cacheMu               sync.RWMutex
	cleanupOnce           sync.Once
	cleanupStop           chan struct{}

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

	// Apply header order if specified (for regular headers, not pseudo-headers).
	//
	// When HeaderOrder is non-empty we:
	//   1. Place the user-agent under the lowercase key so fhttp's header-order
	//      logic can position it deterministically.
	//   2. Run the request headers through MarshalHeader for case-insensitive
	//      ordering, then convert back via ConvertHttpHeader.
	//   3. Pin the order via the http.HeaderOrderKey sentinel.
	//
	// When HeaderOrder is empty we preserve the legacy behaviour of setting
	// the canonical "User-Agent" key and leaving the rest of the headers
	// untouched, which avoids surprising case changes for callers that did
	// not opt into ordered headers.
	//
	// Pseudo-header order (http.PHeaderOrderKey) is set in index.go based on
	// UserAgent parsing and must not be overwritten here.
	if len(rt.HeaderOrder) > 0 {
		delete(req.Header, "User-Agent")
		delete(req.Header, "user-agent")
		req.Header["user-agent"] = []string{rt.UserAgent}

		req.Header = ConvertHttpHeader(MarshalHeader(req.Header, rt.HeaderOrder))
		req.Header[http.HeaderOrderKey] = rt.HeaderOrder
	} else {
		req.Header.Set("User-Agent", rt.UserAgent)
	}

	// Get address for dialing
	addr := rt.getDialTLSAddr(req)

	// Start cleanup goroutine on first use (needed for both HTTP/2 and HTTP/3)
	rt.cleanupOnce.Do(func() {
		rt.cleanupStop = make(chan struct{})
		go rt.startCacheCleanup()
	})

	// Check if we need HTTP/3 - matches reference implementation pattern
	if rt.ForceHTTP3 {
		// Extract host and port from request
		host := req.URL.Hostname()
		port := req.URL.Port()
		if port == "" {
			port = "443" // Default HTTPS port
		}

		// Create cache key for HTTP/3 transport (use h3: prefix to distinguish from HTTP/2)
		http3CacheKey := "h3:" + net.JoinHostPort(host, port)

		// Check for cached HTTP/3 transport
		// Use a single Lock() for check-and-update to avoid TOCTOU race
		rt.cacheMu.Lock()
		cachedH3, hasCached := rt.cachedHTTP3Transports[http3CacheKey]
		if hasCached {
			cachedH3.lastUsed = time.Now()
			transport := cachedH3.transport
			rt.cacheMu.Unlock()
			return rt.makeHTTP3RequestWithTransport(req, transport)
		}
		rt.cacheMu.Unlock()

		// No cached transport, need to create new connection
		// Check for USpec (matches reference implementation logic)
		if rt.USpec != nil {
			// Use UQuic-based HTTP/3 dialing
			conn, err := rt.uhttp3Dial(req.Context(), rt.USpec, host, port)
			if err != nil {
				return nil, fmt.Errorf("uhttp3 dial failed: %w", err)
			}
			// Connection will be cached by makeHTTP3Request, cleanup handled by LRU
			return rt.makeHTTP3Request(req, conn, http3CacheKey)
		}

		// Fall back to standard HTTP/3 dialing
		conn, err := rt.ghttp3Dial(req.Context(), host, port)
		if err != nil {
			return nil, fmt.Errorf("ghttp3 dial failed: %w", err)
		}
		// Connection will be cached by makeHTTP3Request, cleanup handled by LRU
		return rt.makeHTTP3Request(req, conn, http3CacheKey)
	}

	// Use cached transport if available, otherwise create a new one
	// Use a single Lock() for check-and-update to avoid TOCTOU race
	rt.cacheMu.Lock()
	ct, ok := rt.cachedTransports[addr]
	if ok {
		ct.lastUsed = time.Now()
	}
	rt.cacheMu.Unlock()

	if !ok {
		if err := rt.getTransport(req, addr); err != nil {
			return nil, err
		}
		rt.cacheMu.RLock()
		ct = rt.cachedTransports[addr]
		rt.cacheMu.RUnlock()
	}

	// Perform the request
	return ct.transport.RoundTrip(req)
}

func (rt *roundTripper) getTransport(req *http.Request, addr string) error {
	switch strings.ToLower(req.URL.Scheme) {
	case "http":
		// Allow connection reuse by removing DisableKeepAlives
		rt.cacheMu.Lock()
		rt.cachedTransports[addr] = &cachedTransport{
			transport: &http.Transport{
				DialContext: rt.dialer.DialContext,
			},
			lastUsed: time.Now(),
		}
		rt.cacheMu.Unlock()
		return nil
	case "https":
	default:
		return fmt.Errorf("invalid URL scheme: [%v]", req.URL.Scheme)
	}

	// Use per-address mutex to serialize transport creation and avoid races
	addressMutex := rt.getAddressMutex(addr)
	addressMutex.Lock()
	defer addressMutex.Unlock()

	// Double-check if transport was created while we were waiting for the lock
	rt.cacheMu.RLock()
	_, exists := rt.cachedTransports[addr]
	rt.cacheMu.RUnlock()
	if exists {
		// Another goroutine already created the transport
		return nil
	}

	// Establish TLS connection
	_, err := rt.dialTLS(req.Context(), "tcp", addr)
	switch err {
	case errProtocolNegotiated:
		// Transport and connection have been negotiated and cached by dialTLS
		// Nothing else to do here.
	case nil:
		// A cached connection/transport already exists (e.g., created by another goroutine).
		// Treat as success instead of panicking.
		// No action needed; RoundTrip will use the cached transport.
	default:
		return err
	}

	return nil
}

func (rt *roundTripper) dialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	rt.Lock()
	defer rt.Unlock()

	// Return cached connection if available
	// Use a single Lock() for check-and-update to avoid TOCTOU race
	// (RLock->RUnlock->Lock allows another goroutine to delete the entry between locks)
	rt.cacheMu.Lock()
	if cc := rt.cachedConnections[addr]; cc != nil {
		cc.lastUsed = time.Now()
		rt.cacheMu.Unlock()
		return cc.conn, nil
	}
	rt.cacheMu.Unlock()

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
	// Determine SNI to use (custom serverName takes precedence)
	serverName := host
	if rt.ServerName != "" {
		serverName = rt.ServerName
	}

	var spec *utls.ClientHelloSpec
	var helloID utls.ClientHelloID
	var proactivelyUpgraded bool // Track if we proactively upgraded TLS 1.2 to 1.3

	// Determine which fingerprint to use
	if rt.QUICFingerprint != "" {
		// Use QUIC fingerprint
		spec, err = QUICStringToSpec(rt.QUICFingerprint, rt.UserAgent, rt.ForceHTTP1)
		if err != nil {
			return nil, err
		}
	} else if rt.JA3 != "" {
		// Check if we should proactively upgrade TLS 1.2 to TLS 1.3
		if rt.TLS13AutoRetry && strings.HasPrefix(rt.JA3, "771,") {
			// Use TLS 1.3 compatible spec to avoid retry cycle
			spec, err = StringToTLS13CompatibleSpec(rt.JA3, rt.UserAgent, rt.ForceHTTP1)
			proactivelyUpgraded = true
		} else {
			// Use original JA3 fingerprint
			spec, err = StringToSpec(rt.JA3, rt.UserAgent, rt.ForceHTTP1)
		}
		if err != nil {
			return nil, err
		}
	} else if rt.JA4r != "" {
		// Use JA4r (raw) fingerprint
		spec, err = JA4RStringToSpec(rt.JA4r, rt.UserAgent, rt.ForceHTTP1, rt.DisableGrease, serverName)
		if err != nil {
			return nil, err
		}
	} else {
		parsedUserAgent := parseUserAgent(rt.UserAgent)
		switch parsedUserAgent.UserAgent {
		case chrome:
			spec = ModernChromeSpec(rt.ForceHTTP1)
		case firefox:
			helloID = utls.HelloFirefox_Auto
		default:
			spec, err = StringToSpec(DefaultChrome_JA3, rt.UserAgent, rt.ForceHTTP1)
			if err != nil {
				return nil, err
			}
		}
	}

	// Create TLS client
	clientHelloID := utls.HelloCustom
	if helloID.Client != "" {
		clientHelloID = helloID
	}
	conn := utls.UClient(rawConn, &utls.Config{
		ServerName:         serverName,
		OmitEmptyPsk:       true,
		InsecureSkipVerify: rt.InsecureSkipVerify,
	}, clientHelloID)

	// Apply TLS fingerprint
	if spec != nil {
		if err := conn.ApplyPreset(spec); err != nil {
			return nil, err
		}
	}

	// Perform TLS handshake
	if err = conn.Handshake(); err != nil {
		_ = conn.Close()

		if err.Error() == "tls: CurvePreferences includes unsupported curve" {
			// Check if TLS 1.3 retry is enabled
			if rt.TLS13AutoRetry {
				// Automatically retry with TLS 1.3 compatible curves
				return rt.retryWithTLS13CompatibleCurves(ctx, network, addr, host)
			}
			return nil, fmt.Errorf("conn.Handshake() error for TLS 1.3 (retry disabled): %+v", err)
		}

		// If we proactively upgraded to TLS 1.3 and it failed, try falling back to original TLS 1.2 JA3
		if proactivelyUpgraded && rt.JA3 != "" {
			return rt.retryWithOriginalTLS12JA3(ctx, network, addr, host)
		}

		return nil, fmt.Errorf("uTlsConn.Handshake() error: %+v", err)
	}

	// If transport already exists, return connection
	rt.cacheMu.RLock()
	existingTransport := rt.cachedTransports[addr]
	rt.cacheMu.RUnlock()
	if existingTransport != nil {
		return conn, nil
	}

	// Cache transport and connection
	if err := rt.cacheTransportAndConnection(addr, conn, time.Now()); err != nil {
		return nil, err
	}

	return nil, errProtocolNegotiated
}

// retryWithTLS13CompatibleCurves retries the TLS connection with TLS 1.3 compatible curves
func (rt *roundTripper) retryWithTLS13CompatibleCurves(ctx context.Context, network, addr, host string) (net.Conn, error) {
	rawConn, err := rt.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	// Use TLS 1.3 compatible spec based on the original fingerprint type
	var spec *utls.ClientHelloSpec
	switch {
	case rt.QUICFingerprint != "":
		spec, err = QUICStringToSpec(rt.QUICFingerprint, rt.UserAgent, rt.ForceHTTP1)
	case rt.JA3 != "":
		spec, err = StringToTLS13CompatibleSpec(rt.JA3, rt.UserAgent, rt.ForceHTTP1)
	default:
		// For JA4r or default, use TLS 1.3 compatible Chrome fingerprint
		spec, err = StringToTLS13CompatibleSpec(DefaultChrome_JA3, rt.UserAgent, rt.ForceHTTP1)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS 1.3 compatible spec: %v", err)
	}

	conn := utls.UClient(rawConn, &utls.Config{
		ServerName:         host,
		OmitEmptyPsk:       true,
		InsecureSkipVerify: rt.InsecureSkipVerify,
	}, utls.HelloCustom)

	if err := conn.ApplyPreset(spec); err != nil {
		return nil, fmt.Errorf("failed to apply TLS 1.3 compatible preset: %v", err)
	}

	if err = conn.Handshake(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("TLS 1.3 compatible handshake failed: %+v", err)
	}

	if err := rt.cacheTransportAndConnection(addr, conn, time.Now()); err != nil {
		return nil, err
	}

	return nil, errProtocolNegotiated
}

// retryWithOriginalTLS12JA3 retries the TLS connection with the original TLS 1.2 JA3
func (rt *roundTripper) retryWithOriginalTLS12JA3(ctx context.Context, network, addr, host string) (net.Conn, error) {
	rawConn, err := rt.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	spec, err := StringToSpec(rt.JA3, rt.UserAgent, rt.ForceHTTP1)
	if err != nil {
		return nil, fmt.Errorf("failed to create original TLS 1.2 JA3 spec: %v", err)
	}

	conn := utls.UClient(rawConn, &utls.Config{
		ServerName:         host,
		OmitEmptyPsk:       true,
		InsecureSkipVerify: rt.InsecureSkipVerify,
	}, utls.HelloCustom)

	if err := conn.ApplyPreset(spec); err != nil {
		return nil, fmt.Errorf("failed to apply original TLS 1.2 preset: %v", err)
	}

	if err = conn.Handshake(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("original TLS 1.2 handshake failed: %+v", err)
	}

	if err := rt.cacheTransportAndConnection(addr, conn, time.Now()); err != nil {
		return nil, err
	}

	return nil, errProtocolNegotiated
}

func (rt *roundTripper) dialTLSHTTP2(network, addr string, _ *utls.Config) (net.Conn, error) {
	return rt.dialTLS(context.Background(), network, addr)
}

// cacheTransportAndConnection creates and caches the appropriate transport based on negotiated protocol.
// This consolidates duplicate code from dialTLS, retryWithTLS13CompatibleCurves, and retryWithOriginalTLS12JA3.
func (rt *roundTripper) cacheTransportAndConnection(addr string, conn *utls.UConn, now time.Time) error {
	// Create appropriate transport based on negotiated protocol
	switch conn.ConnectionState().NegotiatedProtocol {
	case http2.NextProtoTLS:
		// HTTP/2 transport
		parsedUserAgent := parseUserAgent(rt.UserAgent)

		http2Transport := http2.Transport{
			DialTLS:     rt.dialTLSHTTP2,
			PushHandler: &http2.DefaultPushHandler{},
			Navigator:   parsedUserAgent.UserAgent,
		}

		// Apply HTTP/2 fingerprint if specified
		if rt.HTTP2Fingerprint != "" {
			h2Fingerprint, err := NewHTTP2Fingerprint(rt.HTTP2Fingerprint)
			if err != nil {
				return fmt.Errorf("failed to parse HTTP/2 fingerprint: %v", err)
			}
			h2Fingerprint.Apply(&http2Transport)
		}

		rt.cacheMu.Lock()
		rt.cachedTransports[addr] = &cachedTransport{
			transport: &http2Transport,
			lastUsed:  now,
		}
		rt.cacheMu.Unlock()
	default:
		// HTTP/1.x transport with keep-alives
		rt.cacheMu.Lock()
		rt.cachedTransports[addr] = &cachedTransport{
			transport: &http.Transport{
				DialTLSContext:      rt.dialTLS,
				DisableKeepAlives:   false,
				IdleConnTimeout:     90 * time.Second,
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
			},
			lastUsed: now,
		}
		rt.cacheMu.Unlock()
	}

	// Cache the connection
	rt.cacheMu.Lock()
	rt.cachedConnections[addr] = &cachedConn{
		conn:     conn,
		lastUsed: now,
	}
	rt.cacheMu.Unlock()

	return nil
}

func (rt *roundTripper) getDialTLSAddr(req *http.Request) string {
	host, port, err := net.SplitHostPort(req.URL.Host)
	if err == nil {
		return net.JoinHostPort(host, port)
	}
	return net.JoinHostPort(req.URL.Host, "443") // Default HTTPS port
}

// getAddressMutex returns a mutex for the specific address to serialize transport creation
func (rt *roundTripper) getAddressMutex(addr string) *sync.Mutex {
	rt.addressMutexLock.Lock()
	defer rt.addressMutexLock.Unlock()

	if rt.addressMutexes == nil {
		rt.addressMutexes = make(map[string]*sync.Mutex)
	}

	if mu, exists := rt.addressMutexes[addr]; exists {
		return mu
	}

	mu := &sync.Mutex{}
	rt.addressMutexes[addr] = mu
	return mu
}

// CloseIdleConnections closes connections that have been idle for too long
// If selectedAddr is provided, only close connections not matching this address
func (rt *roundTripper) CloseIdleConnections(selectedAddr ...string) {
	rt.Lock()
	defer rt.Unlock()

	rt.cacheMu.Lock()
	defer rt.cacheMu.Unlock()

	// If we have a specific address to keep, only close other connections
	if len(selectedAddr) > 0 && selectedAddr[0] != "" {
		addr := selectedAddr[0]
		// Keep the connection for the provided address, close others
		for connAddr, cc := range rt.cachedConnections {
			if connAddr != addr {
				if cc != nil && cc.conn != nil {
					// Ignore errors from closing - connection might already be closed
					_ = cc.conn.Close()
				}
				delete(rt.cachedConnections, connAddr)
				// Also clean up the transport cache for this address
				delete(rt.cachedTransports, connAddr)
			}
		}
		// Also handle HTTP/3 transports (with h3: prefix)
		h3Addr := "h3:" + addr
		for h3Key, h3t := range rt.cachedHTTP3Transports {
			if h3Key != h3Addr {
				if h3t.transport != nil {
					_ = h3t.transport.Close()
				}
				if h3t.conn != nil {
					rt.closeHTTP3Connection(h3t.conn)
				}
				delete(rt.cachedHTTP3Transports, h3Key)
			}
		}
	} else {
		// No address specified, close all connections (original behavior)
		for addr, cc := range rt.cachedConnections {
			if cc != nil && cc.conn != nil {
				// Ignore errors from closing - connection might already be closed
				_ = cc.conn.Close()
			}
			delete(rt.cachedConnections, addr)
			// Also clean up the transport cache for this address
			delete(rt.cachedTransports, addr)
		}
		// Close all HTTP/3 transports
		for key, h3t := range rt.cachedHTTP3Transports {
			if h3t.transport != nil {
				_ = h3t.transport.Close()
			}
			if h3t.conn != nil {
				rt.closeHTTP3Connection(h3t.conn)
			}
			delete(rt.cachedHTTP3Transports, key)
		}
		// Stop the cleanup goroutine since all connections are closed
		// and there is nothing left to manage.
		rt.StopCacheCleanup()
	}
}

// startCacheCleanup runs the periodic cache cleanup goroutine
func (rt *roundTripper) startCacheCleanup() {
	ticker := time.NewTicker(cacheCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rt.cleanupCache()
		case <-rt.cleanupStop:
			return
		}
	}
}

// cleanupCache removes expired entries and enforces LRU eviction
func (rt *roundTripper) cleanupCache() {
	rt.cacheMu.Lock()
	defer rt.cacheMu.Unlock()

	now := time.Now()

	// Clean old connections
	for key, cc := range rt.cachedConnections {
		if now.Sub(cc.lastUsed) > connectionMaxAge {
			if cc.conn != nil {
				_ = cc.conn.Close()
			}
			delete(rt.cachedConnections, key)
		}
	}

	// Clean old transports
	for key, ct := range rt.cachedTransports {
		if now.Sub(ct.lastUsed) > connectionMaxAge {
			delete(rt.cachedTransports, key)
		}
	}

	// Clean old HTTP/3 transports
	for key, h3t := range rt.cachedHTTP3Transports {
		if now.Sub(h3t.lastUsed) > connectionMaxAge {
			// Close the HTTP/3 transport
			if h3t.transport != nil {
				_ = h3t.transport.Close()
			}
			// Close the underlying QUIC connection
			if h3t.conn != nil {
				rt.closeHTTP3Connection(h3t.conn)
			}
			delete(rt.cachedHTTP3Transports, key)
		}
	}

	// Batch eviction: sort by lastUsed and remove excess in one pass (O(n log n))
	// instead of scanning for the oldest entry each iteration (O(n^2)).
	if excess := len(rt.cachedConnections) - maxCachedConnections; excess > 0 {
		type connEntry struct {
			key      string
			lastUsed time.Time
		}
		entries := make([]connEntry, 0, len(rt.cachedConnections))
		for k, v := range rt.cachedConnections {
			entries = append(entries, connEntry{k, v.lastUsed})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].lastUsed.Before(entries[j].lastUsed)
		})
		for i := 0; i < excess; i++ {
			key := entries[i].key
			if cc := rt.cachedConnections[key]; cc != nil && cc.conn != nil {
				_ = cc.conn.Close()
			}
			delete(rt.cachedConnections, key)
		}
	}

	// Batch eviction for transports
	if excess := len(rt.cachedTransports) - maxCachedTransports; excess > 0 {
		type transportEntry struct {
			key      string
			lastUsed time.Time
		}
		entries := make([]transportEntry, 0, len(rt.cachedTransports))
		for k, v := range rt.cachedTransports {
			entries = append(entries, transportEntry{k, v.lastUsed})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].lastUsed.Before(entries[j].lastUsed)
		})
		for i := 0; i < excess; i++ {
			delete(rt.cachedTransports, entries[i].key)
		}
	}

	// Batch eviction for HTTP/3 transports
	if excess := len(rt.cachedHTTP3Transports) - maxCachedTransports; excess > 0 {
		type h3Entry struct {
			key      string
			lastUsed time.Time
		}
		entries := make([]h3Entry, 0, len(rt.cachedHTTP3Transports))
		for k, v := range rt.cachedHTTP3Transports {
			entries = append(entries, h3Entry{k, v.lastUsed})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].lastUsed.Before(entries[j].lastUsed)
		})
		for i := 0; i < excess; i++ {
			key := entries[i].key
			h3t := rt.cachedHTTP3Transports[key]
			if h3t.transport != nil {
				_ = h3t.transport.Close()
			}
			if h3t.conn != nil {
				rt.closeHTTP3Connection(h3t.conn)
			}
			delete(rt.cachedHTTP3Transports, key)
		}
	}
}

// closeHTTP3Connection closes an HTTP/3 connection properly
func (rt *roundTripper) closeHTTP3Connection(conn *HTTP3Connection) {
	if conn.RawConn != nil {
		_ = conn.RawConn.Close()
	}
	if conn.QuicConn != nil {
		if conn.IsUQuic {
			if uquicConn, ok := conn.QuicConn.(interface{ CloseWithError(uint64, string) error }); ok {
				_ = uquicConn.CloseWithError(0, "connection closed")
			}
		} else {
			if quicConn, ok := conn.QuicConn.(interface{ CloseWithError(uint64, string) error }); ok {
				_ = quicConn.CloseWithError(0, "connection closed")
			}
		}
	}
}

// StopCacheCleanup stops the cache cleanup goroutine.
// Safe to call multiple times or when cleanup was never started.
func (rt *roundTripper) StopCacheCleanup() {
	if rt.cleanupStop == nil {
		return
	}
	// Use select to avoid panic on double-close
	select {
	case <-rt.cleanupStop:
		// Already closed, nothing to do
	default:
		close(rt.cleanupStop)
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
		dialer:                contextDialer,
		JA3:                   browser.JA3,
		JA4r:                  browser.JA4r,
		HTTP2Fingerprint:      browser.HTTP2Fingerprint,
		QUICFingerprint:       browser.QUICFingerprint,
		USpec:                 browser.USpec, // Add USpec field initialization
		DisableGrease:         browser.DisableGrease,
		UserAgent:             browser.UserAgent,
		HeaderOrder:           browser.HeaderOrder,
		TLSConfig:             browser.TLSConfig,
		ServerName:            browser.ServerName,
		Cookies:               browser.Cookies,
		cachedTransports:      make(map[string]*cachedTransport),
		cachedConnections:     make(map[string]*cachedConn),
		cachedHTTP3Transports: make(map[string]*cachedHTTP3Transport),
		InsecureSkipVerify:    browser.InsecureSkipVerify,
		ForceHTTP1:            browser.ForceHTTP1,
		ForceHTTP3:            browser.ForceHTTP3,

		// TLS 1.3 specific options
		TLS13AutoRetry: browser.TLS13AutoRetry,
	}
}

// makeHTTP3Request performs an HTTP/3 request and caches the transport for reuse
func (rt *roundTripper) makeHTTP3Request(req *http.Request, conn *HTTP3Connection, cacheKey string) (*http.Response, error) {
	// Create HTTP/3 RoundTripper with custom dial function that uses our established connection
	tlsConfig := ConvertUtlsConfig(rt.TLSConfig)
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}
	// Apply custom SNI if provided
	if rt.ServerName != "" {
		tlsConfig.ServerName = rt.ServerName
	}

	quicConfig := &quic.Config{
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
		Allow0RTT:                      false,
	}

	var h3Transport *http3.Transport

	// Handle connection based on type to avoid leaking pre-dialed connections
	if !conn.IsUQuic {
		// Standard QUIC: wire the pre-dialed connection into the transport
		// The Dial function returns the pre-dialed connection on first call,
		// then falls back to creating new connections for subsequent calls
		var connUsed int32
		preDialedConn, ok := conn.QuicConn.(*quic.Conn)
		if ok && preDialedConn != nil {
			h3Transport = &http3.Transport{
				TLSClientConfig: tlsConfig,
				QUICConfig:      quicConfig,
				Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (*quic.Conn, error) {
					// Use atomic to ensure we only use the pre-dialed connection once
					if atomic.AddInt32(&connUsed, 1) == 1 {
						return preDialedConn, nil
					}
					// For subsequent connections (e.g., reconnects), dial normally
					// Parse the address to get host and port
					udpAddr, err := net.ResolveUDPAddr("udp", addr)
					if err != nil {
						return nil, err
					}
					udpConn, err := net.ListenPacket("udp", "")
					if err != nil {
						return nil, err
					}
					return quic.DialEarly(ctx, udpConn, udpAddr, tlsCfg, cfg)
				},
			}
		} else {
			// Fallback: no valid pre-dialed connection, let transport manage its own
			// Close the unused connection to prevent leak
			rt.closeHTTP3Connection(conn)
			conn = nil
			h3Transport = &http3.Transport{
				TLSClientConfig: tlsConfig,
				QUICConfig:      quicConfig,
			}
		}
	} else {
		// UQuic connection: http3.Transport cannot use uquic.EarlyConnection
		// Close the pre-dialed connection to prevent leak - the fingerprinting
		// happened during the dial phase, but http3.Transport will create its own connection
		rt.closeHTTP3Connection(conn)
		conn = nil
		h3Transport = &http3.Transport{
			TLSClientConfig: tlsConfig,
			QUICConfig:      quicConfig,
		}
	}

	// Cache the HTTP/3 transport for future requests
	rt.cacheMu.Lock()
	rt.cachedHTTP3Transports[cacheKey] = &cachedHTTP3Transport{
		transport: h3Transport,
		conn:      conn, // nil for UQuic, actual conn for standard QUIC
		lastUsed:  time.Now(),
	}
	rt.cacheMu.Unlock()

	// Use the transport to make the request
	return rt.makeHTTP3RequestWithTransport(req, h3Transport)
}

// makeHTTP3RequestWithTransport performs an HTTP/3 request using a cached transport
func (rt *roundTripper) makeHTTP3RequestWithTransport(req *http.Request, h3Transport *http3.Transport) (*http.Response, error) {
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
		TLS:              nil,
		Cancel:           req.Cancel,
		Response:         nil,
	}

	// Use the RoundTripper to make the request
	stdResp, err := h3Transport.RoundTrip(stdReq)
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

// Default JA3 fingerprint for Chrome
const DefaultChrome_JA3 = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0"
