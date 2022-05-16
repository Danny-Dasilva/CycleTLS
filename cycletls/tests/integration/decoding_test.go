//go:build integration
// +build integration

package cycletls_test

import (
	//"fmt"
	"log"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestDeflateDecoding(t *testing.T) {

	client := cycletls.Init()
	resp, err := client.Do("https://httpbin.org/deflate", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	if resp.JSONBody()["deflated"] != true {
		t.Fatalf("Deflated not found in Reponse Body: %s", resp.Body)
	}
}
func TestBrotliDecoding(t *testing.T) {

	client := cycletls.Init()
	resp, err := client.Do("https://httpbin.org/brotli", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	if resp.JSONBody()["brotli"] != true {
		t.Fatalf("brotli not found in Reponse Body: %s", resp.Body)
	}
}

func TestGZIPDecoding(t *testing.T) {

	client := cycletls.Init()
	resp, err := client.Do("https://httpbin.org/gzip", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Headers:   map[string]string{"Accept-Encoding": "gzip, deflate"},
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	if resp.JSONBody()["gzipped"] != true {
		t.Fatalf("gzipped not found in Reponse Body: %s", resp.Body)
	}
}
