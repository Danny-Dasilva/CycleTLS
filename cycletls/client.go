package cycletls

import (
	"context"
	"crypto/sha256"
	"fmt"
	fhttp "github.com/Danny-Dasilva/fhttp"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
	utls "github.com/refraction-networking/utls"
)

// Global client pool for connection reuse
var (
	clientPool      = make(map[string]fhttp.Client)
	clientPoolMutex = sync.RWMutex{}
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
	JA3                string
	JA4                string
	HTTP2Fingerprint   string
	QUICFingerprint    string

	// Browser identification
	UserAgent          string
	
	// Connection options
	Cookies            []Cookie
	InsecureSkipVerify bool
	ForceHTTP1         bool
	ForceHTTP3         bool
	
	// Ordered HTTP header fields
	HeaderOrder        []string

	// TLS configuration
	TLSConfig          *utls.Config

	// HTTP client
	client            *fhttp.Client
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
	//if timeout is not set in call default to 15
	if timeout == 0 {
		timeout = 15
	}
	client := fhttp.Client{
		Transport: newRoundTripper(browser, dialer),
		Timeout:   time.Duration(timeout) * time.Second,
	}
	//if disableRedirect is set to true httpclient will not redirect
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
		JA4:       ja4,
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
	// Create cookie signature for the key
	cookieStr := ""
	for _, cookie := range browser.Cookies {
		cookieStr += fmt.Sprintf("|cookie:%s=%s", cookie.Name, cookie.Value)
	}
	
	// Create a hash of the configuration that affects connection behavior
	configStr := fmt.Sprintf("ja3:%s|ja4:%s|http2:%s|quic:%s|ua:%s|proxy:%s|timeout:%d|redirect:%t|skipverify:%t|forcehttp1:%t|forcehttp3:%t%s",
		browser.JA3,
		browser.JA4,
		browser.HTTP2Fingerprint,
		browser.QUICFingerprint,
		browser.UserAgent,
		proxyURL,
		timeout,
		disableRedirect,
		browser.InsecureSkipVerify,
		browser.ForceHTTP1,
		browser.ForceHTTP3,
		cookieStr,
	)
	
	// Generate SHA256 hash for the key
	hash := sha256.Sum256([]byte(configStr))
	return fmt.Sprintf("%x", hash[:16]) // Use first 16 bytes for shorter key
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
	advancedClientPoolMutex.RLock()
	if entry, exists := advancedClientPool[clientKey]; exists {
		// Update last used time
		entry.LastUsed = time.Now()
		client := entry.Client
		advancedClientPoolMutex.RUnlock()
		return client, nil
	}
	advancedClientPoolMutex.RUnlock()
	
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
		dialer, err = newConnectDialer(proxyURL[0], userAgent)
		if err != nil {
			return fhttp.Client{
				Timeout:       time.Duration(timeout) * time.Second,
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
	}

	// Create http headers directly
	httpHeaders := make(fhttp.Header)
	httpHeaders.Set("User-Agent", browser.UserAgent)
	if browser.JA3 != "" {
		httpHeaders.Set("JA3", browser.JA3)
	}
	if browser.JA4 != "" {
		httpHeaders.Set("JA4", browser.JA4)
	}

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
	if browser.JA3 != "" {
		headers.Set("JA3", browser.JA3)
	}
	if browser.JA4 != "" {
		headers.Set("JA4", browser.JA4)
	}

	// Create SSE client
	sseClient := NewSSEClient(&httpClient, headers)
	
	// Connect to SSE endpoint
	return sseClient.Connect(ctx, urlStr)
}
