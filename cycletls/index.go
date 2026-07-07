package cycletls

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	nhttp "net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Danny-Dasilva/CycleTLS/cycletls/state"
	http "github.com/Danny-Dasilva/fhttp"
	"github.com/gorilla/websocket"
	utls "github.com/refraction-networking/utls"
)

// wsAuthToken is the authentication token for WebSocket connections.
// Generated at startup and passed to the child process via environment variable.
var wsAuthToken string

// wsRateLimiter limits the rate of incoming WebSocket messages.
var wsRateLimiter *RateLimiter

// wsConcurrentTracker limits concurrent in-flight requests.
var wsConcurrentTracker *ConcurrentRequestTracker

// safeChannelWriter wraps a channel to provide thread-safe writes with closed state tracking
type safeChannelWriter struct {
	ch     chan []byte
	mu     sync.RWMutex
	closed bool
	// done is closed by setClosed to unblock any in-flight blocking sends. The
	// underlying channel is never closed, so blocking sends can never panic on a
	// send-to-closed channel; writeSocket drains and exits on this signal instead.
	done chan struct{}
}

// newSafeChannelWriter creates a new safe channel writer
func newSafeChannelWriter(ch chan []byte) *safeChannelWriter {
	return &safeChannelWriter{
		ch:     ch,
		closed: false,
		done:   make(chan struct{}),
	}
}

// write safely writes data to the channel, returning false if channel is closed.
// Uses exclusive Lock and non-blocking select to prevent race between check and send.
// Issue #6 fix: The Lock (not RLock) ensures atomicity between closed check and send.
// Non-blocking select is intentional for the dispatcher hot path to avoid backpressure
// stalls, but callers should log when writes fail.
func (scw *safeChannelWriter) write(data []byte) bool {
	scw.mu.Lock()
	defer scw.mu.Unlock()

	if scw.closed {
		return false
	}

	select {
	case scw.ch <- data:
		return true
	default:
		// Issue #6: Log dropped data for observability instead of silent drop
		debugLogger.Printf("safeChannelWriter: data dropped, channel full (len=%d)", len(data))
		return false
	}
}

// writeBlocking safely writes data to the channel with a timeout, returning false
// if channel is closed or timeout expires. Use for critical messages that should not
// be silently dropped.
func (scw *safeChannelWriter) writeBlocking(data []byte, timeout time.Duration) bool {
	scw.mu.RLock()
	if scw.closed {
		scw.mu.RUnlock()
		return false
	}
	scw.mu.RUnlock()

	select {
	case scw.ch <- data:
		return true
	case <-time.After(timeout):
		debugLogger.Printf("safeChannelWriter: blocking write timed out after %v (len=%d)", timeout, len(data))
		return false
	}
}

// writeBlockingCtx writes data with true backpressure: it blocks until the frame
// is accepted by the writer, the writer is shut down (setClosed), or the request
// context is cancelled. It returns false only in the latter two cases. Use this for
// critical frames (response, data, end, error) that must never be silently dropped,
// which would truncate the body or desync the reader's frame stream.
func (scw *safeChannelWriter) writeBlockingCtx(data []byte, ctx context.Context) bool {
	scw.mu.RLock()
	closed := scw.closed
	scw.mu.RUnlock()
	if closed {
		return false
	}

	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case scw.ch <- data:
		return true
	case <-scw.done:
		return false
	case <-ctx.Done():
		return false
	}
}

// close marks the channel as closed (does not actually close it to avoid double-close panics)
func (scw *safeChannelWriter) setClosed() {
	scw.mu.Lock()
	defer scw.mu.Unlock()
	if scw.closed {
		return
	}
	scw.closed = true
	close(scw.done)
}

// Type definitions moved to types.go

// Backward-compatible aliases to state package
var debugLogger = state.DebugLogger

// getBuffer retrieves a buffer from the pool and resets it for reuse
func getBuffer() *bytes.Buffer {
	return state.GetBuffer()
}

// putBuffer returns a buffer to the pool for reuse
func putBuffer(buf *bytes.Buffer) {
	state.PutBuffer(buf)
}

// WebSocket connection management
type WebSocketConnection struct {
	Conn        *websocket.Conn
	RequestID   string
	URL         string
	ReadyState  int // 0=CONNECTING, 1=OPEN, 2=CLOSING, 3=CLOSED
	mu          sync.RWMutex
	writeMu     sync.Mutex // for thread-safe writes to WebSocket connection
	commandChan chan WebSocketCommand
	closeChan   chan struct{}
	done        chan struct{}  // signals all goroutines to exit
	wg          sync.WaitGroup // tracks goroutine completion for clean shutdown
	chanWrite   *safeChannelWriter
	protocol    string // Negotiated subprotocol
	extensions  string // Negotiated extensions
}

// safeWrite performs a thread-safe write to the WebSocket connection.
// Gorilla WebSocket is NOT thread-safe for concurrent writes, so all writes
// must be serialized through this method. Returns an error if the connection
// is nil or already closed (ReadyState >= 2).
func (ws *WebSocketConnection) safeWrite(conn *websocket.Conn, messageType int, data []byte) error {
	if conn == nil {
		return fmt.Errorf("websocket connection is nil")
	}
	ws.mu.RLock()
	readyState := ws.ReadyState
	ws.mu.RUnlock()
	if readyState >= 3 { // CLOSED
		return fmt.Errorf("websocket connection is closed (readyState=%d)", readyState)
	}
	ws.writeMu.Lock()
	defer ws.writeMu.Unlock()
	return conn.WriteMessage(messageType, data)
}

type WebSocketCommand struct {
	Type        string // "send", "close", "ping", "pong"
	Data        []byte
	IsBinary    bool
	CloseCode   int
	CloseReason string
}

// activeWebSockets is now managed by the state package
// Use state.RegisterWebSocket, state.GetWebSocket, state.UnregisterWebSocket

// browserFromOptions creates a Browser configuration from request options
func browserFromOptions(opts Options) Browser {
	return Browser{
		JA3:                     opts.Ja3,
		JA4r:                    opts.Ja4r,
		HTTP2Fingerprint:        opts.HTTP2Fingerprint,
		QUICFingerprint:         opts.QUICFingerprint,
		DisableGrease:           opts.DisableGrease,
		UserAgent:               opts.UserAgent,
		ServerName:              opts.ServerName,
		Cookies:                 opts.Cookies,
		InsecureSkipVerify:      opts.InsecureSkipVerify,
		ProxyInsecureSkipVerify: opts.ProxyInsecureSkipVerify,
		ForceHTTP1:              opts.ForceHTTP1,
		ForceHTTP3:              opts.ForceHTTP3,
		TLS13AutoRetry:          opts.TLS13AutoRetry,
		HeaderOrder:             opts.HeaderOrder,
	}
}

