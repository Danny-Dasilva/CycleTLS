//go:build integration
// +build integration

package tlsfingerprint_test

import (
	"testing"
)

func TestGet(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/get?foo=bar", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var echoResp EchoResponse
	parseJSONResponse(t, resp.Body, &echoResp)

	// Verify query args are present
	if echoResp.Args == nil {
		t.Fatal("Expected args in response, got nil")
	}
	if val, ok := echoResp.Args["foo"]; !ok {
		t.Error("Expected 'foo' in args")
	} else if val != "bar" {
		t.Errorf("Expected args['foo'] = 'bar', got %v", val)
	}
}

func TestPost(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	opts.Body = `{"message": "hello"}`
	opts.Headers = map[string]string{
		"Content-Type": "application/json",
	}

	resp, err := client.Do(TestServerURL+"/post", opts, "POST")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var echoResp EchoResponse
	parseJSONResponse(t, resp.Body, &echoResp)

	// Verify the data was received
	if echoResp.Data == "" && echoResp.JSON == nil {
		t.Error("Expected data or json in response")
	}
}

func TestPut(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	opts.Body = `{"update": "data"}`
	opts.Headers = map[string]string{
		"Content-Type": "application/json",
	}

	resp, err := client.Do(TestServerURL+"/put", opts, "PUT")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var echoResp EchoResponse
	parseJSONResponse(t, resp.Body, &echoResp)
}

func TestPatch(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	opts.Body = `{"patch": "value"}`
	opts.Headers = map[string]string{
		"Content-Type": "application/json",
	}

	resp, err := client.Do(TestServerURL+"/patch", opts, "PATCH")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var echoResp EchoResponse
	parseJSONResponse(t, resp.Body, &echoResp)
}

func TestDelete(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/delete", opts, "DELETE")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var echoResp EchoResponse
	parseJSONResponse(t, resp.Body, &echoResp)
}

func TestAnything(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/anything", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	var echoResp EchoResponse
	parseJSONResponse(t, resp.Body, &echoResp)

	// Verify method is captured
	if echoResp.Method != "GET" {
		t.Errorf("Expected method='GET', got '%s'", echoResp.Method)
	}
}
