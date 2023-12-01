package cycletls

import (
	"context"
	http "github.com/Danny-Dasilva/fhttp"
	"github.com/Danny-Dasilva/fhttp/http2"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/proxy"
	"net"
	"time"
)

type HttpClientBuilder struct {
	Browser       *Browser
	ProxyUrl      string
	ClientHelloId *utls.ClientHelloID

	MaxIdleConnections   int
	MaxConnectionPerHost int
	Timeout              time.Duration

	connectDialer proxy.ContextDialer

	Product *http.Client
	Err     error
}

func (b *HttpClientBuilder) SetProxyUrl(proxyUrl string) *HttpClientBuilder {
	return b
}

func (b *HttpClientBuilder) Build() (*http.Client, error) {
	if b.MaxConnectionPerHost <= 0 {
		b.MaxConnectionPerHost = 1
	}

	if b.MaxIdleConnections < 0 {
		b.MaxIdleConnections = 0
	}

	b.buildContextDialer()

	tlsDialer := newRoundTripper(*b.Browser, b.connectDialer).(*roundTripper)
	tlsDialer.ClientHelloId = b.ClientHelloId

	httpTransport := &http.Transport{

		MaxIdleConns:    b.MaxIdleConnections,
		MaxConnsPerHost: b.MaxConnectionPerHost,

		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return tlsDialer.DoDialTlsContext(ctx, network, addr)
		},

		PostProcessPersistConn: func(persistConn *http.PersistConn) (*http.PersistConn, error) {
			uConn, ok := persistConn.GetConn().(*utls.UConn)
			if !ok {
				return persistConn, nil
			}

			if uConn.ConnectionState().NegotiatedProtocol != http2.NextProtoTLS {
				return persistConn, nil
			}

			// TODO: log.DEBUG, I don't know the equivalency of go
			ua := parseUserAgent(b.Browser.UserAgent)
			h2Transport := &http2.Transport{
				DialTLS: func(network, addr string, cfg *utls.Config) (net.Conn, error) {
					return persistConn.GetConn(), nil
				},
				Navigator: ua.UserAgent,
			}
			persistConn.SetAlt(h2Transport)

			return persistConn, nil
		},
	}

	realTransport := &customizingRequestTransport{
		Browser:  b.Browser,
		Delegate: httpTransport,
	}

	return &http.Client{
		Transport: realTransport,
		Timeout:   b.Timeout,
	}, nil

}

func (b *HttpClientBuilder) buildContextDialer() {
	if b.Err != nil {
		return
	}

	if len(b.ProxyUrl) == 0 {
		b.connectDialer = proxy.Direct
		return
	}

	dialer, err := newConnectDialer(b.ProxyUrl, b.Browser.UserAgent)
	if err != nil {
		b.Err = err
	}

	b.connectDialer = dialer
}

type customizingRequestTransport struct {
	Browser  *Browser
	Delegate http.RoundTripper
}

func (t *customizingRequestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	const UserAgentHeader = "User-Agent"
	if req.Header == nil {
		req.Header = make(http.Header)
	}

	userAgents, ok := req.Header[UserAgentHeader]
	if !ok || len(userAgents) == 0 {
		req.Header[UserAgentHeader] = []string{t.Browser.UserAgent}
	}

	return t.Delegate.RoundTrip(req)
}
