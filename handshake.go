package main

import (
	"log"
	"net"
	URL "net/url"
	"strings"

	"github.com/Noooste/connectproxy"
	utls "github.com/Noooste/utls"
	Proxy "golang.org/x/net/proxy"
)

func PrepareTransport(helloId utls.ClientHelloID, proxy string, url string) (*utls.UConn, error) {

    //prepare server address (securised address)
    //TODO : put 80
    parsedURL, _ := URL.Parse(url)

    addr := parsedURL.Hostname()

    addr += ":443"

    var conn net.Conn
    var err error

    //if not proxy
    if proxy != "" {

        proxyURI, _ := URL.Parse(proxy)

        //use proxy dialer
        proxyDialer, _ := connectproxy.New(proxyURI, Proxy.Direct)

        conn, err = proxyDialer.Dial("tcp", addr)

        if err != nil {
            return nil, err
        }

        //else
    } else {

        //normal dial
        conn, err = net.Dial("tcp", addr)

        if err != nil {
            return nil, err
        }
    }

    config := utls.Config{
        ServerName: parsedURL.Hostname(),
    }

    //create client
    uconn := utls.UClient(conn, &config, helloId)

    colonPos := strings.LastIndex(addr, ":")
    if colonPos == -1 {
        colonPos = len(addr)
    }

    uconn.SetSNI(addr[:colonPos])

    //tls handshake
    err = uconn.Handshake()
	log.Println(err)
    //return client
    return uconn, err
}
type Request struct {
    Method        string
    Url           string
    Data          string
    Proxy         string
    Header        map[string][]string
    HeaderOrder   []string
    Browser       string
    AllowRedirect bool
}
func getUconn(request *Request) (*utls.UConn, error) {
    var helloId utls.ClientHelloID

    switch request.Browser {
    case "firefox":
        helloId = utls.HelloFirefox_99

    case "chrome":
        helloId = utls.HelloChrome_100

    default:
        panic(request.Browser + " is not supported.\n")
    }

    return PrepareTransport(helloId, request.Proxy, request.Url)
}

func main() {
        getUconn(&Request{Method: "Get", Url: "https://www.coinbase.com/users/oauth_signup?client_id=2d06b9a69c15e183856ff52c250281f6d93f9abef819921eac0d8647bb2b61f9&meta%5Baccount%5D=all&redirect_uri=https%3A%2F%2Fpro.coinbase.com%2Foauth_redirect&response_type=code&scope=user+balance&state=", Browser: "chrome"})

}