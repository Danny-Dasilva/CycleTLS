package cycletls

import (
	"encoding/json"
	"reflect"
	"testing"

	http "github.com/Danny-Dasilva/fhttp"
)

func TestOptionsUnmarshalJSONSupportsMultiValueHeaders(t *testing.T) {
	var options Options
	err := json.Unmarshal([]byte(`{
		"url": "https://example.com",
		"headers": {
			"x-CUSTOM-case": ["one", "two"],
			"accept": "text/plain"
		}
	}`), &options)
	if err != nil {
		t.Fatalf("UnmarshalJSON returned error: %v", err)
	}

	if got, want := options.Headers["x-CUSTOM-case"], "one"; got != want {
		t.Fatalf("Headers first value = %q, want %q", got, want)
	}

	wantValues := []string{"one", "two"}
	if got := options.HeaderValues["x-CUSTOM-case"]; !reflect.DeepEqual(got, wantValues) {
		t.Fatalf("HeaderValues = %#v, want %#v", got, wantValues)
	}
}

func TestSetRequestHeadersPreservesCasingAndMultipleValues(t *testing.T) {
	headers := make(http.Header)
	options := Options{
		HeaderValues: map[string][]string{
			"x-CUSTOM-case": []string{"one", "two"},
		},
	}

	setRequestHeaders(headers, options, false)

	if _, ok := headers["X-Custom-Case"]; ok {
		t.Fatalf("header key was canonicalized: %#v", headers)
	}

	want := []string{"one", "two"}
	if got := headers["x-CUSTOM-case"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("headers = %#v, want %#v", got, want)
	}
}
