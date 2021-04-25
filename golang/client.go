package main

import (
	"golang.org/x/net/proxy"
	"net/http"

)
type Browser struct {
	JA3       string
	UserAgent string
	Cookies   []Cookie
}




func NewClient(browser Browser, proxyUrl ...string) (http.Client, error) {
	//fix check PR
	if len(proxyUrl) > 0 && len(proxyUrl[0]) > 0 {
		dialer, err := newConnectDialer(proxyUrl[0])
		if err != nil {
			return http.Client{}, err
		}
		return http.Client{
			Transport: newRoundTripper(browser, dialer),
		}, nil
	} else {
		return http.Client{
			Transport: newRoundTripper(browser, proxy.Direct),
		}, nil
	}
}
