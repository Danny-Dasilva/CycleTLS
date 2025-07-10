//go:build integration
// +build integration

package cycletls_test

import (
	"encoding/json"
	"log"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestForceHTTP1_h2(t *testing.T) {

	client := cycletls.Init()
	resp, err := client.Do("https://tls.peet.ws/api/all", cycletls.Options{
		Body:       "",
		Ja3:        "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
		ForceHTTP1: false,
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	if resp.Status != 200 {
		t.Fatal("Expected {} Got {} for Status", 200, resp.Status)
	}
	fullResp := new(FullResp)

	err = json.Unmarshal([]byte(resp.Body), &fullResp)
	if err != nil {
		t.Fatal("Unmarshal Error")
	}
	if fullResp.HTTP_Version != "h2" {
		t.Fatal("Expected:", "h2", "Got:", fullResp.HTTP_Version, "for fullResp")
	}

}

func TestForceHTTP1_h1(t *testing.T) {

	client := cycletls.Init()
	resp, err := client.Do("https://tls.peet.ws/api/all", cycletls.Options{
		Body:       "",
		Ja3:        "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
		ForceHTTP1: true,
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	if resp.Status != 200 {
		t.Fatal("Expected {} Got {} for Status", 200, resp.Status)
	}
	fullResp := new(FullResp)

	err = json.Unmarshal([]byte(resp.Body), &fullResp)
	if err != nil {
		t.Log("Unmarshal Error")
	}
	if fullResp.HTTP_Version != "HTTP/1.1" {
		t.Log("Expected:", "HTTP/1.1", "Got:", fullResp.HTTP_Version, "for fullResp")
	}

}