// buildHTTPRequest creates an HTTP request from the options with the given context.
// This is used by both v1 and v2 code paths.
func buildHTTPRequest(request cycleTLSRequest, ctx context.Context) (*http.Request, error) {
	// Handle both string body and byte body
	var bodyReader io.Reader
	if len(request.Options.BodyBytes) > 0 {
		bodyReader = bytes.NewReader(request.Options.BodyBytes)
	} else {
		bodyReader = strings.NewReader(request.Options.Body)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(request.Options.Method), request.Options.URL, bodyReader)
	if err != nil {
		return nil, err
	}

	// Build header order
	headerorder := []string{}
	if len(request.Options.HeaderOrder) > 0 {
		for _, v := range request.Options.HeaderOrder {
			headerorder = append(headerorder, strings.ToLower(v))
		}
	} else {
		headerorder = []string{
			"host", "connection", "cache-control", "device-memory", "viewport-width",
			"rtt", "downlink", "ect", "sec-ch-ua", "sec-ch-ua-mobile",
			"sec-ch-ua-full-version", "sec-ch-ua-arch", "sec-ch-ua-platform",
			"sec-ch-ua-platform-version", "sec-ch-ua-model", "upgrade-insecure-requests",
			"user-agent", "accept", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-user",
			"sec-fetch-dest", "referer", "accept-encoding", "accept-language", "cookie",
		}
	}

	// Build header order key
	headerorderkey := []string{}
	requestHeaders := request.Options.requestHeaders()
	for _, key := range headerorder {
		for k := range requestHeaders {
			if key == strings.ToLower(k) {
				headerorderkey = append(headerorderkey, strings.ToLower(k))
			}
		}
	}

	headerOrder := parseUserAgent(request.Options.UserAgent).HeaderOrder

	// Set headers with ordering
	req.Header = http.Header{
		http.HeaderOrderKey: headerorderkey,
	}

	// Only set PHeaderOrderKey for HTTP/2, not HTTP/3
	if !request.Options.ForceHTTP3 && request.Options.Protocol != "http3" {
		req.Header[http.PHeaderOrderKey] = headerOrder
	}

	// Parse URL for Host header
	u, err := url.Parse(request.Options.URL)
	if err != nil {
		return nil, err
	}

	// Append headers (preserving exact key casing and multi-value entries)
	setRequestHeaders(req.Header, request.Options, true)

	// Set Host header (respect user-provided for domain fronting)
	if !hasRequestHeader(request.Options, "Host") {
		req.Header["Host"] = []string{u.Host}
	}
	setHeaderExact(req.Header, "user-agent", request.Options.UserAgent)

	return req, nil
}

