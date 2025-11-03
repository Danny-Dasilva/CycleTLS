package cycletls

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	nhttp "net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	http "github.com/Danny-Dasilva/fhttp"
	"github.com/gorilla/websocket"
	utls "github.com/refraction-networking/utls"
)

// safeChannelWriter wraps a channel to provide thread-safe writes with closed state tracking
type safeChannelWriter struct {
	ch     chan []byte
	mu     sync.RWMutex
	closed bool
}

// newSafeChannelWriter creates a new safe channel writer
func newSafeChannelWriter(ch chan []byte) *safeChannelWriter {
	return &safeChannelWriter{
		ch:     ch,
		closed: false,
	}
}

// write safely writes data to the channel, returning false if channel is closed
func (scw *safeChannelWriter) write(data []byte) bool {
	scw.mu.RLock()
	defer scw.mu.RUnlock()

	if scw.closed {
		return false
	}

	// Use defer/recover to handle panics from closed channel
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic writing to channel: %v", r)
		}
	}()

	scw.ch <- data
	return true
}

// close marks the channel as closed (does not actually close it to avoid double-close panics)
func (scw *safeChannelWriter) setClosed() {
	scw.mu.Lock()
	defer scw.mu.Unlock()
	scw.closed = true
}

// Time wraps time.Time overriddin the json marshal/unmarshal to pass
// timestamp as integer
type Time struct {
	time.Time
}

