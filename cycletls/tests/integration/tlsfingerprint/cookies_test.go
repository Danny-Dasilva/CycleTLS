//go:build integration
// +build integration

package tlsfingerprint_test

import (
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestCookiesRead(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	opts.Cookies = []cycletls.Cookie{
		{Name: "test", Value: "value1"},
		{Name: "session", Value: "abc123"},
	}

	resp, err := client.Do(TestServerURL+"/cookies", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var cookiesResp CookiesResponse
	parseJSONResponse(t, resp.Body, &cookiesResp)

	// Verify cookies are echoed back
	if cookiesResp.Cookies == nil {
		t.Fatal("Expected cookies in response, got nil")
	}

	if val, ok := cookiesResp.Cookies["test"]; !ok {
		t.Error("Expected 'test' cookie in response")
	} else if val != "value1" {
		t.Errorf("Expected cookies['test'] = 'value1', got '%s'", val)
	}

	if val, ok := cookiesResp.Cookies["session"]; !ok {
		t.Error("Expected 'session' cookie in response")
	} else if val != "abc123" {
		t.Errorf("Expected cookies['session'] = 'abc123', got '%s'", val)
	}
}

func TestCookiesSet(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()

	resp, err := client.Do(TestServerURL+"/cookies/set?name=value&foo=bar", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var cookiesResp CookiesResponse
	parseJSONResponse(t, resp.Body, &cookiesResp)

	// Verify the cookies were set
	if cookiesResp.Cookies == nil {
		t.Fatal("Expected cookies in response, got nil")
	}

	if val, ok := cookiesResp.Cookies["name"]; !ok {
		t.Error("Expected 'name' cookie in response")
	} else if val != "value" {
		t.Errorf("Expected cookies['name'] = 'value', got '%s'", val)
	}

	if val, ok := cookiesResp.Cookies["foo"]; !ok {
		t.Error("Expected 'foo' cookie in response")
	} else if val != "bar" {
		t.Errorf("Expected cookies['foo'] = 'bar', got '%s'", val)
	}
}
