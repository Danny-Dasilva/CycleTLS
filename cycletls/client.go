package cycletls

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"time"

	fhttp "github.com/Danny-Dasilva/fhttp"
	"github.com/gorilla/websocket"
	uquic "github.com/refraction-networking/uquic"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/proxy"
)

// ClientPoolEntry represents a cached client with metadata
type ClientPoolEntry struct {
	Client    fhttp.Client
	CreatedAt time.Time
	LastUsed  time.Time
}

// Global client pool with metadata
var (
	advancedClientPool      = make(map[string]*ClientPoolEntry)
	advancedClientPoolMutex = sync.RWMutex{}
)

type Browser struct {
	// TLS fingerprinting options
	JA3              string
	JA4r             string // JA4 raw format with explicit cipher/extension values
	HTTP2Fingerprint string
	QUICFingerprint  string
	USpec            *uquic.QUICSpec // UQuic QUIC specification for HTTP3 fingerprinting
	DisableGrease    bool

	// Browser identification
	UserAgent string

	// Connection options
	ServerName              string
	Cookies                 []Cookie
	InsecureSkipVerify      bool
	ProxyInsecureSkipVerify *bool // TLS verification for proxy connections. nil=default (true for backward compat). Set to false to verify proxy certs.
	ForceHTTP1              bool
	ForceHTTP3              bool

	// TLS 1.3 specific options
	TLS13AutoRetry bool

	// Ordered HTTP header fields
	HeaderOrder []string

	// TLS configuration
	TLSConfig *utls.Config

	// HTTP client
	client *fhttp.Client
}

// Protocol represents the HTTP protocol version
type Protocol string

const (
	// ProtocolHTTP1 represents HTTP/1.1
	ProtocolHTTP1 Protocol = "http1"

	// ProtocolHTTP2 represents HTTP/2
	ProtocolHTTP2 Protocol = "http2"

	// ProtocolHTTP3 represents HTTP/3
	ProtocolHTTP3 Protocol = "http3"

	// ProtocolWebSocket represents WebSocket protocol
	ProtocolWebSocket Protocol = "websocket"

	// ProtocolSSE represents Server-Sent Events
	ProtocolSSE Protocol = "sse"
)

var disabledRedirect = func(req *fhttp.Request, via []*fhttp.Request) error {
	return fhttp.ErrUseLastResponse
}

func clientBuilder(browser Browser, dialer proxy.ContextDialer, timeout int, disableRedirect bool) fhttp.Client {
	client := fhttp.Client{
		Transport: newRoundTripper(browser, dialer),
	}
	if disableRedirect {
		client.CheckRedirect = disabledRedirect
	}
	return client
}

// NewTransport creates a new HTTP client transport that modifies HTTPS requests
// to imitiate a specific JA3 hash and User-Agent.
// # Example Usage
// import (
//
//	"github.com/Danny-Dasilva/CycleTLS/cycletls"
//	http "github.com/Danny-Dasilva/fhttp" // note this is a drop-in replacement for net/http
//
// )
//
// ja3 := "771,52393-52392-52244-52243-49195-49199-49196-49200-49171-49172-156-157-47-53-10,65281-0-23-35-13-5-18-16-30032-11-10,29-23-24,0"
// ua := "Chrome Version 57.0.2987.110 (64-bit) Linux"
//
//	cycleClient := &http.Client{
//		Transport:     cycletls.NewTransport(ja3, ua),
//	}
//
// cycleClient.Get("https://tls.peet.ws/")
func NewTransport(ja3 string, useragent string) fhttp.RoundTripper {
	return newRoundTripper(Browser{
		JA3:       ja3,
		UserAgent: useragent,
	})
}

// NewTransportWithJA4 creates a new HTTP client transport that modifies HTTPS requests
// using JA4 fingerprinting.
func NewTransportWithJA4(ja4 string, useragent string) fhttp.RoundTripper {
	return newRoundTripper(Browser{
		JA4r:      ja4,
		UserAgent: useragent,
	})
}

