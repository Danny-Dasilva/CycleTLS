package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

type proxyError struct {
	err           error
	code          int
	dialingFailed bool
}

func (e proxyError) Error() string {
	return fmt.Sprintf("proxyError: %v", e.err)
}

type myDialer struct {
	net.Dialer
	f func(ctx context.Context, network, addr string) (net.Conn, error)
}

func myDialerNew(d net.Dialer) *myDialer {
	return &myDialer{
		Dialer: d,
		f:      d.DialContext,
	}
}

func (dc *myDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	con, err := dc.f(ctx, network, addr)
	if err != nil {
		return nil, proxyError{
			err:           err,
			dialingFailed: true,
		}
	}
	return con, nil
}

func main() {

	tr := &http.Transport{
		DialContext: myDialerNew(net.Dialer{
			Timeout:   1 * time.Millisecond,
			KeepAlive: 1 * time.Second,
			DualStack: true,
		}).DialContext,
		TLSHandshakeTimeout:   200 * time.Millisecond,
		ResponseHeaderTimeout: 500 * time.Millisecond,
		IdleConnTimeout:       5 * time.Second,
	}

	go func(rt http.RoundTripper) {
		for {
			time.Sleep(1 * time.Second)

			req, err := http.NewRequest("GET", "http://127.0.0.1:10000/", nil)
			if err != nil {
				log.Printf("Failed to create request: %v", err)
				continue
			}

			resp, err := rt.RoundTrip(req)
			if err != nil {
				if perr, ok := err.(proxyError); ok {
					if nerr, ok := perr.err.(interface {
						Temporary() bool
						Timeout() bool
					}); ok {
						log.Printf("Failed to do roundtrip %v %v: %v", nerr.Temporary(), nerr.Timeout(), perr.err)
					}
					log.Printf("dial error: %v", perr)
				} else {
					if nerr, ok := err.(interface {
						Temporary() bool
						Timeout() bool
					}); ok {
						log.Printf("Failed to do roundtrip %v %v: %v", nerr.Temporary(), nerr.Timeout(), err)
					}
					log.Printf("not a dial error: %v", err)
				}
				continue
			}
			resp.Body.Close()
			log.Printf("resp status: %v", resp.Status)
		}
	}(tr)

	ch := make(chan struct{})
	<-ch
}
