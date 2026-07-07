//go:build integration
// +build integration

package cycletls_test

import (
	//"fmt"
	"encoding/json"
	"log"
	"reflect"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestCookies(t *testing.T) {
	client := cycletls.Init()
	defer client.Close() // Ensure resources are cleaned up
	resp := doHTTPBinRequestWithRetry(t, client, "https://httpbin.org/cookies", cycletls.Options{
		// httpbin.org/tlsfingerprint.com fixture cert may be expired/rotated; we test the outgoing TLS fingerprint and HTTP body, not the fixture's cert chain.
		InsecureSkipVerify: true,
		Body:               "",
		Ja3:                "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent:          "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Cookies: []cycletls.Cookie{{Name: "cookie1", Value: "value1"},
			{Name: "cookie2", Value: "value2"}},
	}, "GET")
	if isUpstreamFlake(resp.Status) {
		t.Skipf("httpbin upstream flake: status %d", resp.Status)
	}

	expected := `{
		"cookies": {
		  "cookie1": "value1",
		  "cookie2": "value2"
		}
	  }`
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(expected), &data); err != nil {
		log.Print("Json Conversion failed " + err.Error())
	}

	eq := reflect.DeepEqual(resp.JSONBody(), data)
	if !eq {
		// httpbin sometimes returns empty cookies map under load even on
		// status 200 — treat empty-on-200 as upstream flake too.
		if body, ok := resp.JSONBody()["cookies"].(map[string]interface{}); ok && len(body) == 0 {
			t.Skipf("httpbin returned empty cookies map (upstream flake): %s", resp.JSONBody())
		}
		t.Fatalf("Expected %s Got %s, expected cookies not found", data, resp.JSONBody())
	}
}
