package cycletls_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// TestBinaryImageUpload tests uploading binary image data using BodyBytes
func TestBinaryImageUpload(t *testing.T) {
	client := cycletls.Init()
	defer client.Close()

	// Read a test image file
	imageData, err := os.ReadFile("../../../tests/images/test.jpeg")
	if err != nil {
		t.Skip("Test image not found, skipping binary upload test")
		return
	}

	// Upload using BodyBytes
	response, err := client.Do("https://httpbin.org/post", cycletls.Options{
		BodyBytes: imageData,
		Headers: map[string]string{
			"Content-Type": "image/jpeg",
		},
	}, "POST")
	if err != nil {
		t.Fatal("Request Failed: ", err)
	}

	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}

	// Parse response to verify data was received
	var respData map[string]interface{}
	err = json.Unmarshal([]byte(response.Body), &respData)
	if err != nil {
		t.Fatal("Failed to parse response: ", err)
	}

	// httpbin.org returns the data as base64 in the "data" field
	if data, ok := respData["data"].(string); !ok || data == "" {
		t.Fatal("Expected non-empty data field in response")
	}
}

// TestBinaryImageDownload tests downloading binary image data and accessing via BodyBytes
func TestBinaryImageDownload(t *testing.T) {
	client := cycletls.Init()
	defer client.Close()

	// Download an image
	response, err := client.Do("https://httpbin.org/image/jpeg", cycletls.Options{
		Headers: map[string]string{
			"Accept": "image/jpeg",
		},
	}, "GET")
	if err != nil {
		t.Fatal("Request Failed: ", err)
	}

	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}

	// Verify we got binary data in BodyBytes
	if len(response.BodyBytes) == 0 {
		t.Fatal("Expected non-empty BodyBytes for image response")
	}

	// Verify it's a valid JPEG (JPEG files start with 0xFF 0xD8)
	if len(response.BodyBytes) < 2 || response.BodyBytes[0] != 0xFF || response.BodyBytes[1] != 0xD8 {
		t.Fatal("Downloaded data doesn't appear to be a valid JPEG")
	}

	// Verify Body string is also populated (for backward compatibility)
	if len(response.Body) == 0 {
		t.Fatal("Expected non-empty Body string for backward compatibility")
	}

	// Verify they contain the same data
	if !bytes.Equal(response.BodyBytes, []byte(response.Body)) {
		t.Fatal("BodyBytes and Body should contain the same data")
	}
}

// TestMixedMultipartWithBinary tests multipart form data with binary file using BodyBytes
func TestMixedMultipartWithBinary(t *testing.T) {
	client := cycletls.Init()
	defer client.Close()

	// Create a simple test binary data
	binaryData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10} // JPEG header

	// Create multipart form data manually
	var body bytes.Buffer
	writer := io.MultiWriter(&body)

	boundary := "----WebKitFormBoundary7MA4YWxkTrZu0gW"

	// Write text field
	writer.Write([]byte("------WebKitFormBoundary7MA4YWxkTrZu0gW\r\n"))
	writer.Write([]byte("Content-Disposition: form-data; name=\"field1\"\r\n\r\n"))
	writer.Write([]byte("value1\r\n"))

	// Write binary file field
	writer.Write([]byte("------WebKitFormBoundary7MA4YWxkTrZu0gW\r\n"))
	writer.Write([]byte("Content-Disposition: form-data; name=\"file\"; filename=\"test.jpg\"\r\n"))
	writer.Write([]byte("Content-Type: image/jpeg\r\n\r\n"))
	writer.Write(binaryData)
	writer.Write([]byte("\r\n"))

	// Write closing boundary
	writer.Write([]byte("------WebKitFormBoundary7MA4YWxkTrZu0gW--\r\n"))

	// Send using BodyBytes
	response, err := client.Do("https://httpbin.org/post", cycletls.Options{
		BodyBytes: body.Bytes(),
		Headers: map[string]string{
			"Content-Type": "multipart/form-data; boundary=" + boundary,
		},
	}, "POST")
	if err != nil {
		t.Fatal("Request Failed: ", err)
	}

	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}

	// Parse response
	var respData map[string]interface{}
	err = json.Unmarshal([]byte(response.Body), &respData)
	if err != nil {
		t.Fatal("Failed to parse response: ", err)
	}

	// Verify the form data was received
	if form, ok := respData["form"].(map[string]interface{}); ok {
		if form["field1"] != "value1" {
			t.Fatal("Expected field1 to be 'value1'")
		}
	} else {
		t.Fatal("Expected form data in response")
	}

	// Verify file was received
	if files, ok := respData["files"].(map[string]interface{}); ok {
		if _, hasFile := files["file"]; !hasFile {
			t.Fatal("Expected file in response files")
		}
	} else {
		t.Fatal("Expected files in response")
	}
}

