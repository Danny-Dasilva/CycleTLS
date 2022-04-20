//go:build integration
// +build integration

package cycletls_test

import (
	//"fmt"
	"log"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestRedirectEnabled(t *testing.T) {

	client := cycletls.Init()
	resp, err := client.Do("https://ssl.com", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	if resp.Status != 200 {
		t.Fatal("Expected {} Got {} for Status", 200, resp.Status)
	}

}

func TestRedirectDisabled(t *testing.T) {

	client := cycletls.Init()
	resp, err := client.Do("https://ssl.com", cycletls.Options{
		Body:            "",
		Ja3:             "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
		UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
		DisableRedirect: true,
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	if resp.Status != 301 {
		t.Fatal("Expected {} Got {} for Status", 301, resp.Status)
	}

}
