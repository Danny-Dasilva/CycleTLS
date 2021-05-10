package cycletls

import (
	"golang.org/x/net/proxy"
	"net/http"
)

type browser struct {
	// Return a greeting that embeds the name in a message.
	JA3       string
	UserAgent string
	Cookies   []Cookie
}

// New Client
func newClient(browser browser, proxyURL ...string) (http.Client, error) {
	//fix check PR
	if len(proxyURL) > 0 && len(proxyURL[0]) > 0 {
		dialer, err := newConnectDialer(proxyURL[0])
		if err != nil {
			return http.Client{}, err
		}
		return http.Client{
			Transport: newRoundTripper(browser, dialer),
		}, nil
	}
	return http.Client{
		Transport: newRoundTripper(browser, proxy.Direct),
	}, nil

}
