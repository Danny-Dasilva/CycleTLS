package cycletls

import (
	http "github.com/Danny-Dasilva/fhttp"

	"time"

	"golang.org/x/net/proxy"
)

type Browser struct {
	// Return a greeting that embeds the name in a message.
	JA3                string
	UserAgent          string
	Cookies            []Cookie
	InsecureSkipVerify bool
	forceHTTP1         bool
}

var disabledRedirect = func(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}

func clientBuilder(browser Browser, dialer proxy.ContextDialer, timeout int, disableRedirect bool) http.Client {
	//if timeout is not set in call default to 15
	if timeout == 0 {
		timeout = 15
	}
	client := http.Client{
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
func NewTransport(ja3 string, useragent string) http.RoundTripper {
	return newRoundTripper(Browser{
		JA3:       ja3,
		UserAgent: useragent,
	})
}

// NewTransport creates a new HTTP client transport that modifies HTTPS requests
// to imitiate a specific JA3 hash and User-Agent, optionally specifying a proxy via proxy.ContextDialer.
func NewTransportWithProxy(ja3 string, useragent string, proxy proxy.ContextDialer) http.RoundTripper {
	return newRoundTripper(Browser{
		JA3:       ja3,
		UserAgent: useragent,
	}, proxy)
}

// newClient creates a new http client
func newClient(browser Browser, timeout int, disableRedirect bool, UserAgent string, proxyURL ...string) (http.Client, error) {
	var dialer proxy.ContextDialer
	if len(proxyURL) > 0 && len(proxyURL[0]) > 0 {
		var err error
		dialer, err = newConnectDialer(proxyURL[0], UserAgent)
		if err != nil {
			return http.Client{
				Timeout:       time.Duration(timeout) * time.Second,
				CheckRedirect: disabledRedirect,
			}, err
		}
	} else {
		dialer = proxy.Direct
	}

	return clientBuilder(browser, dialer, timeout, disableRedirect), nil
}