// TestBinaryDataPreservation tests that binary data is preserved correctly without corruption
func TestBinaryDataPreservation(t *testing.T) {
	client := cycletls.Init()
	defer client.Close()

	// Create test binary data with various byte values
	testData := make([]byte, 256)
	for i := range testData {
		testData[i] = byte(i)
	}

	// Upload the binary data
	response, err := client.Do("https://httpbin.org/post", cycletls.Options{
		BodyBytes: testData,
		Headers: map[string]string{
			"Content-Type": "application/octet-stream",
		},
	}, "POST")
	if err != nil {
		t.Fatal("Request Failed: ", err)
	}

	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}

	// httpbin.org should echo back our data
	var respData map[string]interface{}
	err = json.Unmarshal([]byte(response.Body), &respData)
	if err != nil {
		t.Fatal("Failed to parse response: ", err)
	}

	// Verify data field exists
	if _, ok := respData["data"].(string); !ok {
		t.Fatal("Expected data field in response")
	}
}

// TestIssue297BinaryCorruptionFix specifically tests the fix for GitHub issue #297
// where binary data was corrupted due to UTF-8 string conversion
func TestIssue297BinaryCorruptionFix(t *testing.T) {
	client := cycletls.Init()
	defer client.Close()

	// Create binary data with sequences that would corrupt in UTF-8 encoding
	// These are the exact problematic scenarios mentioned in issue #297
	problematicData := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, // JPEG header
		0x80, 0x81, 0x82, 0x83, // Invalid UTF-8 sequences that would be replaced
		0x00, 0x01, 0x02, 0x03, // Null bytes and control characters
		0xFE, 0xFF, 0xC0, 0xC1, // More problematic bytes
		0xEF, 0xBF, 0xBD, // UTF-8 replacement character sequence (U+FFFD)
		0xF0, 0x90, 0x8D, // Incomplete 4-byte UTF-8 sequence
	}

	// Upload using BodyBytes field (the fix for issue #297)
	response, err := client.Do("https://httpbin.org/post", cycletls.Options{
		BodyBytes: problematicData,
		Headers: map[string]string{
			"Content-Type": "application/octet-stream",
		},
	}, "POST")

	if err != nil {
		t.Fatal("Request Failed: ", err)
	}

	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}

	// Parse response and verify the data was received without corruption
	var respData map[string]interface{}
	err = json.Unmarshal([]byte(response.Body), &respData)
	if err != nil {
		t.Fatal("Failed to parse response: ", err)
	}

	// httpbin.org returns data as data URI format, extract base64 and decode
	if dataField, ok := respData["data"].(string); ok && dataField != "" {
		// Extract base64 data from data URI format: "data:application/octet-stream;base64,AAH/gH8="
		base64Data := dataField
		if commaIndex := strings.Index(dataField, ","); commaIndex != -1 {
			base64Data = dataField[commaIndex+1:]
		}
		decodedData, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			t.Fatal("Failed to decode base64 data: ", err)
		}

		// Verify the data is identical to what we sent (no corruption)
		if !bytes.Equal(decodedData, problematicData) {
			t.Fatal("Binary data was corrupted during transmission")
		}

	} else {
		t.Fatal("Expected non-empty data field in response")
	}
}

