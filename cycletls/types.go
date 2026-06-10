// Package cycletls provides TLS fingerprinting capabilities for HTTP requests.
// This file contains shared type definitions used across the package.
package cycletls

import (
	"context"
	"encoding/json"
	nhttp "net/http"
	"strings"
	"time"

	http "github.com/Danny-Dasilva/fhttp"
)

// Time wraps time.Time overriding the json marshal/unmarshal to pass
// timestamp as integer.
type Time struct {
	time.Time
}

// Cookie represents an HTTP cookie as sent in the Set-Cookie header of an
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

// Options sets CycleTLS client options.
type Options struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	// HeaderValues supports request headers with multiple values and exact key casing.
	// It is populated automatically when JSON headers contain array values.
	HeaderValues map[string][]string `json:"-"`
	Body         string              `json:"body"`
	BodyBytes    []byte              `json:"bodyBytes"` // New field for binary request data

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
	InsecureSkipVerify      bool  `json:"insecureSkipVerify"`
	ProxyInsecureSkipVerify *bool `json:"proxyInsecureSkipVerify,omitempty"` // TLS verification for proxy connections. Defaults to true for backward compat. Set to false to verify proxy certificates.

	// Protocol options
	ForceHTTP1 bool   `json:"forceHTTP1"`
	ForceHTTP3 bool   `json:"forceHTTP3"`
	Protocol   string `json:"protocol"` // "http1", "http2", "http3", "websocket", "sse"

	// TLS 1.3 specific options
	TLS13AutoRetry bool `json:"tls13AutoRetry"` // Automatically retry with TLS 1.3 compatible curves (default: true)

	// Connection reuse options
	EnableConnectionReuse bool `json:"enableConnectionReuse"` // Enable connection reuse across requests (default: true)

	// Certificate pinning (optional)
	CertPins []string `json:"certPins,omitempty"` // SHA256 certificate fingerprints for pinning (hex-encoded)
}

// UnmarshalJSON accepts both legacy string header values and multi-value arrays.
func (options *Options) UnmarshalJSON(data []byte) error {
	type optionsAlias Options
	var raw struct {
		optionsAlias
		Headers map[string]json.RawMessage `json:"headers"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*options = Options(raw.optionsAlias)
	options.Headers = make(map[string]string, len(raw.Headers))
	options.HeaderValues = make(map[string][]string, len(raw.Headers))

	for k, rawValue := range raw.Headers {
		var single string
		if err := json.Unmarshal(rawValue, &single); err == nil {
			options.Headers[k] = single
			options.HeaderValues[k] = []string{single}
			continue
		}

		var values []string
		if err := json.Unmarshal(rawValue, &values); err != nil {
			return err
		}
		options.HeaderValues[k] = append([]string(nil), values...)
		if len(values) > 0 {
			options.Headers[k] = values[0]
		} else {
			options.Headers[k] = ""
		}
	}

	if len(options.Headers) == 0 {
		options.Headers = nil
	}
	if len(options.HeaderValues) == 0 {
		options.HeaderValues = nil
	}

	return nil
}

// requestHeaders merges Headers and HeaderValues into a single multi-value map.
// HeaderValues wins when a key is present in both; keys keep their exact casing.
func (options Options) requestHeaders() map[string][]string {
	if len(options.HeaderValues) > 0 {
		headers := make(map[string][]string, len(options.HeaderValues)+len(options.Headers))
		for k, values := range options.HeaderValues {
			headers[k] = append([]string(nil), values...)
		}
		for k, v := range options.Headers {
			if _, ok := headers[k]; !ok {
				headers[k] = []string{v}
			}
		}
		return headers
	}

	headers := make(map[string][]string, len(options.Headers))
	for k, v := range options.Headers {
		headers[k] = []string{v}
	}
	return headers
}

// setRequestHeaders writes the request headers onto an http.Header using direct
// map assignment so the exact key casing is preserved (http.Header.Set would
// canonicalize the key) and multiple values are sent as repeated headers.
func setRequestHeaders(headers http.Header, options Options, skipContentLength bool) {
	for k, values := range options.requestHeaders() {
		if skipContentLength && strings.EqualFold(k, "Content-Length") {
			continue
		}
		headers[k] = append([]string(nil), values...)
	}
}

// hasRequestHeader reports whether a header is present under any key casing.
func hasRequestHeader(options Options, name string) bool {
	for k := range options.requestHeaders() {
		if strings.EqualFold(k, name) {
			return true
		}
	}
	return false
}

// setHeaderExact sets a header under the exact key given, removing any existing
// entries for the same header under different casings first. This keeps the
// "options override headers" semantics that http.Header.Set provided while
// avoiding key canonicalization.
func setHeaderExact(headers http.Header, name, value string) {
	for k := range headers {
		if strings.EqualFold(k, name) {
			delete(headers, k)
		}
	}
	headers[name] = []string{value}
}

// cycleTLSRequest represents an internal request with ID and options.
type cycleTLSRequest struct {
	RequestID string  `json:"requestId"`
	Options   Options `json:"options"`
}

// fullRequest contains all components needed to execute a request.
type fullRequest struct {
	req       *http.Request
	client    http.Client
	options   cycleTLSRequest
	sseClient *SSEClient       // For SSE connections
	wsClient  *WebSocketClient // For WebSocket connections
	err       error            // For early validation errors (e.g., invalid URL)

	// V2 flow control fields
	ctx         context.Context         // Parent context for cancellation
	cancel      context.CancelFunc      // Cancel function for the request
	limiter     *creditWindow           // Credit window for flow control (v2 only)
	wsCommandCh chan WebSocketCommandV2 // Channel for WebSocket commands (v2 only)
}

// CycleTLS is the main client for making requests with TLS fingerprinting.
type CycleTLS struct {
	ReqChan    chan fullRequest
	RespChan   chan Response // V1 default: chan Response for backward compatibility
	RespChanV2 chan []byte   `json:"-"` // V2 performance: chan []byte for opt-in users
}

// Option configures a CycleTLS client.
type Option func(*CycleTLS)

// WithRawBytes enables the performance enhancement channel (RespChanV2 chan []byte).
// Use this option for performance-critical applications that can handle raw byte responses.
func WithRawBytes() Option {
	return func(client *CycleTLS) {
		if client.RespChanV2 == nil {
			client.RespChanV2 = make(chan []byte, 100)
		}
	}
}

// Response represents the result of an HTTP request.
type Response struct {
	RequestID string            `json:"requestId"`
	Status    int               `json:"status"`
	Body      string            `json:"body"`
	BodyBytes []byte            `json:"bodyBytes"` // New field for binary response data
	Headers   map[string]string `json:"headers"`
	Cookies   []*nhttp.Cookie   `json:"cookies"`
	FinalUrl  string            `json:"finalUrl"`
}

// JSONBody parses the response body as JSON and returns a map.
func (r Response) JSONBody() map[string]interface{} {
	var result map[string]interface{}
	json.Unmarshal([]byte(r.Body), &result)
	return result
}