// NewTransportWithHTTP2Fingerprint creates a new HTTP client transport with HTTP/2 fingerprinting
func NewTransportWithHTTP2Fingerprint(http2fp string, useragent string) fhttp.RoundTripper {
	return newRoundTripper(Browser{
		HTTP2Fingerprint: http2fp,
		UserAgent:        useragent,
	})
}

// NewTransportWithProxy creates a new HTTP client transport that modifies HTTPS requests
// to imitiate a specific JA3 hash and User-Agent, optionally specifying a proxy via proxy.ContextDialer.
func NewTransportWithProxy(ja3 string, useragent string, proxy proxy.ContextDialer) fhttp.RoundTripper {
	return newRoundTripper(Browser{
		JA3:       ja3,
		UserAgent: useragent,
	}, proxy)
}

// generateClientKey creates a unique key for client pooling based on browser configuration
func generateClientKey(browser Browser, timeout int, disableRedirect bool, proxyURL string) string {
	// Cookies form a set, so sort their signatures to make the key independent
	// of cookie slice ordering (identical cookie sets -> identical key).
	cookieSigs := make([]string, 0, len(browser.Cookies))
	for _, c := range browser.Cookies {
		cookieSigs = append(cookieSigs, fmt.Sprintf("%s=%s;path=%s;domain=%s", c.Name, c.Value, c.Path, c.Domain))
	}
	sort.Strings(cookieSigs)

	// HeaderOrder is order-significant (it defines header emission order and thus
	// the fingerprint), so it is joined as-is and NOT sorted.
	headerOrder := strings.Join(browser.HeaderOrder, ",")

	// ProxyInsecureSkipVerify is a *bool; distinguish nil (default) from true/false.
	proxyInsecure := "nil"
	if browser.ProxyInsecureSkipVerify != nil {
		proxyInsecure = fmt.Sprintf("%t", *browser.ProxyInsecureSkipVerify)
	}

	// TLSConfig carries func pointers, so incorporate only its stable,
	// behavior-affecting sub-fields to keep the key deterministic within a process.
	tlsCfg := "nil"
	if browser.TLSConfig != nil {
		tlsCfg = fmt.Sprintf("set:sni=%s:skip=%t", browser.TLSConfig.ServerName, browser.TLSConfig.InsecureSkipVerify)
	}

	// Every field below influences newRoundTripper / Browser behavior; changing
	// any one of them must produce a different pooled client. timeout is included
	// because clients with different timeouts must not share a pool entry.
	configStr := fmt.Sprintf(
		"ja3:%s|ja4r:%s|http2:%s|quic:%s|uspec:%v|grease:%t|ua:%s|sni:%s|proxy:%s|timeout:%d|redirect:%t|skipverify:%t|proxyskip:%s|forcehttp1:%t|forcehttp3:%t|tls13retry:%t|headerorder:%s|tlscfg:%s|cookies:%s",
		browser.JA3,
		browser.JA4r,
		browser.HTTP2Fingerprint,
		browser.QUICFingerprint,
		browser.USpec,
		browser.DisableGrease,
		browser.UserAgent,
		browser.ServerName,
		proxyURL,
		timeout,
		disableRedirect,
		browser.InsecureSkipVerify,
		proxyInsecure,
		browser.ForceHTTP1,
		browser.ForceHTTP3,
		browser.TLS13AutoRetry,
		headerOrder,
		tlsCfg,
		strings.Join(cookieSigs, ","),
	)

	// FNV-1a 64-bit hash of the canonical config string. Fixed-size hex key.
	h := fnv.New64a()
	h.Write([]byte(configStr))
	return fmt.Sprintf("%016x", h.Sum64())
}