// TestAllPossibleByteValues tests that all byte values (0-255) are preserved
// This is a comprehensive test for the UTF-8 corruption issue
func TestAllPossibleByteValues(t *testing.T) {
	client := cycletls.Init()
	defer client.Close()

	// Create data with all possible byte values
	allBytesData := make([]byte, 256)
	for i := range allBytesData {
		allBytesData[i] = byte(i)
	}

	// Upload using BodyBytes
	response, err := client.Do("https://httpbin.org/post", cycletls.Options{
		BodyBytes: allBytesData,
		Headers: map[string]string{
			"Content-Type": "application/octet-stream",
		},
	}, "POST")

	if err != nil {
		t.Fatal("Request Failed: ", err)
	}

	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}

	// Parse and verify all byte values are preserved
	var respData map[string]interface{}
	err = json.Unmarshal([]byte(response.Body), &respData)
	if err != nil {
		t.Fatal("Failed to parse response: ", err)
	}

	if dataField, ok := respData["data"].(string); ok && dataField != "" {
		// Extract base64 data from data URI format: "data:application/octet-stream;base64,AAH/gH8="
		base64Data := dataField
		if commaIndex := strings.Index(dataField, ","); commaIndex != -1 {
			base64Data = dataField[commaIndex+1:]
		}
		decodedData, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			t.Fatal("Failed to decode base64 data: ", err)
		}

		// Verify all 256 byte values are preserved
		if len(decodedData) != 256 {
			t.Fatalf("Expected 256 bytes, got %d", len(decodedData))
		}

		for i := 0; i < 256; i++ {
			if decodedData[i] != byte(i) {
				t.Fatalf("Byte at position %d corrupted: expected %d, got %d", i, i, decodedData[i])
			}
		}

	} else {
		t.Fatal("Expected non-empty data field in response")
	}
}

// TestBinaryResponseIntegrity tests that BodyBytes field in responses preserves binary data
func TestBinaryResponseIntegrity(t *testing.T) {
	client := cycletls.Init()
	defer client.Close()

	// Download a binary image
	response, err := client.Do("https://httpbin.org/image/png", cycletls.Options{}, "GET")
	if err != nil {
		t.Fatal("Request Failed: ", err)
	}

	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}

	// Verify BodyBytes contains the binary data
	if len(response.BodyBytes) == 0 {
		t.Fatal("Expected non-empty BodyBytes for PNG response")
	}

	// Verify it's a valid PNG (PNG files start with specific signature)
	pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if len(response.BodyBytes) < len(pngSignature) {
		t.Fatal("PNG data too short to contain signature")
	}

	if !bytes.Equal(response.BodyBytes[:len(pngSignature)], pngSignature) {
		t.Fatal("Invalid PNG signature - possible binary corruption")
	}

	// Verify Body string and BodyBytes contain the same data
	if !bytes.Equal(response.BodyBytes, []byte(response.Body)) {
		t.Fatal("BodyBytes and Body string should contain identical binary data")
	}

}

