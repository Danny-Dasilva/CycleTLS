package cclient

import (
	"golang.org/x/net/proxy"
	"net/http"
	"fmt"

	utls "github.com/refraction-networking/utls"
)

func NewClient(clientHello utls.ClientHelloID, proxyUrl ...string) (http.Client, error) {
	if len(proxyUrl) > 0 && len(proxyUrl[0]) > 0 {
		fmt.Println(proxyUrl[0])
		dialer, err := newConnectDialer(proxyUrl[0])
		if err != nil {
			return http.Client{}, err
		}
		fmt.Println("form request", dialer)
		return http.Client{
			Transport: newRoundTripper(clientHello, dialer),
		}, nil
	} else {
		return http.Client{
			Transport: newRoundTripper(clientHello, proxy.Direct),
		}, nil
	}
}