// getOrCreateClient retrieves a client from the pool or creates a new one
func getOrCreateClient(browser Browser, timeout int, disableRedirect bool, userAgent string, enableConnectionReuse bool, proxyURL ...string) (fhttp.Client, error) {
	// If connection reuse is disabled, always create a new client
	if !enableConnectionReuse {
		return createNewClient(browser, timeout, disableRedirect, userAgent, proxyURL...)
	}

	proxy := ""
	if len(proxyURL) > 0 {
		proxy = proxyURL[0]
	}

	clientKey := generateClientKey(browser, timeout, disableRedirect, proxy)

	// Try to get existing client from pool
	// Use a single Lock() for check-and-update to avoid TOCTOU race
	// (RLock->RUnlock->Lock allows another goroutine to delete the entry between locks)
	advancedClientPoolMutex.Lock()
	entry, exists := advancedClientPool[clientKey]
	if exists {
		entry.LastUsed = time.Now()
		client := entry.Client
		advancedClientPoolMutex.Unlock()
		return client, nil
	}
	advancedClientPoolMutex.Unlock()

	// Create new client if not found in pool
	advancedClientPoolMutex.Lock()
	defer advancedClientPoolMutex.Unlock()

	// Double-check in case another goroutine created it while we were waiting for the write lock
	if entry, exists := advancedClientPool[clientKey]; exists {
		entry.LastUsed = time.Now()
		return entry.Client, nil
	}

	// Create new client
	client, err := createNewClient(browser, timeout, disableRedirect, userAgent, proxyURL...)
	if err != nil {
		return fhttp.Client{}, err
	}

	// Bound the pool at insert time (evict least-recently-used) so it cannot grow
	// without limit. Runs under the held write lock.
	if len(advancedClientPool) >= maxClientPoolSize {
		evictOldestClientLocked()
	}
	// Add to pool
	now := time.Now()
	advancedClientPool[clientKey] = &ClientPoolEntry{
		Client:    client,
		CreatedAt: now,
		LastUsed:  now,
	}

	return client, nil
}

// createNewClient creates a new HTTP client (internal function)
func createNewClient(browser Browser, timeout int, disableRedirect bool, userAgent string, proxyURL ...string) (fhttp.Client, error) {
	var dialer proxy.ContextDialer
	if len(proxyURL) > 0 && len(proxyURL[0]) > 0 {
		var err error
		// Proxy TLS connections use a separate InsecureSkipVerify setting.
		// This defaults to true for backward compatibility since proxies
		// commonly use self-signed certificates. Users can override via
		// ProxyInsecureSkipVerify option (pointer allows distinguishing
		// "not set" from "set to false").
		proxyInsecureSkipVerify := getProxyInsecureSkipVerify(browser)
		dialer, err = newConnectDialer(proxyURL[0], userAgent, proxyInsecureSkipVerify)
		if err != nil {
			return fhttp.Client{
				CheckRedirect: disabledRedirect,
			}, err
		}
	} else {
		dialer = proxy.Direct
	}

	return clientBuilder(browser, dialer, timeout, disableRedirect), nil
}

// cleanupClientPool removes old unused clients from the pool
func cleanupClientPool(maxAge time.Duration) {
	advancedClientPoolMutex.Lock()
	defer advancedClientPoolMutex.Unlock()

	now := time.Now()
	for key, entry := range advancedClientPool {
		if now.Sub(entry.LastUsed) > maxAge {
			delete(advancedClientPool, key)
		}
	}
}

// maxClientPoolSize bounds advancedClientPool so it cannot grow without limit.
// Enforced at insert time via evictOldestClientLocked (LRU eviction), mirroring
// the transport/connection caches (maxCachedConnections/maxCachedTransports).
const maxClientPoolSize = 100

// evictOldestClientLocked removes the least-recently-used entry from
// advancedClientPool, closing its idle connections. The caller MUST hold
// advancedClientPoolMutex.
func evictOldestClientLocked() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for key, entry := range advancedClientPool {
		if first || entry.LastUsed.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastUsed
			first = false
		}
	}
	if first {
		return
	}
	if entry, ok := advancedClientPool[oldestKey]; ok {
		if transport, ok := entry.Client.Transport.(*roundTripper); ok {
			transport.CloseIdleConnections()
		}
	}
	delete(advancedClientPool, oldestKey)
}

