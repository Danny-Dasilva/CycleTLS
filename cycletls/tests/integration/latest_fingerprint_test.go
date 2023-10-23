//go:build integration
// +build integration

package cycletls_test

import (
	"encoding/json"
	"log"
	"testing"

	// cycletls "../../../cycletls"
	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

type AkamaiOptions struct {
	Ja3Hash           string `json:"ja3_hash"`
	Ja3               string `json:"ja3"`
	UserAgent         string `json:"User-Agent"`
	AkamaiFingerprint string
	AkamaiHash        string

	HTTPResponse int
}

type PeetResp struct {
	Ja3               string `json:"ja3"`
	Ja3Hash           string `json:"ja3_hash"`
	AkamaiFingerprint string `json:"akamai"`
	AkamaiHash        string `json:"akamai_hash"`
}

var PeetRequests = []AkamaiOptions{
	{"aa7744226c695c0b2e440419848cf700", // Firefox 101
		"771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:101.0) Gecko/20100101 Firefox/101.0",
		"1:65536,4:131072,5:16384|12517377|3:0:0:201,5:0:0:101,7:0:0:1,9:0:7:1,11:0:3:1,13:0:0:241|m,a,s,p",
		"55cf0afe9d2dd7ec0cb7f0402594f663",
		200},
	{"e1d8b04eeb8ef3954ec4f49267a783ef", // Chrome 104
		"771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36",
		"1:65536,3:1000,4:6291456,6:262144|15663105|0|m,a,s,p",
		"7ad845f20fc17cc8088a0d9312b17da1",
		200},
}

// {"ja3_hash":"aa7744226c695c0b2e440419848cf700", "ja3": "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0", "User-Agent": "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0"}
func TestHTTP2(t *testing.T) {
	client := cycletls.Init()
	for _, options := range PeetRequests {

		response, err := client.Do("https://tls.peet.ws/api/clean", cycletls.Options{
			Ja3:       options.Ja3,
			UserAgent: options.UserAgent,
		}, "GET")
		if err != nil {
			t.Fatal("Unmarshal Error")
		}
		if response.Status != 502 {
			if response.Status != options.HTTPResponse {
				t.Fatal("Expected Result Not given", response.Status, response.Body, options.HTTPResponse, options.Ja3)
			} else {
				log.Println("ja3er: ", response.Status)
			}
			jsonResp := new(PeetResp)

			err = json.Unmarshal([]byte(response.Body), &jsonResp)
			if err != nil {
				t.Fatal("Unmarshal Error")
			}

			if jsonResp.Ja3Hash != options.Ja3Hash {
				t.Fatal("Expected:", options.Ja3Hash, "Got:", jsonResp.Ja3Hash, "for Ja3Hash")
			}
			if jsonResp.Ja3 != options.Ja3 {
				t.Fatal("Expected:", options.Ja3, "Got:", jsonResp.Ja3, "for Ja3")
			}
			if jsonResp.AkamaiFingerprint != options.AkamaiFingerprint {
				t.Fatal("Expected:", options.AkamaiFingerprint, "Got:", jsonResp.AkamaiFingerprint, "for AkamaiFingerprint", options.UserAgent)
			}
			if jsonResp.AkamaiHash != options.AkamaiHash {
				t.Fatal("Expected:", options.AkamaiHash, "Got:", jsonResp.AkamaiHash, "for AkamaiHash", options.UserAgent)
			}

		}
	}
}