// ready Request
func processRequest(request cycleTLSRequest) (result fullRequest) {
	// Handle protocol-specific clients first (they build their own browser config)
	switch {
	case request.Options.Protocol == "websocket":
		return dispatchWebSocketRequest(request)
	case request.Options.Protocol == "sse":
		return dispatchSSERequest(request)
	case request.Options.Protocol == "http3" || request.Options.ForceHTTP3:
		return dispatchHTTP3Request(request)
	}

	ctx, cancel := context.WithCancel(context.Background())
	browser := browserFromOptions(request.Options)

	// Connection reuse is enabled by default
	client, err := newClientWithReuse(
		browser,
		request.Options.Timeout,
		request.Options.DisableRedirect,
		request.Options.UserAgent,
		request.Options.EnableConnectionReuse != false,
		request.Options.Proxy,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Handle both string body and byte body
	var bodyReader io.Reader
	if len(request.Options.BodyBytes) > 0 {
		bodyReader = bytes.NewReader(request.Options.BodyBytes)
	} else {
		bodyReader = strings.NewReader(request.Options.Body)
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(request.Options.Method), request.Options.URL, bodyReader)
	if err != nil {
		log.Fatal(err)
	}
	headerorder := []string{}
	//master header order, all your headers will be ordered based on this list and anything extra will be appended to the end
	//if your site has any custom headers, see the header order chrome uses and then add those headers to this list
	if len(request.Options.HeaderOrder) > 0 {
		//lowercase headers
		for _, v := range request.Options.HeaderOrder {
			lowercasekey := strings.ToLower(v)
			headerorder = append(headerorder, lowercasekey)
		}
	} else {
		headerorder = append(headerorder,
			"host",
			"connection",
			"cache-control",
			"device-memory",
			"viewport-width",
			"rtt",
			"downlink",
			"ect",
			"sec-ch-ua",
			"sec-ch-ua-mobile",
			"sec-ch-ua-full-version",
			"sec-ch-ua-arch",
			"sec-ch-ua-platform",
			"sec-ch-ua-platform-version",
			"sec-ch-ua-model",
			"upgrade-insecure-requests",
			"user-agent",
			"accept",
			"sec-fetch-site",
			"sec-fetch-mode",
			"sec-fetch-user",
			"sec-fetch-dest",
			"referer",
			"accept-encoding",
			"accept-language",
			"cookie",
		)
	}

	//TODO: Shorten this
	headerorderkey := []string{}
	requestHeaders := request.Options.requestHeaders()
	for _, key := range headerorder {
		for k := range requestHeaders {
			lowercasekey := strings.ToLower(k)
			if key == lowercasekey {
				headerorderkey = append(headerorderkey, lowercasekey)
			}
		}

	}
	headerOrder := parseUserAgent(request.Options.UserAgent).HeaderOrder

	//ordering the pseudo headers and our normal headers
	req.Header = http.Header{
		http.HeaderOrderKey: headerorderkey,
	}
	// Only set PHeaderOrderKey for HTTP/2, not HTTP/3
	// HTTP/3 requests are handled by dispatchHTTP3Request() which doesn't reach this code
	if !request.Options.ForceHTTP3 && request.Options.Protocol != "http3" {
		req.Header[http.PHeaderOrderKey] = headerOrder
	}
	//set our Host header
	u, err := url.Parse(request.Options.URL)
	if err != nil {
		result.options = request
		result.err = fmt.Errorf("invalid URL: %w", err)
		return result
	}

	//append our normal headers (preserving exact key casing and multi-value entries)
	setRequestHeaders(req.Header, request.Options, true)

	// Respect user-provided Host header for domain fronting; otherwise default to URL host
	if !hasRequestHeader(request.Options, "Host") {
		req.Header["Host"] = []string{u.Host}
	}
	setHeaderExact(req.Header, "user-agent", request.Options.UserAgent)

	state.RegisterRequest(request.RequestID, cancel)

	return fullRequest{
		req:     req,
		client:  client,
		options: request,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// reqContext returns the request's cancellation context, falling back to the
// underlying *http.Request context and finally context.Background so callers can
// always thread a non-nil context into blocking sends.
func (res fullRequest) reqContext() context.Context {
	if res.ctx != nil {
		return res.ctx
	}
	if res.req != nil {
		return res.req.Context()
	}
	return context.Background()
}

// dispatchHTTP3Request handles HTTP/3 specific request processing
func dispatchHTTP3Request(request cycleTLSRequest) (result fullRequest) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create browser configuration for HTTP/3 with forced settings
	browser := browserFromOptions(request.Options)
	browser.ForceHTTP1 = false
	browser.ForceHTTP3 = true

	// Connection reuse is enabled by default
	client, err := newClientWithReuse(
		browser,
		request.Options.Timeout,
		request.Options.DisableRedirect,
		request.Options.UserAgent,
		request.Options.EnableConnectionReuse != false,
		request.Options.Proxy,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Handle both string body and byte body
	var bodyReader io.Reader
	if len(request.Options.BodyBytes) > 0 {
		bodyReader = bytes.NewReader(request.Options.BodyBytes)
	} else {
		bodyReader = strings.NewReader(request.Options.Body)
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(request.Options.Method), request.Options.URL, bodyReader)
	if err != nil {
		log.Fatal(err)
	}

	// Set headers for HTTP/3 request (preserving exact key casing and multi-value entries)
	setRequestHeaders(req.Header, request.Options, true)

	// Parse URL for Host header
	u, err := url.Parse(request.Options.URL)
	if err != nil {
		result.options = request
		result.err = fmt.Errorf("invalid URL: %w", err)
		return result
	}
	// Respect user-provided Host header for domain fronting; otherwise default to URL host
	if !hasRequestHeader(request.Options, "Host") {
		req.Header["Host"] = []string{u.Host}
	}
	setHeaderExact(req.Header, "user-agent", request.Options.UserAgent)

	state.RegisterRequest(request.RequestID, cancel)

	return fullRequest{
		req:     req,
		client:  client,
		options: request,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// dispatchSSERequest handles SSE specific request processing
func dispatchSSERequest(request cycleTLSRequest) (result fullRequest) {
	ctx, cancel := context.WithCancel(context.Background())
	browser := browserFromOptions(request.Options)

	// Connection reuse is enabled by default
	client, err := newClientWithReuse(
		browser,
		request.Options.Timeout,
		request.Options.DisableRedirect,
		request.Options.UserAgent,
		request.Options.EnableConnectionReuse != false,
		request.Options.Proxy,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Prepare headers for SSE (preserving exact key casing and multi-value entries)
	headers := make(http.Header)
	setRequestHeaders(headers, request.Options, false)

	// Create SSE client
	sseClient := NewSSEClient(&client, headers)

	// Create a placeholder request for consistency
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, request.Options.URL, nil)
	if err != nil {
		log.Fatal(err)
	}

	state.RegisterRequest(request.RequestID, cancel)

	return fullRequest{
		req:       req,
		client:    client,
		options:   request,
		sseClient: sseClient,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// dispatchWebSocketRequest handles WebSocket specific request processing
func dispatchWebSocketRequest(request cycleTLSRequest) (result fullRequest) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create browser configuration for WebSocket (no HTTP/3 support)
	browser := browserFromOptions(request.Options)
	browser.ForceHTTP3 = false

	// Get TLS config for WebSocket
	tlsConfig := &utls.Config{
		InsecureSkipVerify: browser.InsecureSkipVerify,
		ServerName:         browser.ServerName,
	}

	// Prepare headers for WebSocket (preserving exact key casing and multi-value entries)
	headers := make(http.Header)
	setRequestHeaders(headers, request.Options, false)

	// Create WebSocket client
	convertedHeaders := ConvertFhttpHeader(headers)
	wsClient := NewWebSocketClient(tlsConfig, convertedHeaders)

	// Create a placeholder request for consistency
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, request.Options.URL, nil)
	if err != nil {
		log.Fatal(err)
	}

	state.RegisterRequest(request.RequestID, cancel)

	return fullRequest{
		req:      req,
		client:   http.Client{}, // Empty client as WebSocket uses its own dialer
		options:  request,
		wsClient: wsClient,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func dispatcherAsync(res fullRequest, chanWrite *safeChannelWriter) {
	// Panic recovery: prevent crashes and propagate error to the response channel
	// so callers are not left waiting indefinitely for a response.
	defer func() {
		if r := recover(); r != nil {
			debugLogger.Printf("Recovered from panic in dispatcherAsync for request %s: %v", res.options.RequestID, r)
			sendPanicError(chanWrite, res.options.RequestID, r)
		}
	}()

	// Check for early errors (URL parsing, etc.)
	if res.err != nil {
		// Cancel context on error path
		if res.cancel != nil {
			res.cancel()
		}
		state.UnregisterRequest(res.options.RequestID)
		frame := buildErrorFrame(res.options.RequestID, 400, res.err.Error())
		if !chanWrite.writeBlockingCtx(frame, res.reqContext()) {
			debugLogger.Printf("Failed to write error response for request %s: writer closed", res.options.RequestID)
		}
		return
	}

	// Issue #3 fix: SSE and WebSocket handlers manage their own context lifecycle.
	// Don't cancel the context here - these are long-lived connections where the
	// handler needs the context to remain active. Each handler cancels its own
	// context in its cleanup defer.

	// Handle SSE connections (dispatchSSEAsync handles its own context + UnregisterRequest)
	if res.sseClient != nil {
		dispatchSSEAsync(res, chanWrite)
		return
	}

	// Handle WebSocket connections (dispatchWebSocketAsync handles its own context + UnregisterRequest)
	if res.wsClient != nil {
		dispatchWebSocketAsync(res, chanWrite)
		return
	}

	// Issue #3: For HTTP paths, cancel context on exit
	if res.cancel != nil {
		defer res.cancel()
	}

	defer func() {
		state.UnregisterRequest(res.options.RequestID)
	}()

	// Issue #10 fix: Check for nil URL parse result to prevent nil pointer panic
	urlObj, err := url.Parse(res.options.Options.URL)
	hostPort := ""
	if err != nil || urlObj == nil {
		debugLogger.Printf("Failed to parse URL for connection reuse tracking: %s", res.options.Options.URL)
		hostPort = "unknown:443"
	} else {
		hostPort = urlObj.Host
		if !strings.Contains(hostPort, ":") {
			if urlObj.Scheme == "https" {
				hostPort = hostPort + ":443" // Default HTTPS port
			} else {
				hostPort = hostPort + ":80" // Default HTTP port
			}
		}
	}

	// Don't close connections when finished - they'll be reused for the same host
	// Instead, tell the roundtripper to keep this connection but close others
	defer func() {
		// Use type assertion to access the roundTripper
		if transport, ok := res.client.Transport.(*roundTripper); ok {
			transport.CloseIdleConnections(hostPort)
		}
	}()

	finalUrl := res.options.Options.URL

	timeout := timeoutSeconds(res.options.Options.Timeout)
	resp, err := doRequestWithHeaderTimeout(res.ctx, res.cancel, res.client, res.req, timeout)

	if err != nil {
		// Close response body on error path - Go http.Client can return
		// a non-nil Response alongside an error (e.g., redirect failures).
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		parsedError := parseError(err)
		frame := buildErrorFrame(res.options.RequestID, parsedError.StatusCode, parsedError.ErrorMsg+"-> \n"+err.Error())
		if !chanWrite.writeBlockingCtx(frame, res.reqContext()) {
			debugLogger.Printf("Failed to write error response for request %s: writer closed", res.options.RequestID)
		}
		return
	}

	defer resp.Body.Close()

	// Update finalUrl if redirect occurred
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		finalUrl = resp.Request.URL.String()
	}

	{
		// Checked encoder: on overflow (e.g. header value >= 64KB) send an error
		// frame instead of a truncated response, so the reader stream stays synced.
		frame, buildErr := buildResponseFrame(res.options.RequestID, resp.StatusCode, finalUrl, resp.Header)
		if buildErr != nil {
			debugLogger.Printf("Failed to build response frame for request %s: %v", res.options.RequestID, buildErr)
			errFrame := buildErrorFrame(res.options.RequestID, 500, "failed to encode response frame: "+buildErr.Error())
			if !chanWrite.writeBlockingCtx(errFrame, res.reqContext()) {
				debugLogger.Printf("Failed to write to channel: writer closed")
			}
			return
		}
		if !chanWrite.writeBlockingCtx(frame, res.reqContext()) {
			debugLogger.Printf("Failed to write to channel: writer closed")
			return
		}
	}

	{
		bufferSize := 8192
		chunkBuffer := make([]byte, bufferSize)

	loop:
		for {
			select {
			case <-res.req.Context().Done():
				debugLogger.Printf("Request %s was canceled during processing", res.options.RequestID)
				break loop

			default:
				n, err := resp.Body.Read(chunkBuffer)

				if res.req.Context().Err() != nil {
					debugLogger.Printf("Request %s was canceled during body read", res.options.RequestID)
					break loop
				}

				if err != nil && err != io.EOF {
					// Log to stdout instead of stderr to avoid process restart
					debugLogger.Printf("Read error: %s", err.Error())

					// Send error frame before breaking
					parsedError := parseError(err)
					frame := buildErrorFrame(res.options.RequestID, parsedError.StatusCode, parsedError.ErrorMsg)
					if !chanWrite.writeBlockingCtx(frame, res.reqContext()) {
						debugLogger.Printf("Failed to write to channel: writer closed")
						return
					}
					break loop
				}

				if err == io.EOF {
					// Handle any remaining data first
					if n > 0 {
						frame := buildDataFrame(res.options.RequestID, chunkBuffer[:n])
						if !chanWrite.writeBlockingCtx(frame, res.reqContext()) {
							debugLogger.Printf("Failed to write to channel: writer closed")
							return
						}
					}
					// EOF reached, exit the loop
					break loop
				}

				if n == 0 {
					// No data available right now, continue reading (don't break)
					continue
				}

				frame := buildDataFrame(res.options.RequestID, chunkBuffer[:n])
				if !chanWrite.writeBlockingCtx(frame, res.reqContext()) {
					debugLogger.Printf("Failed to write to channel: writer closed")
					return
				}
			}
		}
	}

	{
		frame := buildEndFrame(res.options.RequestID)
		if !chanWrite.writeBlockingCtx(frame, res.reqContext()) {
			debugLogger.Printf("Failed to write to channel: writer closed")
			return
		}
	}
}

// dispatchSSEAsync handles SSE connections asynchronously
func dispatchSSEAsync(res fullRequest, chanWrite *safeChannelWriter) {
	defer func() {
		if r := recover(); r != nil {
			debugLogger.Printf("Recovered from panic in dispatchSSEAsync for request %s: %v", res.options.RequestID, r)
			sendPanicError(chanWrite, res.options.RequestID, r)
		}
		// Issue #4 fix: Cancel the context when SSE handler exits to clean up resources
		if res.cancel != nil {
			res.cancel()
		}
		state.UnregisterRequest(res.options.RequestID)
	}()

	// Connect to SSE endpoint
	timeout := timeoutSeconds(res.options.Options.Timeout)
	sseResp, err := res.sseClient.ConnectWithTimeout(res.req.Context(), res.options.Options.URL, timeout)
	if err != nil {
		// Send error response
		b := getBuffer()
		requestIDLength := len(res.options.RequestID)

		b.WriteByte(byte(requestIDLength >> 8))
		b.WriteByte(byte(requestIDLength))
		b.WriteString(res.options.RequestID)
		b.WriteByte(0)
		b.WriteByte(5)
		b.WriteString("error")
		b.WriteByte(0) // Status code 0
		b.WriteByte(0)

		message := "SSE connection failed: " + err.Error()
		messageLength := len(message)

		b.WriteByte(byte(messageLength >> 8))
		b.WriteByte(byte(messageLength))
		b.WriteString(message)

		data := make([]byte, b.Len())
		copy(data, b.Bytes())
		putBuffer(b)
		if !chanWrite.writeBlockingCtx(data, res.reqContext()) {
			debugLogger.Printf("Failed to write to channel: writer closed or request cancelled")
			return
		}
		return
	}
	defer sseResp.Close()

	// Send initial response with headers
	{
		b := getBuffer()
		headerLength := len(sseResp.Response.Header)
		requestIDLength := len(res.options.RequestID)
		finalUrlLength := len(res.options.Options.URL)

		b.WriteByte(byte(requestIDLength >> 8))
		b.WriteByte(byte(requestIDLength))
		b.WriteString(res.options.RequestID)
		b.WriteByte(0)
		b.WriteByte(8)
		b.WriteString("response")
		b.WriteByte(byte(sseResp.Response.StatusCode >> 8))
		b.WriteByte(byte(sseResp.Response.StatusCode))

		// Write finalUrl length and value
		b.WriteByte(byte(finalUrlLength >> 8))
		b.WriteByte(byte(finalUrlLength))
		b.WriteString(res.options.Options.URL)

		// Write headers
		b.WriteByte(byte(headerLength >> 8))
		b.WriteByte(byte(headerLength))

		for name, values := range sseResp.Response.Header {
			nameLength := len(name)
			valuesLength := len(values)

			b.WriteByte(byte(nameLength >> 8))
			b.WriteByte(byte(nameLength))
			b.WriteString(name)
			b.WriteByte(byte(valuesLength >> 8))
			b.WriteByte(byte(valuesLength))

			for _, value := range values {
				valueLength := len(value)

				b.WriteByte(byte(valueLength >> 8))
				b.WriteByte(byte(valueLength))
				b.WriteString(value)
			}
		}

		data := make([]byte, b.Len())
		copy(data, b.Bytes())
		putBuffer(b)
		if !chanWrite.writeBlockingCtx(data, res.reqContext()) {
			debugLogger.Printf("Failed to write to channel: writer closed or request cancelled")
			return
		}
	}

	// Read SSE events
	// Issue #1 fix: Use labeled loop so break exits the for loop, not just the select
sseLoop:
	for {
		select {
		case <-res.req.Context().Done():
			debugLogger.Printf("SSE request %s was canceled", res.options.RequestID)
			break sseLoop

		default:
			event, err := sseResp.NextEvent()
			if err != nil {
				if err == io.EOF {
					// Normal end of stream
					break sseLoop
				}
				debugLogger.Printf("SSE read error: %s", err.Error())
				break sseLoop
			}

			if event == nil {
				continue
			}

			// Format SSE event as JSON for transmission
			eventData := map[string]interface{}{
				"event": event.Event,
				"data":  event.Data,
				"id":    event.ID,
				"retry": event.Retry,
			}

			eventBytes, err := json.Marshal(eventData)
			if err != nil {
				debugLogger.Printf("SSE event marshal error: %s", err.Error())
				continue
			}

			// Send event data
			b := getBuffer()
			requestIDLength := len(res.options.RequestID)
			bodyChunkLength := len(eventBytes)

			b.WriteByte(byte(requestIDLength >> 8))
			b.WriteByte(byte(requestIDLength))
			b.WriteString(res.options.RequestID)
			b.WriteByte(0)
			b.WriteByte(4)
			b.WriteString("data")
			b.WriteByte(byte(bodyChunkLength >> 24))
			b.WriteByte(byte(bodyChunkLength >> 16))
			b.WriteByte(byte(bodyChunkLength >> 8))
			b.WriteByte(byte(bodyChunkLength))
			b.Write(eventBytes)

			data := make([]byte, b.Len())
			copy(data, b.Bytes())
			putBuffer(b)
			if !chanWrite.writeBlockingCtx(data, res.reqContext()) {
				debugLogger.Printf("Failed to write to channel: writer closed or request cancelled")
				return
			}
		}
	}

	// Send end message
	{
		b := getBuffer()
		requestIDLength := len(res.options.RequestID)

		b.WriteByte(byte(requestIDLength >> 8))
		b.WriteByte(byte(requestIDLength))
		b.WriteString(res.options.RequestID)
		b.WriteByte(0)
		b.WriteByte(3)
		b.WriteString("end")

		data := make([]byte, b.Len())
		copy(data, b.Bytes())
		putBuffer(b)
		if !chanWrite.writeBlockingCtx(data, res.reqContext()) {
			debugLogger.Printf("Failed to write to channel: writer closed or request cancelled")
			return
		}
	}
}

// dispatchWebSocketAsync handles WebSocket connections asynchronously with full bidirectional support
func dispatchWebSocketAsync(res fullRequest, chanWrite *safeChannelWriter) {
	// Issue #2 fix: Track whether WebSocket was actually registered to avoid
	// unregistering on error paths before registration occurred (registry leak).
	wsRegistered := false
	defer func() {
		if r := recover(); r != nil {
			debugLogger.Printf("Recovered from panic in dispatchWebSocketAsync for request %s: %v", res.options.RequestID, r)
			sendPanicError(chanWrite, res.options.RequestID, r)
		}
		// Issue #4 fix: Cancel the context when WebSocket handler exits to clean up resources
		if res.cancel != nil {
			res.cancel()
		}
		state.UnregisterRequest(res.options.RequestID)

		// Only unregister WebSocket if it was actually registered
		if wsRegistered {
			state.UnregisterWebSocket(res.options.RequestID)
		}
	}()

	// Connect to WebSocket endpoint
	conn, resp, err := res.wsClient.Connect(res.options.Options.URL)
	if err != nil {
		sendWebSocketError(chanWrite, res.options.RequestID, res.options.Options.URL, resp, err)
		return
	}
	defer conn.Close()

	// Extract negotiated protocol and extensions
	negotiatedProtocol := resp.Header.Get("Sec-WebSocket-Protocol")
	negotiatedExtensions := resp.Header.Get("Sec-WebSocket-Extensions")

	// Create WebSocket connection object with done channel for goroutine cleanup
	wsConn := &WebSocketConnection{
		Conn:        conn,
		RequestID:   res.options.RequestID,
		URL:         res.options.Options.URL,
		ReadyState:  1, // OPEN
		commandChan: make(chan WebSocketCommand, 100),
		closeChan:   make(chan struct{}),
		done:        make(chan struct{}), // signals all goroutines to exit
		chanWrite:   chanWrite,
		protocol:    negotiatedProtocol,
		extensions:  negotiatedExtensions,
	}

	// Register the WebSocket connection
	state.RegisterWebSocket(res.options.RequestID, wsConn)
	wsRegistered = true

	// Send initial response with headers
	sendWebSocketResponse(chanWrite, res.options.RequestID, res.options.Options.URL, resp)

	// Send ws_open event
	sendWebSocketOpen(chanWrite, res.options.RequestID, negotiatedProtocol, negotiatedExtensions)

	// If there's body data, send it as the first WebSocket message
	if res.options.Options.Body != "" {
		err := wsConn.safeWrite(conn, websocket.TextMessage, []byte(res.options.Options.Body))
		if err != nil {
			debugLogger.Printf("WebSocket write error: %s", err.Error())
		}
	}

	// Read deadline timeout - prevents goroutine from blocking forever on reads
	const wsReadDeadline = 30 * time.Second

	// Start goroutines with WaitGroup tracking
	wsConn.wg.Add(2)

	// Goroutine to handle incoming WebSocket messages
	go func() {
		defer wsConn.wg.Done()
		for {
			// Check if we should exit before blocking on read
			select {
			case <-wsConn.done:
				return
			default:
			}

			// Set read deadline to prevent blocking forever
			conn.SetReadDeadline(time.Now().Add(wsReadDeadline))

			messageType, message, err := conn.ReadMessage()
			if err != nil {
				// Check if this is a timeout - if so, just continue the loop
				if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
					// Issue #5 fix: Check both done channel and context cancellation
					// on timeout to prevent goroutine leak when parent context is cancelled
					select {
					case <-wsConn.done:
						return
					case <-res.req.Context().Done():
						return
					default:
						continue // Just a timeout, keep reading
					}
				}

				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					// Normal close
					sendWebSocketClose(chanWrite, res.options.RequestID, websocket.CloseNormalClosure, "Connection closed normally")
				} else {
					debugLogger.Printf("WebSocket read error: %s", err.Error())
					sendWebSocketError(chanWrite, res.options.RequestID, res.options.Options.URL, nil, err)
				}
				return
			}

			// Send ws_message event
			sendWebSocketMessage(chanWrite, res.options.RequestID, messageType, message)
		}
	}()

	// Goroutine to handle outgoing commands
	go func() {
		defer wsConn.wg.Done()
		for {
			select {
			case <-wsConn.done:
				return

			case cmd := <-wsConn.commandChan:
				switch cmd.Type {
				case "send":
					msgType := websocket.TextMessage
					if cmd.IsBinary {
						msgType = websocket.BinaryMessage
					}
					err := wsConn.safeWrite(conn, msgType, cmd.Data)
					if err != nil {
						debugLogger.Printf("WebSocket send error: %s", err.Error())
						sendWebSocketError(chanWrite, res.options.RequestID, res.options.Options.URL, nil, err)
						// Issue #11 fix: Stop processing commands on write error -
						// the connection is likely broken. Continuing would just
						// accumulate errors on a dead connection.
						return
					}

				case "close":
					wsConn.mu.Lock()
					wsConn.ReadyState = 2 // CLOSING
					wsConn.mu.Unlock()

					closeCode := cmd.CloseCode
					if closeCode == 0 {
						closeCode = websocket.CloseNormalClosure
					}

					err := wsConn.safeWrite(conn, websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, cmd.CloseReason))
					if err != nil {
						debugLogger.Printf("WebSocket close error: %s", err.Error())
					}

					sendWebSocketClose(chanWrite, res.options.RequestID, closeCode, cmd.CloseReason)
					return

				case "ping":
					err := wsConn.safeWrite(conn, websocket.PingMessage, cmd.Data)
					if err != nil {
						debugLogger.Printf("WebSocket ping error: %s", err.Error())
						// Issue #11 fix: Stop on write error for ping too
						return
					}

				case "pong":
					err := wsConn.safeWrite(conn, websocket.PongMessage, cmd.Data)
					if err != nil {
						debugLogger.Printf("WebSocket pong error: %s", err.Error())
						// Issue #11 fix: Stop on write error for pong too
						return
					}
				}

			case <-wsConn.closeChan:
				return

			case <-res.req.Context().Done():
				debugLogger.Printf("WebSocket request %s was canceled", res.options.RequestID)
				return
			}
		}
	}()

	// Wait for context cancellation or close signal
	select {
	case <-wsConn.closeChan:
		// Connection close requested
	case <-res.req.Context().Done():
		debugLogger.Printf("WebSocket request %s was canceled", res.options.RequestID)
	}

	// Signal all goroutines to exit and wait for them
	close(wsConn.done)
	wsConn.wg.Wait()

	// Update connection state to CLOSED
	wsConn.mu.Lock()
	wsConn.ReadyState = 3
	wsConn.mu.Unlock()

	// Send end message
	sendWebSocketEnd(chanWrite, res.options.RequestID)
}

// sendPanicError sends an error response for a recovered panic so callers are
// not left waiting. Uses defer putBuffer for safety since we are already in a
// recovery context where another panic must not escape.
func sendPanicError(chanWrite *safeChannelWriter, requestID string, r interface{}) {
	b := getBuffer()
	defer putBuffer(b)

	errMsg := fmt.Sprintf("panic: %v", r)
	requestIDLength := len(requestID)
	statusCode := 500

	b.WriteByte(byte(requestIDLength >> 8))
	b.WriteByte(byte(requestIDLength))
	b.WriteString(requestID)
	b.WriteByte(0)
	b.WriteByte(5)
	b.WriteString("error")
	b.WriteByte(byte(statusCode >> 8))
	b.WriteByte(byte(statusCode))

	messageLength := len(errMsg)
	b.WriteByte(byte(messageLength >> 8))
	b.WriteByte(byte(messageLength))
	b.WriteString(errMsg)

	data := make([]byte, b.Len())
	copy(data, b.Bytes())
	chanWrite.write(data)
}

// Helper functions for sending WebSocket messages
func sendWebSocketError(chanWrite *safeChannelWriter, requestID, _ string, resp *nhttp.Response, err error) {
	b := getBuffer()
	defer putBuffer(b)
	requestIDLength := len(requestID)

	b.WriteByte(byte(requestIDLength >> 8))
	b.WriteByte(byte(requestIDLength))
	b.WriteString(requestID)
	b.WriteByte(0)
	b.WriteByte(8)
	b.WriteString("ws_error")

	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}

	b.WriteByte(byte(statusCode >> 8))
	b.WriteByte(byte(statusCode))

	message := err.Error()
	messageLength := len(message)

	b.WriteByte(byte(messageLength >> 8))
	b.WriteByte(byte(messageLength))
	b.WriteString(message)

	data := make([]byte, b.Len())
	copy(data, b.Bytes())
	chanWrite.write(data)
}

func sendWebSocketResponse(chanWrite *safeChannelWriter, requestID, url string, resp *nhttp.Response) {
	b := getBuffer()
	defer putBuffer(b)
	headerLength := len(resp.Header)
	requestIDLength := len(requestID)
	finalUrlLength := len(url)

	b.WriteByte(byte(requestIDLength >> 8))
	b.WriteByte(byte(requestIDLength))
	b.WriteString(requestID)
	b.WriteByte(0)
	b.WriteByte(8)
	b.WriteString("response")
	b.WriteByte(byte(resp.StatusCode >> 8))
	b.WriteByte(byte(resp.StatusCode))

	// Write finalUrl length and value
	b.WriteByte(byte(finalUrlLength >> 8))
	b.WriteByte(byte(finalUrlLength))
	b.WriteString(url)

	// Write headers
	b.WriteByte(byte(headerLength >> 8))
	b.WriteByte(byte(headerLength))

	for name, values := range resp.Header {
		nameLength := len(name)
		valuesLength := len(values)

		b.WriteByte(byte(nameLength >> 8))
		b.WriteByte(byte(nameLength))
		b.WriteString(name)
		b.WriteByte(byte(valuesLength >> 8))
		b.WriteByte(byte(valuesLength))

		for _, value := range values {
			valueLength := len(value)

			b.WriteByte(byte(valueLength >> 8))
			b.WriteByte(byte(valueLength))
			b.WriteString(value)
		}
	}

	data := make([]byte, b.Len())
	copy(data, b.Bytes())
	chanWrite.write(data)
}

func sendWebSocketOpen(chanWrite *safeChannelWriter, requestID, protocol, extensions string) {
	openMsg := map[string]interface{}{
		"type":       "open",
		"protocol":   protocol,
		"extensions": extensions,
	}

	// Issue #9 fix: Handle JSON marshal error
	msgBytes, err := json.Marshal(openMsg)
	if err != nil {
		debugLogger.Printf("Failed to marshal ws_open message for request %s: %v", requestID, err)
		return
	}

	b := getBuffer()
	defer putBuffer(b)
	requestIDLength := len(requestID)
	bodyChunkLength := len(msgBytes)

	b.WriteByte(byte(requestIDLength >> 8))
	b.WriteByte(byte(requestIDLength))
	b.WriteString(requestID)
	b.WriteByte(0)
	b.WriteByte(7)
	b.WriteString("ws_open")
	b.WriteByte(byte(bodyChunkLength >> 24))
	b.WriteByte(byte(bodyChunkLength >> 16))
	b.WriteByte(byte(bodyChunkLength >> 8))
	b.WriteByte(byte(bodyChunkLength))
	b.Write(msgBytes)

	data := make([]byte, b.Len())
	copy(data, b.Bytes())
	chanWrite.write(data)
}

func sendWebSocketMessage(chanWrite *safeChannelWriter, requestID string, messageType int, message []byte) {
	b := getBuffer()
	defer putBuffer(b)
	requestIDLength := len(requestID)

	b.WriteByte(byte(requestIDLength >> 8))
	b.WriteByte(byte(requestIDLength))
	b.WriteString(requestID)
	b.WriteByte(0)
	b.WriteByte(10)
	b.WriteString("ws_message")

	// Message type (1 byte)
	b.WriteByte(byte(messageType))

	// Message data length (4 bytes)
	messageLength := len(message)
	b.WriteByte(byte(messageLength >> 24))
	b.WriteByte(byte(messageLength >> 16))
	b.WriteByte(byte(messageLength >> 8))
	b.WriteByte(byte(messageLength))

	// Message data
	b.Write(message)

	data := make([]byte, b.Len())
	copy(data, b.Bytes())
	chanWrite.write(data)
}

func sendWebSocketClose(chanWrite *safeChannelWriter, requestID string, code int, reason string) {
	closeMsg := map[string]interface{}{
		"type":   "close",
		"code":   code,
		"reason": reason,
	}

	// Issue #9 fix: Handle JSON marshal error
	msgBytes, err := json.Marshal(closeMsg)
	if err != nil {
		debugLogger.Printf("Failed to marshal ws_close message for request %s: %v", requestID, err)
		return
	}

	b := getBuffer()
	defer putBuffer(b)
	requestIDLength := len(requestID)
	bodyChunkLength := len(msgBytes)

	b.WriteByte(byte(requestIDLength >> 8))
	b.WriteByte(byte(requestIDLength))
	b.WriteString(requestID)
	b.WriteByte(0)
	b.WriteByte(8)
	b.WriteString("ws_close")
	b.WriteByte(byte(bodyChunkLength >> 24))
	b.WriteByte(byte(bodyChunkLength >> 16))
	b.WriteByte(byte(bodyChunkLength >> 8))
	b.WriteByte(byte(bodyChunkLength))
	b.Write(msgBytes)

	data := make([]byte, b.Len())
	copy(data, b.Bytes())
	chanWrite.write(data)
}

func sendWebSocketEnd(chanWrite *safeChannelWriter, requestID string) {
	b := getBuffer()
	defer putBuffer(b)
	requestIDLength := len(requestID)

	b.WriteByte(byte(requestIDLength >> 8))
	b.WriteByte(byte(requestIDLength))
	b.WriteString(requestID)
	b.WriteByte(0)
	b.WriteByte(3)
	b.WriteString("end")

	data := make([]byte, b.Len())
	copy(data, b.Bytes())
	chanWrite.write(data)
}

func writeSocket(scw *safeChannelWriter, wsSocket *websocket.Conn) {
	for {
		select {
		case buf := <-scw.ch:
			if err := wsSocket.WriteMessage(websocket.BinaryMessage, buf); err != nil {
				debugLogger.Printf("Socket WriteMessage Failed: %s", err.Error())
				continue
			}
		case <-scw.done:
			// Writer shut down: drain any buffered frames so in-flight responses
			// are flushed, then exit. The underlying channel is never closed, so
			// this done signal is the sole termination path.
			for {
				select {
				case buf := <-scw.ch:
					if err := wsSocket.WriteMessage(websocket.BinaryMessage, buf); err != nil {
						debugLogger.Printf("Socket WriteMessage Failed: %s", err.Error())
					}
				default:
					return
				}
			}
		}
	}
}

func readSocket(chanRead chan fullRequest, wsSocket *websocket.Conn) {
	for {
		_, message, err := wsSocket.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return
			}
			debugLogger.Printf("Socket Error: %v", err)
			return
		}

		// Issue #7: Rate limiting on local WebSocket
		if wsRateLimiter != nil && !wsRateLimiter.Allow() {
			debugLogger.Printf("Rate limit exceeded, dropping message")
			continue
		}

		var baseMessage map[string]interface{}
		if err := json.Unmarshal(message, &baseMessage); err != nil {
			log.Print("Unmarshal Error", err)
			return
		}
		if action, ok := baseMessage["action"]; ok {
			if action == "exit" {
				// Respond by sending a close frame and then close the connection.
				wsSocket.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "exit"))
				wsSocket.Close()
				return
			}
			if action == "cancel" {
				requestId, _ := baseMessage["requestId"].(string)
				state.CancelRequest(requestId)
				continue
			}
			// Handle WebSocket commands
			if action == "ws_send" || action == "ws_close" || action == "ws_ping" || action == "ws_pong" {
				requestId, _ := baseMessage["requestId"].(string)

				wsConnInterface, exists := state.GetWebSocket(requestId)
				if !exists {
					debugLogger.Printf("WebSocket connection not found for request ID: %s", requestId)
					continue
				}
				wsConn, ok := wsConnInterface.(*WebSocketConnection)
				if !ok {
					debugLogger.Printf("Invalid WebSocket connection type for request ID: %s", requestId)
					continue
				}

				cmd := WebSocketCommand{}

				switch action {
				case "ws_send":
					cmd.Type = "send"
					if dataStr, ok := baseMessage["data"].(string); ok {
						if isBinary, ok := baseMessage["isBinary"].(bool); ok && isBinary {
							// TypeScript client base64-encodes binary data before sending via JSON
							// We must decode it here to get the original binary bytes
							decoded, err := base64.StdEncoding.DecodeString(dataStr)
							if err != nil {
								log.Printf("Failed to decode base64 data for ws_send: %v", err)
								cmd.Data = []byte(dataStr) // Fallback to raw string
							} else {
								cmd.Data = decoded
							}
							cmd.IsBinary = true
						} else {
							// Text message - use as-is
							cmd.Data = []byte(dataStr)
							cmd.IsBinary = false
						}
					}

				case "ws_close":
					cmd.Type = "close"
					if code, ok := baseMessage["code"].(float64); ok {
						cmd.CloseCode = int(code)
					}
					if reason, ok := baseMessage["reason"].(string); ok {
						cmd.CloseReason = reason
					}

				case "ws_ping":
					cmd.Type = "ping"
					if dataStr, ok := baseMessage["data"].(string); ok {
						cmd.Data = []byte(dataStr)
					}

				case "ws_pong":
					cmd.Type = "pong"
					if dataStr, ok := baseMessage["data"].(string); ok {
						cmd.Data = []byte(dataStr)
					}
				}

				// Issue #7 fix: Send command with timeout instead of silently dropping.
				// Use a short timeout to allow backpressure relief while still reporting
				// failures via the WebSocket connection's chanWrite.
				select {
				case wsConn.commandChan <- cmd:
					// Command sent successfully
				case <-time.After(5 * time.Second):
					debugLogger.Printf("WebSocket command channel full for request ID: %s, command type=%s dropped after timeout", requestId, cmd.Type)
					// Report error back through the WebSocket connection's writer
					if wsConn.chanWrite != nil {
						sendWebSocketError(wsConn.chanWrite, requestId, "", nil, fmt.Errorf("command channel full, %s command dropped", cmd.Type))
					}
				}

				continue
			}
		}
		// (If there was no "action" field, process as usual)
		request := new(cycleTLSRequest)
		if err := json.Unmarshal(message, &request); err != nil {
			log.Print("Unmarshal Error", err)
			return
		}
		chanRead <- processRequest(*request)
	}
}

