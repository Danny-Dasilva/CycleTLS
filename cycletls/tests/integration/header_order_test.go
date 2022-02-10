//go:build integration
// +build integration

package cycletls_test

import (
	//"fmt"
	"log"
	"testing"
	"strings"
	"github.com/PuerkitoBio/goquery"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

//TODO rewrite this so its not reliant on goquery

func TestDefaultHeaderOrder(t *testing.T) {
	client := cycletls.Init()

	resp, err := client.Do("https://pgl.yoyo.org/http/browser-headers.php", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Headers: map[string]string{ "host": "pgl.yoyo.org","connection": "keep-alive", "cache-control": "no-cache"},
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(resp.Body)))

    if err != nil {
        log.Fatal(err)
    }

    headername := doc.Find(".headername").Text()
	// headervalue := doc.Find(".headerval").Text()
	expected_order := "Accept-Encoding:User-Agent:Cache-Control:Connection:Host:"

	if expected_order != headername {
		t.Fatalf("Headers are ordered incorrectly: %s", headername)
	}
}

func TestCustomHeaderOrder(t *testing.T) {
	client := cycletls.Init()
	resp, err := client.Do("https://pgl.yoyo.org/http/browser-headers.php", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Headers: map[string]string{ "host": "pgl.yoyo.org","connection": "keep-alive", "cache-control": "no-cache"},
		HeaderOrder: []string{"cache-control", "connection", "host"},
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(resp.Body)))

    if err != nil {
        log.Fatal(err)
    }

    headername := doc.Find(".headername").Text()
	expected_order := "Accept-Encoding:User-Agent:Host:Connection:Cache-Control:"

	if expected_order != headername {
		t.Fatalf("Custom Headers are ordered incorrectly: %s", headername)
	}
}

func TestCustomHeaderOrderFailure(t *testing.T) {
	client := cycletls.Init()
	resp, err := client.Do("https://pgl.yoyo.org/http/browser-headers.php", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Headers: map[string]string{ "host": "pgl.yoyo.org","connection": "keep-alive", "cache-control": "no-cache"},
		HeaderOrder: []string{"cache-control", "connection", "host"},
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(resp.Body)))

    if err != nil {
        log.Fatal(err)
    }

    headername := doc.Find(".headername").Text()
	unexpected_order := "Accept-Encoding:User-Agent:Cache-Control:Connection:Host:"

	if unexpected_order == headername {
		t.Fatalf("Custom Headers Failures are ordered incorrectly: %s", headername)
	}
}