// TestLargeBinaryDataHandling tests handling of large binary files without corruption
func TestLargeBinaryDataHandling(t *testing.T) {
	client := cycletls.Init()
	defer client.Close()

	// Create a large binary file with repeating problematic UTF-8 sequences
	pattern := []byte{0xFF, 0x00, 0x80, 0x81, 0xC0, 0xC1, 0xFE, 0xFF}
	repetitions := 5000 // 40KB of problematic binary data
	largeData := bytes.Repeat(pattern, repetitions)

	// Upload using BodyBytes
	response, err := client.Do("https://httpbin.org/post", cycletls.Options{
		BodyBytes: largeData,
		Headers: map[string]string{
			"Content-Type": "application/octet-stream",
		},
	}, "POST")

	if err != nil {
		t.Fatal("Request Failed: ", err)
	}

	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}

	// Parse and verify data integrity
	var respData map[string]interface{}
	err = json.Unmarshal([]byte(response.Body), &respData)
	if err != nil {
		t.Fatal("Failed to parse response: ", err)
	}

	if dataField, ok := respData["data"].(string); ok && dataField != "" {
		// Extract base64 data from data URI format: "data:application/octet-stream;base64,AAH/gH8="
		base64Data := dataField
		if commaIndex := strings.Index(dataField, ","); commaIndex != -1 {
			base64Data = dataField[commaIndex+1:]
		}
		decodedData, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			t.Fatal("Failed to decode base64 data: ", err)
		}

		if !bytes.Equal(decodedData, largeData) {
			t.Fatal("Large binary data was corrupted during transmission")
		}

	} else {
		t.Fatal("Expected non-empty data field in response")
	}
}

// TestDebugBinaryResponse provides detailed debugging information for binary data handling
// This test helps verify the implementation details and provides extensive logging
func TestDebugBinaryResponse(t *testing.T) {
	client := cycletls.Init()
	defer client.Close()

	// Create some test binary data with problematic UTF-8 sequences
	testData := []byte{0x00, 0x01, 0xFF, 0x80, 0x7F, 0xFE, 0xC0, 0xC1}

	// Upload using BodyBytes field
	response, err := client.Do("https://httpbin.org/post", cycletls.Options{
		BodyBytes: testData,
		Headers: map[string]string{
			"Content-Type": "application/octet-stream",
		},
	}, "POST")

	if err != nil {
		t.Fatal("Request Failed: ", err)
	}

	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}

	// Parse response to see structure and verify binary data handling
	var respData map[string]interface{}
	err = json.Unmarshal([]byte(response.Body), &respData)
	if err != nil {
		t.Fatal("Failed to parse response: ", err)
	}

	// Verify that binary data was properly handled
	if data, ok := respData["data"].(string); ok && data != "" {

		// httpbin.org returns binary data in data URI format or as base64
		var decodedData []byte
		var err error

		if strings.HasPrefix(data, "data:") {
			// Extract base64 from data URI: "data:application/octet-stream;base64,AAH/gH8="
			if commaIndex := strings.Index(data, ","); commaIndex != -1 {
				base64Data := data[commaIndex+1:]
				decodedData, err = base64.StdEncoding.DecodeString(base64Data)
			} else {
				t.Error("Data URI format detected but no comma separator found")
				return
			}
		} else {
			// Try direct base64 decoding
			decodedData, err = base64.StdEncoding.DecodeString(data)
		}

		if err != nil {
			t.Fatal("Failed to decode base64 data: ", err)
		}

		// Verify data integrity - the decoded data should match our original test data
		if !bytes.Equal(testData, decodedData) {
			t.Errorf("Binary data corruption detected:\nOriginal: %v\nReceived: %v", testData, decodedData)

			// Detailed byte-by-byte comparison for debugging
			minLen := min(len(testData), len(decodedData))
			for i := 0; i < minLen; i++ {
				if testData[i] != decodedData[i] {
					t.Errorf("First difference at byte %d: expected 0x%02X, got 0x%02X", i, testData[i], decodedData[i])
					break
				}
			}
			if len(testData) != len(decodedData) {
				t.Errorf("Length mismatch: expected %d bytes, got %d bytes", len(testData), len(decodedData))
			}
		}

	} else {
		t.Error("No data field found in response or data is empty")
	}

	// Verify BodyBytes field in response contains binary data
	if len(response.BodyBytes) > 0 {

		// Verify BodyBytes matches Body string for this text response
		if !bytes.Equal(response.BodyBytes, []byte(response.Body)) {
			t.Error("BodyBytes and Body string differ unexpectedly")
		}
	} else {
		t.Error("BodyBytes field is empty in response")
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