// A Cookie represents an HTTP cookie as sent in the Set-Cookie header of an
// HTTP response or the Cookie header of an HTTP request.
//
// See https://tools.ietf.org/html/rfc6265 for details.
type Cookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`

	Path        string `json:"path"`   // optional
	Domain      string `json:"domain"` // optional
	Expires     time.Time
	JSONExpires Time   `json:"expires"`    // optional
	RawExpires  string `json:"rawExpires"` // for reading cookies only

	// MaxAge=0 means no 'Max-Age' attribute specified.
	// MaxAge<0 means delete cookie now, equivalently 'Max-Age: 0'
	// MaxAge>0 means Max-Age attribute present and given in seconds
	MaxAge   int            `json:"maxAge"`
	Secure   bool           `json:"secure"`
	HTTPOnly bool           `json:"httpOnly"`
	SameSite nhttp.SameSite `json:"sameSite"`
	Raw      string
	Unparsed []string `json:"unparsed"` // Raw text of unparsed attribute-value pairs
}

// Options sets CycleTLS client options
type Options struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
	BodyBytes []byte            `json:"bodyBytes"` // New field for binary request data

	// TLS fingerprinting options
	Ja3              string `json:"ja3"`
	Ja4r             string `json:"ja4r"` // JA4 raw format with explicit cipher/extension values
	HTTP2Fingerprint string `json:"http2Fingerprint"`
	QUICFingerprint  string `json:"quicFingerprint"`
	DisableGrease    bool   `json:"disableGrease"` // Disable GREASE for exact JA4 matching

	// Browser identification
	UserAgent string `json:"userAgent"`

	// Connection options
	Proxy              string   `json:"proxy"`
	ServerName         string   `json:"serverName"` // Custom TLS SNI override
	Cookies            []Cookie `json:"cookies"`
	Timeout            int      `json:"timeout"`
	DisableRedirect    bool     `json:"disableRedirect"`
	HeaderOrder        []string `json:"headerOrder"`
	OrderAsProvided    bool     `json:"orderAsProvided"` //TODO
	InsecureSkipVerify bool     `json:"insecureSkipVerify"`

	// Protocol options
	ForceHTTP1 bool   `json:"forceHTTP1"`
	ForceHTTP3 bool   `json:"forceHTTP3"`
	Protocol   string `json:"protocol"` // "http1", "http2", "http3", "websocket", "sse"

	// TLS 1.3 specific options
	TLS13AutoRetry bool `json:"tls13AutoRetry"` // Automatically retry with TLS 1.3 compatible curves (default: true)

	// Connection reuse options
	EnableConnectionReuse bool `json:"enableConnectionReuse"` // Enable connection reuse across requests (default: true)
}

type cycleTLSRequest struct {
	RequestID string  `json:"requestId"`
	Options   Options `json:"options"`
}

// rename to request+client+options
type fullRequest struct {
	req       *http.Request
	client    http.Client
	options   cycleTLSRequest
	sseClient *SSEClient       // For SSE connections
	wsClient  *WebSocketClient // For WebSocket connections
}

// CycleTLS creates full request and response
type CycleTLS struct {
	ReqChan    chan fullRequest
	RespChan   chan Response // V1 default: chan Response for backward compatibility
	RespChanV2 chan []byte   `json:"-"` // V2 performance: chan []byte for opt-in users
}

// Option configures a CycleTLS client
type Option func(*CycleTLS)

// WithRawBytes enables the performance enhancement channel (RespChanV2 chan []byte)
// Use this option for performance-critical applications that can handle raw byte responses
func WithRawBytes() Option {
	return func(client *CycleTLS) {
		if client.RespChanV2 == nil {
			client.RespChanV2 = make(chan []byte, 100)
		}
	}
}

var activeRequests = make(map[string]context.CancelFunc)
var activeRequestsMutex sync.Mutex
var debugLogger = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)

// WebSocket connection management
type WebSocketConnection struct {
	Conn         *websocket.Conn
	RequestID    string
	URL          string
	ReadyState   int // 0=CONNECTING, 1=OPEN, 2=CLOSING, 3=CLOSED
	mu           sync.RWMutex
	commandChan  chan WebSocketCommand
	closeChan    chan struct{}
	chanWrite    *safeChannelWriter
	protocol     string // Negotiated subprotocol
	extensions   string // Negotiated extensions
}

type WebSocketCommand struct {
	Type       string // "send", "close", "ping", "pong"
	Data       []byte
	IsBinary   bool
	CloseCode  int
	CloseReason string
}

var activeWebSockets = make(map[string]*WebSocketConnection)
var activeWebSocketsMutex sync.RWMutex

// ready Request
func processRequest(request cycleTLSRequest) (result fullRequest) {
	ctx, cancel := context.WithCancel(context.Background())

	var browser = Browser{
		// TLS fingerprinting options
		JA3:              request.Options.Ja3,
		JA4r:             request.Options.Ja4r,
		HTTP2Fingerprint: request.Options.HTTP2Fingerprint,
		QUICFingerprint:  request.Options.QUICFingerprint,
		DisableGrease:    request.Options.DisableGrease,

		// Browser identification
		UserAgent: request.Options.UserAgent,

		// Connection options
		ServerName:         request.Options.ServerName,
		Cookies:            request.Options.Cookies,
		InsecureSkipVerify: request.Options.InsecureSkipVerify,
		ForceHTTP1:         request.Options.ForceHTTP1,
		ForceHTTP3:         request.Options.ForceHTTP3,

		// TLS 1.3 specific options
		TLS13AutoRetry: request.Options.TLS13AutoRetry,

		// Header ordering
		HeaderOrder: request.Options.HeaderOrder,
	}

	// Handle protocol-specific clients
	if request.Options.Protocol == "websocket" {
		// WebSocket requests are handled separately
		return dispatchWebSocketRequest(request)
	} else if request.Options.Protocol == "sse" {
		// SSE requests are handled separately
		return dispatchSSERequest(request)
	} else if request.Options.Protocol == "http3" || request.Options.ForceHTTP3 {
		// HTTP/3 requests are handled separately and will be implemented later
		// HTTP/3 requests are now supported
		return dispatchHTTP3Request(request)
	}

	// Default to true for connection reuse
	enableConnectionReuse := true
	if request.Options.EnableConnectionReuse == false {
		// Only disable if explicitly set to false
		enableConnectionReuse = false
	}

	client, err := newClientWithReuse(
		browser,
		request.Options.Timeout,
		request.Options.DisableRedirect,
		request.Options.UserAgent,
		enableConnectionReuse,
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

	headermap := make(map[string]string)
	//TODO: Shorten this
	headerorderkey := []string{}
	for _, key := range headerorder {
		for k, v := range request.Options.Headers {
			lowercasekey := strings.ToLower(k)
			if key == lowercasekey {
				headermap[k] = v
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
		panic(err)
	}

	//append our normal headers
	for k, v := range request.Options.Headers {
		if k != "Content-Length" {
			req.Header.Set(k, v)
		}
	}

	// Respect user-provided Host header for domain fronting; otherwise default to URL host
	if _, ok := request.Options.Headers["Host"]; !ok {
		if _, ok := request.Options.Headers["host"]; !ok {
			req.Header.Set("Host", u.Host)
		}
	}
	req.Header.Set("user-agent", request.Options.UserAgent)

	activeRequestsMutex.Lock()
	activeRequests[request.RequestID] = cancel
	activeRequestsMutex.Unlock()

	return fullRequest{req: req, client: client, options: request}
}

// dispatchHTTP3Request handles HTTP/3 specific request processing
func dispatchHTTP3Request(request cycleTLSRequest) (result fullRequest) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create browser configuration for HTTP/3
	var browser = Browser{
		// TLS fingerprinting options
		JA3:              request.Options.Ja3,
		JA4r:             request.Options.Ja4r,
		HTTP2Fingerprint: request.Options.HTTP2Fingerprint,
		QUICFingerprint:  request.Options.QUICFingerprint,
		DisableGrease:    request.Options.DisableGrease,

		// Browser identification
		UserAgent: request.Options.UserAgent,

		// Connection options
		ServerName:         request.Options.ServerName,
		Cookies:            request.Options.Cookies,
		InsecureSkipVerify: request.Options.InsecureSkipVerify,
		ForceHTTP1:         false, // Force HTTP/3
		ForceHTTP3:         true,  // Force HTTP/3

		// TLS 1.3 specific options (HTTP/3 requires TLS 1.3)
		TLS13AutoRetry: request.Options.TLS13AutoRetry,

		// Header ordering
		HeaderOrder: request.Options.HeaderOrder,
	}

	// Default to true for connection reuse
	enableConnectionReuse := true
	if request.Options.EnableConnectionReuse == false {
		// Only disable if explicitly set to false
		enableConnectionReuse = false
	}

	client, err := newClientWithReuse(
		browser,
		request.Options.Timeout,
		request.Options.DisableRedirect,
		request.Options.UserAgent,
		enableConnectionReuse,
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

	// Set headers for HTTP/3 request
	for k, v := range request.Options.Headers {
		if k != "Content-Length" {
			req.Header.Set(k, v)
		}
	}

	// Parse URL for Host header
	u, err := url.Parse(request.Options.URL)
	if err != nil {
		panic(err)
	}
	// Respect user-provided Host header for domain fronting; otherwise default to URL host
	if _, ok := request.Options.Headers["Host"]; !ok {
		if _, ok := request.Options.Headers["host"]; !ok {
			req.Header.Set("Host", u.Host)
		}
	}
	req.Header.Set("user-agent", request.Options.UserAgent)

	activeRequestsMutex.Lock()
	activeRequests[request.RequestID] = cancel
	activeRequestsMutex.Unlock()

	return fullRequest{req: req, client: client, options: request}
}

// dispatchSSERequest handles SSE specific request processing
func dispatchSSERequest(request cycleTLSRequest) (result fullRequest) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create browser configuration for SSE
	var browser = Browser{
		// TLS fingerprinting options
		JA3:              request.Options.Ja3,
		JA4r:             request.Options.Ja4r,
		HTTP2Fingerprint: request.Options.HTTP2Fingerprint,
		QUICFingerprint:  request.Options.QUICFingerprint,
		DisableGrease:    request.Options.DisableGrease,

		// Browser identification
		UserAgent: request.Options.UserAgent,

		// Connection options
		ServerName:         request.Options.ServerName,
		Cookies:            request.Options.Cookies,
		InsecureSkipVerify: request.Options.InsecureSkipVerify,
		ForceHTTP1:         request.Options.ForceHTTP1,
		ForceHTTP3:         request.Options.ForceHTTP3,

		// TLS 1.3 specific options
		TLS13AutoRetry: request.Options.TLS13AutoRetry,

		// Header ordering
		HeaderOrder: request.Options.HeaderOrder,
	}

	// Default to true for connection reuse
	enableConnectionReuse := true
	if request.Options.EnableConnectionReuse == false {
		// Only disable if explicitly set to false
		enableConnectionReuse = false
	}

	client, err := newClientWithReuse(
		browser,
		request.Options.Timeout,
		request.Options.DisableRedirect,
		request.Options.UserAgent,
		enableConnectionReuse,
		request.Options.Proxy,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Prepare headers for SSE
	headers := make(http.Header)
	for k, v := range request.Options.Headers {
		headers.Set(k, v)
	}

	// Create SSE client
	sseClient := NewSSEClient(&client, headers)

	// Create a placeholder request for consistency
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, request.Options.URL, nil)
	if err != nil {
		log.Fatal(err)
	}

	activeRequestsMutex.Lock()
	activeRequests[request.RequestID] = cancel
	activeRequestsMutex.Unlock()

	return fullRequest{
		req:       req,
		client:    client,
		options:   request,
		sseClient: sseClient,
	}
}

// dispatchWebSocketRequest handles WebSocket specific request processing
func dispatchWebSocketRequest(request cycleTLSRequest) (result fullRequest) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create browser configuration for WebSocket
	var browser = Browser{
		// TLS fingerprinting options
		JA3:              request.Options.Ja3,
		JA4r:             request.Options.Ja4r,
		HTTP2Fingerprint: request.Options.HTTP2Fingerprint,
		QUICFingerprint:  request.Options.QUICFingerprint,
		DisableGrease:    request.Options.DisableGrease,

		// Browser identification
		UserAgent: request.Options.UserAgent,

		// Connection options
		Cookies:            request.Options.Cookies,
		InsecureSkipVerify: request.Options.InsecureSkipVerify,
		ForceHTTP1:         request.Options.ForceHTTP1,
		ForceHTTP3:         false, // WebSocket doesn't support HTTP/3

		// TLS 1.3 specific options
		TLS13AutoRetry: request.Options.TLS13AutoRetry,

		// Header ordering
		HeaderOrder: request.Options.HeaderOrder,
	}

	// Get TLS config for WebSocket
	tlsConfig := &utls.Config{
		InsecureSkipVerify: browser.InsecureSkipVerify,
		ServerName:         request.Options.ServerName,
	}

	// Prepare headers for WebSocket
	headers := make(http.Header)
	for k, v := range request.Options.Headers {
		headers.Set(k, v)
	}

	// Create WebSocket client
	convertedHeaders := ConvertFhttpHeader(headers)
	wsClient := NewWebSocketClient(tlsConfig, convertedHeaders)

	// Create a placeholder request for consistency
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, request.Options.URL, nil)
	if err != nil {
		log.Fatal(err)
	}

	activeRequestsMutex.Lock()
	activeRequests[request.RequestID] = cancel
	activeRequestsMutex.Unlock()

	return fullRequest{
		req:      req,
		client:   http.Client{}, // Empty client as WebSocket uses its own dialer
		options:  request,
		wsClient: wsClient,
	}
}

// // Queue queues request in worker pool
// func (client CycleTLS) Queue(URL string, options Options, Method string) {

// 	options.URL = URL
// 	options.Method = Method
// 	//TODO add timestamp to request
// 	opt := cycleTLSRequest{"Queued Request", options}
// 	response := processRequest(opt)
// 	client.ReqChan <- response
// }

// // Do creates a single request
// func (client CycleTLS) Do(URL string, options Options, Method string) (response Response, err error) {

// 	options.URL = URL
// 	options.Method = Method
// 	// Set default values if not provided
// 	if options.Ja3 == "" {
// 		options.Ja3 = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,18-35-65281-45-17513-27-65037-16-10-11-5-13-0-43-23-51,29-23-24,0"
// 	}
// 	if options.UserAgent == "" {
// 		// Mac OS Chrome 121
// 		options.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"
// 	}
// 	opt := cycleTLSRequest{"cycleTLSRequest", options}

// 	res := processRequest(opt)
// 	response, err = dispatcher(res)
// 	if err != nil {
// 		log.Print("Request Failed: " + err.Error())
// 		return response, err
// 	}

// 	return response, nil
// }

// Init starts the worker pool or returns a empty cycletls struct
// func Init(workers ...bool) CycleTLS {
// 	if len(workers) > 0 && workers[0] {
// 		reqChan := make(chan fullRequest)
// 		respChan := make(chan Response)
// 		go workerPool(reqChan, respChan)
// 		log.Println("Worker Pool Started")

// 		return CycleTLS{ReqChan: reqChan, RespChan: respChan}
// 	}
// 	return CycleTLS{}

// }

// // Close closes channels
// func (client CycleTLS) Close() {
// 	close(client.ReqChan)
// 	close(client.RespChan)

// }

// // Worker Pool
// func workerPool(reqChan chan fullRequest, respChan chan Response) {
// 	//MAX
// 	for i := 0; i < 100; i++ {
// 		go worker(reqChan, respChan)
// 	}
// }

// // Worker
// func worker(reqChan chan fullRequest, respChan chan Response) {
// 	for res := range reqChan {
// 		response, err := dispatcher(res)
// 		if err != nil {
// 			log.Print("Request Failed: " + err.Error())
// 		}
// 		respChan <- response
// 	}
// }

func dispatcherAsync(res fullRequest, chanWrite *safeChannelWriter) {
	// Add panic recovery to prevent crashes from channel issues
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in dispatcherAsync for request %s: %v", res.options.RequestID, r)
		}
	}()

	// Handle SSE connections
	if res.sseClient != nil {
		dispatchSSEAsync(res, chanWrite)
		return
	}

	// Handle WebSocket connections
	if res.wsClient != nil {
		dispatchWebSocketAsync(res, chanWrite)
		return
	}

	defer func() {
		activeRequestsMutex.Lock()
		delete(activeRequests, res.options.RequestID)
		activeRequestsMutex.Unlock()
	}()

	// Extract host from URL for connection reuse tracking
	urlObj, _ := url.Parse(res.options.Options.URL)
	hostPort := urlObj.Host
	if !strings.Contains(hostPort, ":") {
		if urlObj.Scheme == "https" {
			hostPort = hostPort + ":443" // Default HTTPS port
		} else {
			hostPort = hostPort + ":80" // Default HTTP port
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

	resp, err := res.client.Do(res.req)

	if err != nil {
		parsedError := parseError(err)

		{
			var b bytes.Buffer
			var requestIDLength = len(res.options.RequestID)

			b.WriteByte(byte(requestIDLength >> 8))
			b.WriteByte(byte(requestIDLength))
			b.WriteString(res.options.RequestID)
			b.WriteByte(0)
			b.WriteByte(5)
			b.WriteString("error")
			b.WriteByte(byte(parsedError.StatusCode >> 8))
			b.WriteByte(byte(parsedError.StatusCode))

			var message = parsedError.ErrorMsg + "-> \n" + string(err.Error())
			var messageLength = len(message)

			b.WriteByte(byte(messageLength >> 8))
			b.WriteByte(byte(messageLength))
			b.WriteString(message)

			if !chanWrite.write(b.Bytes()) {
				log.Printf("Failed to write error response for request %s: channel closed", res.options.RequestID)
			}
		}

		return
	}

	defer resp.Body.Close()

	// Update finalUrl if redirect occurred
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		finalUrl = resp.Request.URL.String()
	}

	{
		var b bytes.Buffer
		var headerLength = len(resp.Header)
		var requestIDLength = len(res.options.RequestID)
		var finalUrlLength = len(finalUrl)

		b.WriteByte(byte(requestIDLength >> 8))
		b.WriteByte(byte(requestIDLength))
		b.WriteString(res.options.RequestID)
		b.WriteByte(0)
		b.WriteByte(8)
		b.WriteString("response")
		b.WriteByte(byte(resp.StatusCode >> 8))
		b.WriteByte(byte(resp.StatusCode))

		// Write finalUrl length and value
		b.WriteByte(byte(finalUrlLength >> 8))
		b.WriteByte(byte(finalUrlLength))
		b.WriteString(finalUrl)

		// Write headers
		b.WriteByte(byte(headerLength >> 8))
		b.WriteByte(byte(headerLength))

		for name, values := range resp.Header {
			var nameLength = len(name)
			var valuesLength = len(values)

			b.WriteByte(byte(nameLength >> 8))
			b.WriteByte(byte(nameLength))
			b.WriteString(name)
			b.WriteByte(byte(valuesLength >> 8))
			b.WriteByte(byte(valuesLength))

			for _, value := range values {
				var valueLength = len(value)

				b.WriteByte(byte(valueLength >> 8))
				b.WriteByte(byte(valueLength))
				b.WriteString(value)
			}
		}

		if !chanWrite.write(b.Bytes()) {
			log.Printf("Failed to write to channel: channel closed")
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
					var b bytes.Buffer
					requestIDLength := len(res.options.RequestID)

					b.WriteByte(byte(requestIDLength >> 8))
					b.WriteByte(byte(requestIDLength))
					b.WriteString(res.options.RequestID)
					b.WriteByte(0)
					b.WriteByte(5)
					b.WriteString("error")
					b.WriteByte(byte(parsedError.StatusCode >> 8))
					b.WriteByte(byte(parsedError.StatusCode))

					message := parsedError.ErrorMsg
					messageLength := len(message)

					b.WriteByte(byte(messageLength >> 8))
					b.WriteByte(byte(messageLength))
					b.WriteString(message)

					if !chanWrite.write(b.Bytes()) {
			log.Printf("Failed to write to channel: channel closed")
			return
		}
					break loop
				}

				if err == io.EOF {
					// Handle any remaining data first
					if n > 0 {
						var b bytes.Buffer
						requestIDLength := len(res.options.RequestID)
						bodyChunkLength := n

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
						b.Write(chunkBuffer[:n])

						if !chanWrite.write(b.Bytes()) {
			log.Printf("Failed to write to channel: channel closed")
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

				var b bytes.Buffer
				requestIDLength := len(res.options.RequestID)
				bodyChunkLength := n

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
				b.Write(chunkBuffer[:n])

				if !chanWrite.write(b.Bytes()) {
			log.Printf("Failed to write to channel: channel closed")
			return
		}
			}
		}
	}

	{
		var b bytes.Buffer
		requestIDLength := len(res.options.RequestID)

		b.WriteByte(byte(requestIDLength >> 8))
		b.WriteByte(byte(requestIDLength))
		b.WriteString(res.options.RequestID)
		b.WriteByte(0)
		b.WriteByte(3)
		b.WriteString("end")

		if !chanWrite.write(b.Bytes()) {
			log.Printf("Failed to write to channel: channel closed")
			return
		}
	}
}

// dispatchSSEAsync handles SSE connections asynchronously
func dispatchSSEAsync(res fullRequest, chanWrite *safeChannelWriter) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in dispatchSSEAsync for request %s: %v", res.options.RequestID, r)
		}
		activeRequestsMutex.Lock()
		delete(activeRequests, res.options.RequestID)
		activeRequestsMutex.Unlock()
	}()

	// Connect to SSE endpoint
	sseResp, err := res.sseClient.Connect(res.req.Context(), res.options.Options.URL)
	if err != nil {
		// Send error response
		var b bytes.Buffer
		var requestIDLength = len(res.options.RequestID)

		b.WriteByte(byte(requestIDLength >> 8))
		b.WriteByte(byte(requestIDLength))
		b.WriteString(res.options.RequestID)
		b.WriteByte(0)
		b.WriteByte(5)
		b.WriteString("error")
		b.WriteByte(byte(0 >> 8)) // Status code 0
		b.WriteByte(byte(0))

		var message = "SSE connection failed: " + err.Error()
		var messageLength = len(message)

		b.WriteByte(byte(messageLength >> 8))
		b.WriteByte(byte(messageLength))
		b.WriteString(message)

		if !chanWrite.write(b.Bytes()) {
			log.Printf("Failed to write to channel: channel closed")
			return
		}
		return
	}
	defer sseResp.Close()

	// Send initial response with headers
	{
		var b bytes.Buffer
		var headerLength = len(sseResp.Response.Header)
		var requestIDLength = len(res.options.RequestID)
		var finalUrlLength = len(res.options.Options.URL)

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
			var nameLength = len(name)
			var valuesLength = len(values)

			b.WriteByte(byte(nameLength >> 8))
			b.WriteByte(byte(nameLength))
			b.WriteString(name)
			b.WriteByte(byte(valuesLength >> 8))
			b.WriteByte(byte(valuesLength))

			for _, value := range values {
				var valueLength = len(value)

				b.WriteByte(byte(valueLength >> 8))
				b.WriteByte(byte(valueLength))
				b.WriteString(value)
			}
		}

		if !chanWrite.write(b.Bytes()) {
			log.Printf("Failed to write to channel: channel closed")
			return
		}
	}

	// Read SSE events
	for {
		select {
		case <-res.req.Context().Done():
			debugLogger.Printf("SSE request %s was canceled", res.options.RequestID)
			break

		default:
			event, err := sseResp.NextEvent()
			if err != nil {
				if err == io.EOF {
					// Normal end of stream
					break
				}
				debugLogger.Printf("SSE read error: %s", err.Error())
				break
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
			var b bytes.Buffer
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

			if !chanWrite.write(b.Bytes()) {
			log.Printf("Failed to write to channel: channel closed")
			return
		}
		}
	}

	// Send end message
	{
		var b bytes.Buffer
		requestIDLength := len(res.options.RequestID)

		b.WriteByte(byte(requestIDLength >> 8))
		b.WriteByte(byte(requestIDLength))
		b.WriteString(res.options.RequestID)
		b.WriteByte(0)
		b.WriteByte(3)
		b.WriteString("end")

		if !chanWrite.write(b.Bytes()) {
			log.Printf("Failed to write to channel: channel closed")
			return
		}
	}
}

// dispatchWebSocketAsync handles WebSocket connections asynchronously with full bidirectional support
func dispatchWebSocketAsync(res fullRequest, chanWrite *safeChannelWriter) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in dispatchWebSocketAsync for request %s: %v", res.options.RequestID, r)
		}
		activeRequestsMutex.Lock()
		delete(activeRequests, res.options.RequestID)
		activeRequestsMutex.Unlock()

		// Remove from active WebSockets
		activeWebSocketsMutex.Lock()
		delete(activeWebSockets, res.options.RequestID)
		activeWebSocketsMutex.Unlock()
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

	// Create WebSocket connection object
	wsConn := &WebSocketConnection{
		Conn:       conn,
		RequestID:  res.options.RequestID,
		URL:        res.options.Options.URL,
		ReadyState: 1, // OPEN
		commandChan: make(chan WebSocketCommand, 100),
		closeChan:  make(chan struct{}),
		chanWrite:  chanWrite,
		protocol:   negotiatedProtocol,
		extensions: negotiatedExtensions,
	}

	// Register the WebSocket connection
	activeWebSocketsMutex.Lock()
	activeWebSockets[res.options.RequestID] = wsConn
	activeWebSocketsMutex.Unlock()

	// Send initial response with headers
	sendWebSocketResponse(chanWrite, res.options.RequestID, res.options.Options.URL, resp)

	// Send ws_open event
	sendWebSocketOpen(chanWrite, res.options.RequestID, negotiatedProtocol, negotiatedExtensions)

	// If there's body data, send it as the first WebSocket message
	if res.options.Options.Body != "" {
		err := conn.WriteMessage(websocket.TextMessage, []byte(res.options.Options.Body))
		if err != nil {
			debugLogger.Printf("WebSocket write error: %s", err.Error())
		}
	}

	// Create channels for goroutine coordination
	readDone := make(chan struct{})
	writeDone := make(chan struct{})

	// Goroutine to handle incoming WebSocket messages
	go func() {
		defer close(readDone)
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
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
		defer close(writeDone)
		for {
			select {
			case cmd := <-wsConn.commandChan:
				switch cmd.Type {
				case "send":
					msgType := websocket.TextMessage
					if cmd.IsBinary {
						msgType = websocket.BinaryMessage
					}
					err := conn.WriteMessage(msgType, cmd.Data)
					if err != nil {
						debugLogger.Printf("WebSocket send error: %s", err.Error())
						sendWebSocketError(chanWrite, res.options.RequestID, res.options.Options.URL, nil, err)
					}

				case "close":
					wsConn.mu.Lock()
					wsConn.ReadyState = 2 // CLOSING
					wsConn.mu.Unlock()

					closeCode := cmd.CloseCode
					if closeCode == 0 {
						closeCode = websocket.CloseNormalClosure
					}

					err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, cmd.CloseReason))
					if err != nil {
						debugLogger.Printf("WebSocket close error: %s", err.Error())
					}

					sendWebSocketClose(chanWrite, res.options.RequestID, closeCode, cmd.CloseReason)
					return

				case "ping":
					err := conn.WriteMessage(websocket.PingMessage, cmd.Data)
					if err != nil {
						debugLogger.Printf("WebSocket ping error: %s", err.Error())
					}

				case "pong":
					err := conn.WriteMessage(websocket.PongMessage, cmd.Data)
					if err != nil {
						debugLogger.Printf("WebSocket pong error: %s", err.Error())
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

	// Wait for either read or write to complete
	select {
	case <-readDone:
		close(wsConn.closeChan)
		<-writeDone
	case <-writeDone:
		close(wsConn.closeChan)
		<-readDone
	case <-res.req.Context().Done():
		debugLogger.Printf("WebSocket request %s was canceled", res.options.RequestID)
		close(wsConn.closeChan)
		<-readDone
		<-writeDone
	}

	// Update connection state to CLOSED
	wsConn.mu.Lock()
	wsConn.ReadyState = 3
	wsConn.mu.Unlock()

	// Send end message
	sendWebSocketEnd(chanWrite, res.options.RequestID)
}

// Helper functions for sending WebSocket messages
func sendWebSocketError(chanWrite *safeChannelWriter, requestID, url string, resp *nhttp.Response, err error) {
	var b bytes.Buffer
	var requestIDLength = len(requestID)

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

	var message = err.Error()
	var messageLength = len(message)

	b.WriteByte(byte(messageLength >> 8))
	b.WriteByte(byte(messageLength))
	b.WriteString(message)

	chanWrite.write(b.Bytes())
}

func sendWebSocketResponse(chanWrite *safeChannelWriter, requestID, url string, resp *nhttp.Response) {
	var b bytes.Buffer
	var headerLength = len(resp.Header)
	var requestIDLength = len(requestID)
	var finalUrlLength = len(url)

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
		var nameLength = len(name)
		var valuesLength = len(values)

		b.WriteByte(byte(nameLength >> 8))
		b.WriteByte(byte(nameLength))
		b.WriteString(name)
		b.WriteByte(byte(valuesLength >> 8))
		b.WriteByte(byte(valuesLength))

		for _, value := range values {
			var valueLength = len(value)

			b.WriteByte(byte(valueLength >> 8))
			b.WriteByte(byte(valueLength))
			b.WriteString(value)
		}
	}

	chanWrite.write(b.Bytes())
}

func sendWebSocketOpen(chanWrite *safeChannelWriter, requestID, protocol, extensions string) {
	openMsg := map[string]interface{}{
		"type":       "open",
		"protocol":   protocol,
		"extensions": extensions,
	}

	msgBytes, _ := json.Marshal(openMsg)

	var b bytes.Buffer
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

	chanWrite.write(b.Bytes())
}

func sendWebSocketMessage(chanWrite *safeChannelWriter, requestID string, messageType int, message []byte) {
	var b bytes.Buffer
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

	chanWrite.write(b.Bytes())
}

func sendWebSocketClose(chanWrite *safeChannelWriter, requestID string, code int, reason string) {
	closeMsg := map[string]interface{}{
		"type":   "close",
		"code":   code,
		"reason": reason,
	}

	msgBytes, _ := json.Marshal(closeMsg)

	var b bytes.Buffer
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

	chanWrite.write(b.Bytes())
}

func sendWebSocketEnd(chanWrite *safeChannelWriter, requestID string) {
	var b bytes.Buffer
	requestIDLength := len(requestID)

	b.WriteByte(byte(requestIDLength >> 8))
	b.WriteByte(byte(requestIDLength))
	b.WriteString(requestID)
	b.WriteByte(0)
	b.WriteByte(3)
	b.WriteString("end")

	chanWrite.write(b.Bytes())
}

func writeSocket(chanWrite chan []byte, wsSocket *websocket.Conn) {
	for buf := range chanWrite {
		err := wsSocket.WriteMessage(websocket.BinaryMessage, buf)

		if err != nil {
			log.Print("Socket WriteMessage Failed" + err.Error())
			continue
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
			log.Print("Socket Error", err)
			return
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
				activeRequestsMutex.Lock()
				if cancel, exists := activeRequests[requestId]; exists {
					cancel()
					delete(activeRequests, requestId)
				}
				activeRequestsMutex.Unlock()
				continue
			}
			// Handle WebSocket commands
			if action == "ws_send" || action == "ws_close" || action == "ws_ping" || action == "ws_pong" {
				requestId, _ := baseMessage["requestId"].(string)

				activeWebSocketsMutex.RLock()
				wsConn, exists := activeWebSockets[requestId]
				activeWebSocketsMutex.RUnlock()

				if !exists {
					log.Printf("WebSocket connection not found for request ID: %s", requestId)
					continue
				}

				cmd := WebSocketCommand{}

				switch action {
				case "ws_send":
					cmd.Type = "send"
					if dataStr, ok := baseMessage["data"].(string); ok {
						cmd.Data = []byte(dataStr)
					}
					if isBinary, ok := baseMessage["isBinary"].(bool); ok {
						cmd.IsBinary = isBinary
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

				// Send command to WebSocket connection
				select {
				case wsConn.commandChan <- cmd:
					// Command sent successfully
				default:
					log.Printf("WebSocket command channel full for request ID: %s", requestId)
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
		go dispatcherAsync(request, chanWrite)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WSEndpoint exports the main cycletls function as we websocket connection that clients can connect to
func WSEndpoint(w nhttp.ResponseWriter, r *nhttp.Request) {
	upgrader.CheckOrigin = func(r *nhttp.Request) bool { return true }

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
		chanRead := make(chan fullRequest)
		chanWrite := make(chan []byte)
		safeWriter := newSafeChannelWriter(chanWrite)

		go readSocket(chanRead, ws)
		go readProcess(chanRead, safeWriter)

		// Run as main thread - when this exits, mark channel as closed
		writeSocket(chanWrite, ws)
		safeWriter.setClosed()
	}
}

func setupRoutes() {
	nhttp.HandleFunc("/", WSEndpoint)
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

	setupRoutes()
	log.Fatal(nhttp.ListenAndServe(*addr, nil))
}

// Backward compatibility types and functions for integration tests
type Response struct {
	RequestID string            `json:"requestId"`
	Status    int               `json:"status"`
	Body      string            `json:"body"`
	BodyBytes []byte            `json:"bodyBytes"` // New field for binary response data
	Headers   map[string]string `json:"headers"`
	Cookies   []*nhttp.Cookie   `json:"cookies"`
	FinalUrl  string            `json:"finalUrl"`
}

// JSONBody parses the response body as JSON
func (r Response) JSONBody() map[string]interface{} {
	var result map[string]interface{}
	json.Unmarshal([]byte(r.Body), &result)
	return result
}

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
	// Create browser from options
	browser := Browser{
		JA3:                options.Ja3,
		JA4r:               options.Ja4r,
		HTTP2Fingerprint:   options.HTTP2Fingerprint,
		QUICFingerprint:    options.QUICFingerprint,
		UserAgent:          options.UserAgent,
		Cookies:            options.Cookies,
		InsecureSkipVerify: options.InsecureSkipVerify,
		ForceHTTP1:         options.ForceHTTP1,
		ForceHTTP3:         options.ForceHTTP3,
		HeaderOrder:        options.HeaderOrder,
	}

	// Note: Don't automatically set HeaderOrder from UserAgent here as it can interfere with connection management
	// The pseudo-header order should be set through explicit HTTP2Fingerprint or Options.HeaderOrder

	// Create HTTP client with connection reuse
	// Default to true for connection reuse
	enableConnectionReuse := true
	if options.EnableConnectionReuse == false {
		// Only disable if explicitly set to false
		enableConnectionReuse = false
	}

	httpClient, err := newClientWithReuse(
		browser,
		options.Timeout,
		options.DisableRedirect,
		options.UserAgent,
		enableConnectionReuse,
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
	req, err := http.NewRequest(Method, URL, bodyReader)
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

	// Set headers
	for k, v := range options.Headers {
		req.Header.Set(k, v)
	}

	// Make request
	resp, err := httpClient.Do(req)
	if err != nil {
		parsedError := parseError(err)
		return Response{
			Status: parsedError.StatusCode,
			Body:   parsedError.ErrorMsg + " -> " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	// Read body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
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
