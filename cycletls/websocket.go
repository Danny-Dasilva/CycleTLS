package cycletls

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/proxy"
)

// WebSocketClient represents a client for WebSocket connections
type WebSocketClient struct {
	// Dialer is the websocket dialer
	Dialer *websocket.Dialer

	// HTTP client for WebSocket handshake
	HTTPClient *http.Client

	// Headers to be included in the WebSocket handshake
	Headers http.Header
}

// NewWebSocketClient creates a new WebSocket client
func NewWebSocketClient(tlsConfig *utls.Config, headers http.Header) *WebSocketClient {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       ConvertUtlsConfig(tlsConfig),
	}

	client := &http.Client{
		Transport: transport,
	}

	// Convert TLS config but ensure HTTP/1.1 for WebSocket
	tlsConf := ConvertUtlsConfig(tlsConfig)
	if tlsConf != nil {
		// WebSocket requires HTTP/1.1, so remove HTTP/2 protocols
		tlsConf.NextProtos = []string{"http/1.1"}
	}

	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		TLSClientConfig:  tlsConf,
	}

	return &WebSocketClient{
		Dialer:     dialer,
		HTTPClient: client,
		Headers:    headers,
	}
}

// UseProxyDialer sends the WebSocket connection through dialer in place of the
// proxy named by the environment. dialer is the same proxy dialer the HTTP
// path uses (HTTP CONNECT, HTTPS CONNECT or SOCKS), so the upstream server
// sees the proxy's address. The TLS handshake with the server still runs here,
// over the tunnel.
func (wsc *WebSocketClient) UseProxyDialer(dialer proxy.ContextDialer) {
	wsc.Dialer.Proxy = nil
	wsc.Dialer.NetDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		if _, ok := conn.(*http2Conn); ok {
			return noDeadlineConn{conn}, nil
		}
		return conn, nil
	}
}

// noDeadlineConn wraps a tunnel that runs as one stream of an HTTP/2
// connection to the proxy. A deadline set on it would land on that whole
// HTTP/2 connection, and a write deadline left behind by one WebSocket control
// frame would fail the proxy connection's own later writes. Deadlines are
// dropped here; closing the connection is what ends a blocked read or write.
type noDeadlineConn struct {
	net.Conn
}

func (noDeadlineConn) SetDeadline(time.Time) error      { return nil }
func (noDeadlineConn) SetReadDeadline(time.Time) error  { return nil }
func (noDeadlineConn) SetWriteDeadline(time.Time) error { return nil }

// Connect establishes a WebSocket connection
func (wsc *WebSocketClient) Connect(urlStr string) (*websocket.Conn, *http.Response, error) {
	return wsc.ConnectContext(context.Background(), urlStr)
}

// ConnectContext establishes a WebSocket connection. Cancelling ctx stops a
// handshake that is still in progress; it does not close a connection that
// has been returned.
func (wsc *WebSocketClient) ConnectContext(ctx context.Context, urlStr string) (*websocket.Conn, *http.Response, error) {
	// Parse the URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, nil, err
	}

	// Determine the scheme for WebSocket
	var scheme string
	switch u.Scheme {
	case "http":
		scheme = "ws"
	case "https":
		scheme = "wss"
	case "ws", "wss":
		scheme = u.Scheme
	default:
		// Default to ws
		scheme = "ws"
	}

	// Create a new URL with the WebSocket scheme
	wsURL := url.URL{
		Scheme:   scheme,
		Host:     u.Host,
		Path:     u.Path,
		RawQuery: u.RawQuery,
	}

	// Connect to the WebSocket server
	conn, resp, err := wsc.Dialer.DialContext(ctx, wsURL.String(), wsc.Headers)
	if err != nil {
		return nil, resp, err
	}

	return conn, resp, nil
}

// WebSocketResponse represents a response from a WebSocket connection
type WebSocketResponse struct {
	// Conn is the WebSocket connection
	Conn *websocket.Conn

	// Response is the HTTP response from the WebSocket handshake
	Response *http.Response
}

// Close closes the WebSocket connection
func (wsr *WebSocketResponse) Close() error {
	if wsr.Conn != nil {
		// Send close message
		err := wsr.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			return err
		}

		// Close the connection
		return wsr.Conn.Close()
	}
	return nil
}

// Send sends a message over the WebSocket connection
func (wsr *WebSocketResponse) Send(messageType int, data []byte) error {
	return wsr.Conn.WriteMessage(messageType, data)
}

// Receive receives a message from the WebSocket connection
func (wsr *WebSocketResponse) Receive() (int, []byte, error) {
	return wsr.Conn.ReadMessage()
}
