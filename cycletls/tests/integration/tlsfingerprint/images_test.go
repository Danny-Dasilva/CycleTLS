//go:build integration
// +build integration

package tlsfingerprint_test

import (
	"bytes"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// TestImageJPEG tests the /image/jpeg endpoint
func TestImageJPEG(t *testing.T) {
	client := cycletls.Init(cycletls.WithRawBytes())
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := doRequestWithRetry(t, client, TestServerURL+"/image/jpeg", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)

	// JPEG magic bytes: 0xFF, 0xD8
	if len(resp.BodyBytes) < 2 {
		t.Fatalf("Response body too short for JPEG, got %d bytes", len(resp.BodyBytes))
	}

	if resp.BodyBytes[0] != 0xFF || resp.BodyBytes[1] != 0xD8 {
		t.Errorf("Expected JPEG magic bytes (0xFF 0xD8), got (0x%02X 0x%02X)", resp.BodyBytes[0], resp.BodyBytes[1])
	}
}

// TestImagePNG tests the /image/png endpoint
func TestImagePNG(t *testing.T) {
	client := cycletls.Init(cycletls.WithRawBytes())
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/image/png", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)

	// PNG signature: 0x89, 0x50, 0x4E, 0x47 (0x89 P N G)
	pngSignature := []byte{0x89, 0x50, 0x4E, 0x47}
	if len(resp.BodyBytes) < 4 {
		t.Fatalf("Response body too short for PNG, got %d bytes", len(resp.BodyBytes))
	}

	if !bytes.HasPrefix(resp.BodyBytes, pngSignature) {
		t.Errorf("Expected PNG signature (0x89 0x50 0x4E 0x47), got (0x%02X 0x%02X 0x%02X 0x%02X)",
			resp.BodyBytes[0], resp.BodyBytes[1], resp.BodyBytes[2], resp.BodyBytes[3])
	}
}

// TestImageSVG tests the /image/svg endpoint
func TestImageSVG(t *testing.T) {
	client := cycletls.Init(cycletls.WithRawBytes())
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/image/svg", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)

	// SVG should contain "<svg" tag
	if !bytes.Contains(resp.BodyBytes, []byte("<svg")) {
		t.Errorf("Expected SVG response to contain '<svg', got: %s", string(resp.BodyBytes[:min(100, len(resp.BodyBytes))]))
	}
}

// TestImageGIF tests the /image/gif endpoint
func TestImageGIF(t *testing.T) {
	client := cycletls.Init(cycletls.WithRawBytes())
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := doRequestWithRetry(t, client, TestServerURL+"/image/gif", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)

	// GIF starts with "GIF89a" or "GIF87a"
	if len(resp.BodyBytes) < 6 {
		t.Fatalf("Response body too short for GIF, got %d bytes", len(resp.BodyBytes))
	}

	gif89a := []byte("GIF89a")
	gif87a := []byte("GIF87a")
	if !bytes.HasPrefix(resp.BodyBytes, gif89a) && !bytes.HasPrefix(resp.BodyBytes, gif87a) {
		t.Errorf("Expected GIF header (GIF89a or GIF87a), got: %s", string(resp.BodyBytes[:6]))
	}
}

// TestImageWebP tests the /image/webp endpoint
func TestImageWebP(t *testing.T) {
	client := cycletls.Init(cycletls.WithRawBytes())
	defer client.Close()

	opts := getDefaultOptions()
	resp, err := client.Do(TestServerURL+"/image/webp", opts, "GET")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	assertStatusCode(t, 200, resp.Status)

	// WebP starts with "RIFF" (0x52, 0x49, 0x46, 0x46)
	riffSignature := []byte{0x52, 0x49, 0x46, 0x46}
	if len(resp.BodyBytes) < 4 {
		t.Fatalf("Response body too short for WebP, got %d bytes", len(resp.BodyBytes))
	}

	if !bytes.HasPrefix(resp.BodyBytes, riffSignature) {
		t.Errorf("Expected WebP RIFF signature (0x52 0x49 0x46 0x46), got (0x%02X 0x%02X 0x%02X 0x%02X)",
			resp.BodyBytes[0], resp.BodyBytes[1], resp.BodyBytes[2], resp.BodyBytes[3])
	}
}
