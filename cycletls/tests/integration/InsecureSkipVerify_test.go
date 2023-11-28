//go:build integration
// +build integration

package cycletls_test

import (
	"log"
	"strings"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

type FullResp struct {
	Method       string `json:"method"`
	HTTP_Version string `json:"http_version"`
}

func TestInsecureSkipVerify_true(t *testing.T) {

	client := cycletls.Init()
	resp, err := client.Do("https://expired.badssl.com", cycletls.Options{
		Body:               "",
		Ja3:                "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
		UserAgent:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
		InsecureSkipVerify: false,
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}

	expectedError := "uTlsConn.Handshake() error: tls: failed to verify certificate: x509: certificate has expired or is not yet valid"
	if !strings.Contains(resp.Body, expectedError) {
		t.Fatalf("Expected response body to contain error: %q, but got: %q", expectedError, resp.Body)
	}

}

func TestInsecureSkipVerify_false(t *testing.T) {

	client := cycletls.Init()
	resp, err := client.Do("https://expired.badssl.com", cycletls.Options{
		Body:               "",
		Ja3:                "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
		UserAgent:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
		InsecureSkipVerify: true,
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	if resp.Status != 200 {
		t.Fatal("Expected {} Got {} for Status", 200, resp.Status)
	}

}
