//go:build integration
// +build integration

package tlsfingerprint_test

import (
	"strings"
	"testing"
)

// TestHTML tests the /html endpoint returns valid HTML
func TestHTML(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/html", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)

	bodyLower := strings.ToLower(resp.Body)
	if !strings.Contains(bodyLower, "<html") && !strings.Contains(bodyLower, "<!doctype") {
		t.Errorf("Expected HTML response to contain '<html' or '<!DOCTYPE', got: %s", resp.Body[:min(200, len(resp.Body))])
	}
}

// TestXML tests the /xml endpoint returns valid XML
func TestXML(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/xml", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)

	if !strings.Contains(resp.Body, "<?xml") {
		t.Errorf("Expected XML response to contain '<?xml', got: %s", resp.Body[:min(200, len(resp.Body))])
	}
}

// TestJSON tests the /json endpoint returns valid JSON with TLS fields and slideshow
func TestJSON(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/json", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)
	assertTLSFieldsPresent(t, resp.Body)

	if !strings.Contains(resp.Body, "slideshow") {
		t.Errorf("Expected JSON response to contain 'slideshow' field, got: %s", resp.Body[:min(200, len(resp.Body))])
	}
}

// TestRobots tests the /robots.txt endpoint returns robots.txt content
func TestRobots(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/robots.txt", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)

	if !strings.Contains(resp.Body, "User-agent") {
		t.Errorf("Expected robots.txt to contain 'User-agent', got: %s", resp.Body)
	}
}

// TestDeny tests the /deny endpoint returns the expected denial message
func TestDeny(t *testing.T) {
	client := newClient()
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/deny", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)

	if !strings.Contains(resp.Body, "YOU SHOULDN'T BE HERE") {
		t.Errorf("Expected deny response to contain 'YOU SHOULDN'T BE HERE', got: %s", resp.Body)
	}
}