// Worker
func readProcess(chanRead chan fullRequest, chanWrite *safeChannelWriter) {
	for request := range chanRead {
		// Issue #7: Check concurrent request limit
		if wsConcurrentTracker != nil && !wsConcurrentTracker.TryAcquire() {
			debugLogger.Printf("Max concurrent requests reached, rejecting request %s", request.options.RequestID)
			// Send error response for rejected request
			sendPanicError(chanWrite, request.options.RequestID, "max concurrent requests exceeded")
			continue
		}
		go func(req fullRequest) {
			if wsConcurrentTracker != nil {
				defer wsConcurrentTracker.Release()
			}
			dispatcherAsync(req, chanWrite)
		}(request)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  65536, // 64KB buffer for large init packets
	WriteBufferSize: 65536, // 64KB buffer for large responses
}

// WSEndpoint exports the main cycletls function as we websocket connection that clients can connect to
func WSEndpoint(w nhttp.ResponseWriter, r *nhttp.Request) {
	upgrader.CheckOrigin = func(r *nhttp.Request) bool { return true }

	// Issue #4: Validate WebSocket authentication token
	if wsAuthToken != "" {
		token := r.Header.Get("X-CycleTLS-Token")
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token != wsAuthToken {
			nhttp.Error(w, "Unauthorized: invalid or missing authentication token", nhttp.StatusUnauthorized)
			return
		}
	}

	// upgrade this connection to a WebSocket
	// connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		//Golang Received a non-standard request to this port, printing request
		var data map[string]interface{}
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Print("Invalid Request: Body Read Error" + err.Error())
		}
		err = json.Unmarshal(bodyBytes, &data)
		if err != nil {
			log.Print("Invalid Request: Json Conversion failed ")
		}
		body, err := PrettyStruct(data)
		if err != nil {
			log.Print("Invalid Request:", err)
		}
		headers, err := PrettyStruct(r.Header)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(headers)
		log.Println(body)

	} else {
		// Version routing: v=2 is now default, v=1 for legacy compatibility
		version := r.URL.Query().Get("v")
		if version == "1" {
			// V1 (legacy): Multiplexed JSON protocol - use ?v=1 for backward compat
			goto legacyHandler
		}

		// V2 (default): One WebSocket per request with flow control
		handleWSRequestV2(ws)
		return

	legacyHandler:
		// Legacy multiplexed JSON protocol
		chanRead := make(chan fullRequest)
		// Buffered so bursts of frames are absorbed; combined with the blocking
		// writeBlockingCtx sends in dispatcherAsync, response/data/end frames are
		// never dropped (previously an unbuffered channel + non-blocking send
		// silently discarded frames under concurrency).
		chanWrite := make(chan []byte, 256)
		safeWriter := newSafeChannelWriter(chanWrite)
		done := make(chan struct{})

		// Start readSocket in goroutine; when it returns, signal shutdown
		go func() {
			readSocket(chanRead, ws)
			close(done)
			close(chanRead)
		}()

		go readProcess(chanRead, safeWriter)

		// When readSocket exits, signal the writer to shut down. We do NOT close
		// chanWrite: blocking senders select on safeWriter.done, so the channel
		// stays open and can never panic on a send-to-closed channel.
		go func() {
			<-done
			safeWriter.setClosed()
		}()

		// Run as main thread - exits when the writer is shut down (safeWriter.done)
		writeSocket(safeWriter, ws)
	}
}

