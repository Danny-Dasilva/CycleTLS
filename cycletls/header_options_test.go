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

	if got, want := options.Headers["accept"], "text/plain"; got != want {
		t.Fatalf("Headers[accept] = %q, want %q", got, want)
	}
	if got := options.HeaderValues["accept"]; !reflect.DeepEqual(got, []string{"text/plain"}) {
		t.Fatalf("HeaderValues[accept] = %#v, want [text/plain]", got)
	}
}

func TestOptionsUnmarshalJSONRejectsInvalidHeaderValue(t *testing.T) {
	var options Options
	err := json.Unmarshal([]byte(`{
		"url": "https://example.com",
		"headers": {"x-bad": 42}
	}`), &options)
	if err == nil {
		t.Fatal("expected error for non-string, non-array header value")
	}
}

func TestOptionsUnmarshalJSONNilHeaders(t *testing.T) {
	var options Options
	if err := json.Unmarshal([]byte(`{"url": "https://example.com"}`), &options); err != nil {
		t.Fatalf("UnmarshalJSON returned error: %v", err)
	}
	if options.Headers != nil {
		t.Fatalf("Headers = %#v, want nil", options.Headers)
	}
	if options.HeaderValues != nil {
		t.Fatalf("HeaderValues = %#v, want nil", options.HeaderValues)
	}
}

func TestSetRequestHeadersPreservesCasingAndMultipleValues(t *testing.T) {
	headers := make(http.Header)
	options := Options{
		HeaderValues: map[string][]string{
			"x-CUSTOM-case": {"one", "two"},
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

func TestSetRequestHeadersSkipsContentLength(t *testing.T) {
	headers := make(http.Header)
	options := Options{
		Headers: map[string]string{
			"content-length": "42",
			"x-keep":         "yes",
		},
	}

	setRequestHeaders(headers, options, true)

	if _, ok := headers["content-length"]; ok {
		t.Fatalf("content-length should have been skipped: %#v", headers)
	}
	if got := headers["x-keep"]; !reflect.DeepEqual(got, []string{"yes"}) {
		t.Fatalf("headers[x-keep] = %#v, want [yes]", got)
	}
}

func TestHasRequestHeaderIsCaseInsensitive(t *testing.T) {
	options := Options{
		Headers: map[string]string{"hOsT": "example.com"},
	}
	if !hasRequestHeader(options, "Host") {
		t.Fatal("hasRequestHeader should match case-insensitively")
	}
	if hasRequestHeader(options, "Accept") {
		t.Fatal("hasRequestHeader matched a header that is not present")
	}
}

func TestSetHeaderExactReplacesOtherCasings(t *testing.T) {
	headers := http.Header{
		"User-Agent": []string{"from-headers"},
	}

	setHeaderExact(headers, "user-agent", "from-options")

	if _, ok := headers["User-Agent"]; ok {
		t.Fatalf("canonical User-Agent entry should have been removed: %#v", headers)
	}
	if got := headers["user-agent"]; !reflect.DeepEqual(got, []string{"from-options"}) {
		t.Fatalf("headers[user-agent] = %#v, want [from-options]", got)
	}
}
