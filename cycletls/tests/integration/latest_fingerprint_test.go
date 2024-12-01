//go:build integration
// +build integration

package cycletls_test

import (
	"encoding/json"
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
	PeetPrint         string `json:"peetprint"`
}

var PeetRequests = []AkamaiOptions{
	{"c0a45cc83cb2005bbd2a860db187a357", // Firefox 121
		"771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53,0-23-65281-10-11-16-5-34-51-43-13-45-28-65037,29-23-24-25-256-257,0",
		"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
		"1:65536;4:131072;5:16384|12517377|3:0:0:201,5:0:0:101,7:0:0:1,9:0:7:1,11:0:3:1,13:0:0:241|m,p,a,s",
		"3d9132023bf26a71d40fe766e5c24c9d",
		200},
	{"d742731fb59499b2ca4cf990dd929c0a", // Chrome 120
		"771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,45-27-23-10-13-35-5-65037-16-51-0-18-43-11-17513-65281,29-23-24,0",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"1:65536;3:1000;4:6291456;5:16384;6:262144|15663105|0|m,a,s,p",
		"d8bfc65c373bfcc03d51b3c4e28d4591",
		200},
}

// {"ja3_hash":"aa7744226c695c0b2e440419848cf700", "ja3": "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0", "User-Agent": "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0"}
func TestLatestVersions(t *testing.T) {
	client := cycletls.Init()
	for _, options := range PeetRequests {

		response, err := client.Do("https://tls.peet.ws/api/clean", cycletls.Options{
			Ja3:       options.Ja3,
			UserAgent: options.UserAgent,
		}, "GET")
		if err != nil {
			t.Fatal("Unmarshal Error")
		}
		if response.Status != options.HTTPResponse {
			t.Fatal("Expected Result Not given", response.Status, response.Body, options.HTTPResponse, options.Ja3)
		}
		jsonResp := new(PeetResp)

		err = json.Unmarshal([]byte(response.Body), &jsonResp)
		if err != nil {
			t.Fatal("Unmarshal Error")
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