// clearAllConnections clears all connections from the pool for test isolation
func clearAllConnections() {
	advancedClientPoolMutex.Lock()
	defer advancedClientPoolMutex.Unlock()

	// Close all connections in the pool before clearing
	for _, entry := range advancedClientPool {
		if transport, ok := entry.Client.Transport.(*roundTripper); ok {
			transport.CloseIdleConnections()
		}
	}

	// Clear the entire pool
	advancedClientPool = make(map[string]*ClientPoolEntry)
}

// newClient creates a new http client (backward compatibility - defaults to no connection reuse)
func newClient(browser Browser, timeout int, disableRedirect bool, UserAgent string, proxyURL ...string) (fhttp.Client, error) {
	// Backward compatibility: default to no connection reuse for existing code
	return getOrCreateClient(browser, timeout, disableRedirect, UserAgent, false, proxyURL...)
}

// newClientWithReuse creates a new http client with configurable connection reuse
func newClientWithReuse(browser Browser, timeout int, disableRedirect bool, UserAgent string, enableConnectionReuse bool, proxyURL ...string) (fhttp.Client, error) {
	return getOrCreateClient(browser, timeout, disableRedirect, UserAgent, enableConnectionReuse, proxyURL...)
}

// WebSocketConnect establishes a WebSocket connection
func (browser Browser) WebSocketConnect(ctx context.Context, urlStr string) (*websocket.Conn, *fhttp.Response, error) {
	// Create TLS config from browser settings
	tlsConfig := &utls.Config{
		InsecureSkipVerify: browser.InsecureSkipVerify,
		ServerName:         browser.ServerName,
	}

	// Create http headers directly
	httpHeaders := make(fhttp.Header)
	httpHeaders.Set("User-Agent", browser.UserAgent)

	// Convert headers and create WebSocket client
	convertedHeaders := ConvertFhttpHeader(httpHeaders)
	wsClient := NewWebSocketClient(tlsConfig, convertedHeaders)

	// Connect and return
	conn, resp, err := wsClient.Connect(urlStr)
	if err != nil {
		return nil, nil, err
	}

	// Convert response to fhttp.Response
	fhttpResp := &fhttp.Response{
		Status:        resp.Status,
		StatusCode:    resp.StatusCode,
		Proto:         resp.Proto,
		ProtoMajor:    resp.ProtoMajor,
		ProtoMinor:    resp.ProtoMinor,
		Body:          resp.Body,
		ContentLength: resp.ContentLength,
	}

	// Convert headers
	fhttpHeaders := make(fhttp.Header)
	for k, v := range resp.Header {
		fhttpHeaders[k] = v
	}
	fhttpResp.Header = fhttpHeaders

	// Convert request if present
	if resp.Request != nil {
		fhttpReq := &fhttp.Request{
			Method: resp.Request.Method,
			URL:    resp.Request.URL,
			Proto:  resp.Request.Proto,
			Header: fhttpHeaders,
			Body:   resp.Request.Body,
		}
		fhttpResp.Request = fhttpReq
	}

	return conn, fhttpResp, nil
}

// SSEConnect establishes an SSE connection
func (browser Browser) SSEConnect(ctx context.Context, urlStr string) (*SSEResponse, error) {
	// Create HTTP client with connection reuse enabled
	httpClient, err := newClientWithReuse(browser, 30, false, browser.UserAgent, true)
	if err != nil {
		return nil, err
	}

	// Create headers from browser settings
	headers := make(fhttp.Header)
	headers.Set("User-Agent", browser.UserAgent)

	// Create SSE client
	sseClient := NewSSEClient(&httpClient, headers)

	// Connect to SSE endpoint
	return sseClient.Connect(ctx, urlStr)
}
