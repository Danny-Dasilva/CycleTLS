//go:build integration
// +build integration

package cycletls_test

import (
	//"fmt"
	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
	"log"
	"runtime"
	"testing"
)

func TestProxySuccess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping this test on non-linux platforms")
		return
	}
	client := cycletls.Init()
	resp, err := client.Do("https://ipinfo.io/json", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Proxy:     "socks5://abc:123@127.0.0.1:1087",
		Headers: map[string]string{
			"Accept": "Application/json, text/plain, */*",
		},
	}, "GET")
	if err != nil {
		t.Fatalf("Request Failed: " + err.Error())
	}
	if resp.Status != 200 {
		t.Fatalf("Expected %d Got %d for Status", 200, resp.Status)
	}
	log.Print("Body: " + resp.Body)
}
func TestSocks4Proxy(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping this test on non-linux platforms")
		return
	}
	client := cycletls.Init()
	resp, err := client.Do("https://ipinfo.io/json", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Proxy:     "socks4://abc:123@127.0.0.1:1087",
		Headers: map[string]string{
			"Accept": "Application/json, text/plain, */*",
		},
	}, "GET")
	if err != nil {
		t.Fatalf("Request Failed: " + err.Error())
	}
	if resp.Status != 200 {
		t.Fatalf("Expected %d Got %d for Status", 200, resp.Status)
	}
	log.Print("Body: " + resp.Body)
}

func TestSocks5hProxy(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping this test on non-linux platforms")
		return
	}
	client := cycletls.Init()
	resp, err := client.Do("https://ipinfo.io/json", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Proxy:     "socks5h://abc:123@127.0.0.1:1087",
		Headers: map[string]string{
			"Accept": "Application/json, text/plain, */*",
		},
	}, "GET")
	if err != nil {
		t.Fatalf("Request Failed: " + err.Error())
	}
	if resp.Status != 200 {
		t.Fatalf("Expected %d Got %d for Status", 200, resp.Status)
	}
	log.Print("Body: " + resp.Body)
}