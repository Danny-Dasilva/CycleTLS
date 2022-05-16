//go:build integration
// +build integration

package cycletls_test

import (
	//"fmt"
	// "encoding/json"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

var TLS13Results = []CycleTLSOptions{
	{"b32309a26951912be7dba376398abc3b", // HelloChrome_100
		"771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27,29-23-24,0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36",
		200},
}

func TestTLS_13(t *testing.T) {
	client := cycletls.Init()
	for _, options := range TLS13Results {

		// response, err := client.Do("https://ja3er.com/json", cycletls.Options{
		// 	Ja3:       options.Ja3,
		// 	UserAgent: options.UserAgent,
		// }, "GET")
		// if err != nil {
		// 	t.Fatal("Unmarshal Error")
		// }
		// if response.Status != 502 {
		// 	if response.Status != options.HTTPResponse {
		// 		t.Fatal("Expected Result Not given")
		// 	} else {
		// 		log.Println("ja3er: ", response.Status)
		// 	}
		// 	ja3resp := new(Ja3erResp)

		// 	err = json.Unmarshal([]byte(response.Body), &ja3resp)
		// 	if err != nil {
		// 		t.Fatal("Unmarshal Error")
		// 	}

		// 	if ja3resp.Ja3Hash != options.Ja3Hash {
		// 		t.Fatal("Expected:", options.Ja3Hash, "Got:", ja3resp.Ja3Hash, "for Ja3Hash")
		// 	}
		// 	if ja3resp.Ja3 != options.Ja3 {
		// 		t.Fatal("Expected:", options.Ja3, "Got:", ja3resp.Ja3, "for Ja3")
		// 	}
		// 	if ja3resp.UserAgent != options.UserAgent {
		// 		t.Fatal("Expected:", options.UserAgent, "Got:", ja3resp.UserAgent, "for UserAgent")
		// 	}

		// }

		response, err := client.Do("https://tls13.1d.pw", cycletls.Options{
			Ja3:       options.Ja3,
			UserAgent: options.UserAgent,
		}, "GET")
		if err != nil {
			t.Fatal(err)
		}
		if response.Status != options.HTTPResponse {
			t.Fatal("Expected:", options.HTTPResponse, "Got:", response.Status, "for", options.Ja3Hash, response.Body)
		}
	}
}