func setupRoutes() {
	nhttp.HandleFunc("/", WSEndpoint)
}

// generateAuthToken creates a cryptographically random hex token.
func generateAuthToken() string {
	b := make([]byte, 32) // 256-bit token
	if _, err := rand.Read(b); err != nil {
		// Fallback: use a less secure but functional token
		return fmt.Sprintf("cycletls-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func main() {
	port, exists := os.LookupEnv("WS_PORT")
	var addr *string
	if exists {
		addr = flag.String("addr", ":"+port, "http service address")
	} else {
		addr = flag.String("addr", ":9112", "http service address")
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	// Issue #4: Load auth token from environment (set by parent process)
	wsAuthToken = os.Getenv("WS_AUTH_TOKEN")
	if wsAuthToken == "" {
		// If no token provided by parent, generate one and print it
		// so the parent process can read it from stdout
		wsAuthToken = generateAuthToken()
		fmt.Printf("WS_AUTH_TOKEN=%s\n", wsAuthToken)
	}

	// Issue #7: Initialize rate limiter and concurrent request tracker
	wsRateLimiter = NewRateLimiter(DefaultWSRateLimitPerSec, DefaultWSBurst)
	wsConcurrentTracker = NewConcurrentRequestTracker(DefaultMaxConcurrentRequests)

	setupRoutes()
	log.Fatal(nhttp.ListenAndServe(*addr, nil))
}

// Response type moved to types.go

// Init creates a CycleTLS client with v1 default behavior (chan Response)
// Use WithRawBytes() option for performance enhancement with chan []byte
func Init(opts ...Option) CycleTLS {
	reqChan := make(chan fullRequest, 100)
	respChan := make(chan Response, 100)

	client := CycleTLS{
		ReqChan:  reqChan,
		RespChan: respChan,
	}

	// Apply options
	for _, opt := range opts {
		opt(&client)
	}

	return client
}

// Queue queues a request (simplified for integration tests)
func (client CycleTLS) Queue(URL string, options Options, Method string) {
	// This is a simplified implementation for integration tests
	// In a real implementation, this would queue the request
}

// Close closes the channels
func (client CycleTLS) Close() {
	if client.ReqChan != nil {
		close(client.ReqChan)
	}
	if client.RespChan != nil {
		close(client.RespChan)
	}
	if client.RespChanV2 != nil {
		close(client.RespChanV2)
	}
	// Clear all connections from the global pool
	clearAllConnections()
}

// Do creates a single HTTP request for integration tests
func (client CycleTLS) Do(URL string, options Options, Method string) (Response, error) {
	totalTimeout := timeoutSeconds(options.Timeout)
	// Use WithCancel (not WithTimeout) - doRequestWithHeaderTimeout handles
	// the timeout via its own timer. Using WithTimeout here would create a
	// second cancel path, causing a double-cancel race.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create browser from options
	browser := Browser{
		JA3:                     options.Ja3,
		JA4r:                    options.Ja4r,
		HTTP2Fingerprint:        options.HTTP2Fingerprint,
		QUICFingerprint:         options.QUICFingerprint,
		UserAgent:               options.UserAgent,
		Cookies:                 options.Cookies,
		InsecureSkipVerify:      options.InsecureSkipVerify,
		ProxyInsecureSkipVerify: options.ProxyInsecureSkipVerify,
		ForceHTTP1:              options.ForceHTTP1,
		ForceHTTP3:              options.ForceHTTP3,
		HeaderOrder:             options.HeaderOrder,
	}

	// Note: Don't automatically set HeaderOrder from UserAgent here as it can interfere with connection management
	// The pseudo-header order should be set through explicit HTTP2Fingerprint or Options.HeaderOrder

	// Create HTTP client with connection reuse enabled by default
	httpClient, err := newClientWithReuse(
		browser,
		options.Timeout,
		options.DisableRedirect,
		options.UserAgent,
		options.EnableConnectionReuse != false,
		options.Proxy,
	)
	if err != nil {
		return Response{}, err
	}

	// Create request using fhttp
	var bodyReader io.Reader
	if len(options.BodyBytes) > 0 {
		bodyReader = bytes.NewReader(options.BodyBytes)
	} else {
		bodyReader = strings.NewReader(options.Body)
	}
	req, err := http.NewRequestWithContext(ctx, Method, URL, bodyReader)
	if err != nil {
		return Response{}, err
	}

	// Set pseudo-header order based on UserAgent - only for HTTP/2, not HTTP/3
	headerOrder := parseUserAgent(options.UserAgent).HeaderOrder
	req.Header = http.Header{}

	// Only set PHeaderOrderKey for HTTP/2, not HTTP/3
	if !options.ForceHTTP3 {
		req.Header[http.PHeaderOrderKey] = headerOrder
	}

	// Set headers (preserving exact key casing and multi-value entries)
	setRequestHeaders(req.Header, options, false)

	// Make request
	headerTimeout := totalTimeout
	resp, err := doRequestWithHeaderTimeout(ctx, cancel, httpClient, req, headerTimeout)
	if err != nil {
		parsedError := parseError(err)
		return Response{
			Status: parsedError.StatusCode,
			Body:   parsedError.ErrorMsg + " -> " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	// Read body with timeout - cancel context if body read exceeds timeout.
	// doRequestWithHeaderTimeout only covers header arrival; we need a
	// separate timer to bound the total body read time.
	if totalTimeout > 0 {
		bodyTimer := time.AfterFunc(totalTimeout, cancel)
		defer bodyTimer.Stop()
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// When context was canceled by our body timer, treat as timeout
		if err == context.Canceled && totalTimeout > 0 {
			err = context.DeadlineExceeded
		}
		parsedError := parseError(err)
		if parsedError.StatusCode != 0 {
			return Response{
				Status: parsedError.StatusCode,
				Body:   parsedError.ErrorMsg + " -> " + err.Error(),
			}, nil
		}
		return Response{}, err
	}

	// Automatic decompression (axios-style) - check Content-Encoding header
	encoding := resp.Header["Content-Encoding"]
	content := resp.Header["Content-Type"]
	if len(encoding) > 0 {
		// Automatically decompress the body like axios does
		bodyBytes = DecompressBody(bodyBytes, encoding, content)
	}

	// Convert headers
	headers := make(map[string]string)
	for name, values := range resp.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	// Get final URL
	finalUrl := URL
	if resp.Request != nil && resp.Request.URL != nil {
		finalUrl = resp.Request.URL.String()
	}

	// Convert fhttp cookies to net/http cookies
	var netCookies []*nhttp.Cookie
	for _, cookie := range resp.Cookies() {
		netCookie := &nhttp.Cookie{
			Name:       cookie.Name,
			Value:      cookie.Value,
			Path:       cookie.Path,
			Domain:     cookie.Domain,
			Expires:    cookie.Expires,
			RawExpires: cookie.RawExpires,
			MaxAge:     cookie.MaxAge,
			Secure:     cookie.Secure,
			HttpOnly:   cookie.HttpOnly,
			SameSite:   nhttp.SameSite(cookie.SameSite),
			Raw:        cookie.Raw,
			Unparsed:   cookie.Unparsed,
		}
		netCookies = append(netCookies, netCookie)
	}

	return Response{
		Status:    resp.StatusCode,
		Body:      string(bodyBytes),
		BodyBytes: bodyBytes, // Provide raw bytes for binary data
		Headers:   headers,
		Cookies:   netCookies,
		FinalUrl:  finalUrl,
	}, nil
}
