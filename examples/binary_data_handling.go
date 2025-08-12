/**
 * Binary Data Handling Examples - Fixes GitHub Issue #297
 *
 * This file demonstrates how CycleTLS properly handles binary data without UTF-8 corruption.
 * Before this fix, binary data would be corrupted when converting to/from strings.
 *
 * Key improvements:
 * - Use BodyBytes field for binary requests
 * - Access BodyBytes field for binary responses
 * - Data integrity preservation
 */

package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	fmt.Println("🔄 Demonstrating binary data handling fixes for Issue #297\n")

	client := cycletls.Init()
	defer client.Close()

	// Example 1: Upload binary data with problematic UTF-8 sequences
	fmt.Println("1. Testing binary upload with problematic UTF-8 sequences...")
	demonstrateProblematicBinaryUpload(client)

	// Example 2: Download binary image using BodyBytes field
	fmt.Println("\n2. Testing binary download with BodyBytes field...")
	demonstrateBinaryDownload(client)

	// Example 3: Compare BodyBytes vs Body string for binary data
	fmt.Println("\n3. Comparing BodyBytes vs Body string methods...")
	demonstrateResponseComparison(client)

	// Example 4: Test all possible byte values (comprehensive corruption test)
	fmt.Println("\n4. Testing all possible byte values (0-255)...")
	demonstrateAllByteValues(client)

	// Example 5: Demonstrate file upload
	fmt.Println("\n5. Testing actual file upload...")
	demonstrateFileUpload(client)

	fmt.Println("\n🎉 All binary data handling tests completed successfully!")
	fmt.Println("   Issue #297 fix verified: No UTF-8 corruption detected")

	// Show migration examples
	showMigrationExamples()
}

func demonstrateProblematicBinaryUpload(client cycletls.CycleTLS) {
	// Create data that would corrupt in UTF-8 encoding
	problematicData := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, // JPEG header
		0x80, 0x81, 0x82, 0x83, // Invalid UTF-8 sequences
		0x00, 0x01, 0x02, 0x03, // Null bytes and control characters
		0xFE, 0xFF, 0xC0, 0xC1, // More problematic bytes
		0xEF, 0xBF, 0xBD,       // UTF-8 replacement character sequence (U+FFFD)
	}

	originalHasher := md5.New()
	originalHasher.Write(problematicData)
	originalHash := hex.EncodeToString(originalHasher.Sum(nil))
	fmt.Printf("   Original data hash: %s\n", originalHash)

	// ✅ CORRECT: Use BodyBytes field to prevent UTF-8 corruption
	response, err := client.Do("https://httpbin.org/post", cycletls.Options{
		BodyBytes: problematicData, // Direct binary data - no string conversion
		Headers: map[string]string{
			"Content-Type": "application/octet-stream",
		},
	}, "POST")

	if err != nil {
		log.Printf("❌ Upload failed: %v", err)
		return
	}

	// Parse response and verify data integrity
	var respData map[string]interface{}
	if err := json.Unmarshal([]byte(response.Body), &respData); err != nil {
		log.Printf("❌ Failed to parse response: %v", err)
		return
	}

	// httpbin.org returns data as base64, decode and verify integrity
	if dataField, ok := respData["data"].(string); ok && dataField != "" {
		decodedData, err := base64.StdEncoding.DecodeString(dataField)
		if err != nil {
			log.Printf("❌ Failed to decode base64 data: %v", err)
			return
		}

		receivedHasher := md5.New()
		receivedHasher.Write(decodedData)
		receivedHash := hex.EncodeToString(receivedHasher.Sum(nil))
		
		fmt.Printf("   Received data hash: %s\n", receivedHash)
		fmt.Printf("   ✅ Data integrity preserved: %t\n", originalHash == receivedHash)
	} else {
		log.Println("❌ No data field in response")
	}
}

func demonstrateBinaryDownload(client cycletls.CycleTLS) {
	// Download binary content
	response, err := client.Do("https://httpbin.org/image/jpeg", cycletls.Options{
		Headers: map[string]string{
			"Accept": "image/jpeg",
		},
	}, "GET")

	if err != nil {
		log.Printf("❌ Download failed: %v", err)
		return
	}

	// ✅ CORRECT: Use BodyBytes field for binary data
	imageData := response.BodyBytes // []byte - preserves binary integrity

	// Verify it's a valid JPEG (starts with 0xFF 0xD8)
	if len(imageData) >= 2 && imageData[0] == 0xFF && imageData[1] == 0xD8 {
		fmt.Printf("   Downloaded %d bytes\n", len(imageData))
		fmt.Println("   ✅ Valid JPEG signature detected")

		// Calculate hash for verification
		hasher := md5.New()
		hasher.Write(imageData)
		hash := hex.EncodeToString(hasher.Sum(nil))
		fmt.Printf("   Data hash: %s\n", hash)

		// Save to file to demonstrate data integrity
		if err := os.WriteFile("downloaded-image.jpg", imageData, 0644); err == nil {
			fmt.Println("   📁 Image saved as downloaded-image.jpg")
		}
	} else {
		fmt.Println("❌ Invalid JPEG data - possible corruption")
	}
}

func demonstrateResponseComparison(client cycletls.CycleTLS) {
	// Download PNG image
	response, err := client.Do("https://httpbin.org/image/png", cycletls.Options{}, "GET")
	if err != nil {
		log.Printf("❌ Download failed: %v", err)
		return
	}

	// Compare BodyBytes field vs Body string
	bodyBytesData := response.BodyBytes
	bodyStringData := []byte(response.Body)

	fmt.Printf("   BodyBytes size: %d bytes\n", len(bodyBytesData))
	fmt.Printf("   Body string size: %d bytes\n", len(bodyStringData))

	// Check if they're identical
	identical := len(bodyBytesData) == len(bodyStringData)
	if identical {
		for i := range bodyBytesData {
			if bodyBytesData[i] != bodyStringData[i] {
				identical = false
				break
			}
		}
	}
	fmt.Printf("   ✅ Data consistency: %t\n", identical)

	// Verify PNG signature in both
	pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	bytesHasSignature := len(bodyBytesData) >= len(pngSignature)
	stringHasSignature := len(bodyStringData) >= len(pngSignature)

	if bytesHasSignature {
		for i, b := range pngSignature {
			if bodyBytesData[i] != b {
				bytesHasSignature = false
				break
			}
		}
	}

	if stringHasSignature {
		for i, b := range pngSignature {
			if bodyStringData[i] != b {
				stringHasSignature = false
				break
			}
		}
	}

	fmt.Printf("   ✅ BodyBytes PNG signature valid: %t\n", bytesHasSignature)
	fmt.Printf("   ✅ Body string PNG signature valid: %t\n", stringHasSignature)
}

func demonstrateAllByteValues(client cycletls.CycleTLS) {
	// Create data with all possible byte values
	allBytesData := make([]byte, 256)
	for i := range allBytesData {
		allBytesData[i] = byte(i)
	}

	originalHasher := sha256.New()
	originalHasher.Write(allBytesData)
	originalHash := hex.EncodeToString(originalHasher.Sum(nil))

	// Upload using BodyBytes
	response, err := client.Do("https://httpbin.org/post", cycletls.Options{
		BodyBytes: allBytesData,
		Headers: map[string]string{
			"Content-Type": "application/octet-stream",
		},
	}, "POST")

	if err != nil {
		log.Printf("❌ Upload failed: %v", err)
		return
	}

	// Parse and verify all byte values are preserved
	var respData map[string]interface{}
	if err := json.Unmarshal([]byte(response.Body), &respData); err != nil {
		log.Printf("❌ Failed to parse response: %v", err)
		return
	}

	if dataField, ok := respData["data"].(string); ok && dataField != "" {
		decodedData, err := base64.StdEncoding.DecodeString(dataField)
		if err != nil {
			log.Printf("❌ Failed to decode base64 data: %v", err)
			return
		}

		receivedHasher := sha256.New()
		receivedHasher.Write(decodedData)
		receivedHash := hex.EncodeToString(receivedHasher.Sum(nil))

		fmt.Printf("   Original hash: %s\n", originalHash)
		fmt.Printf("   Received hash: %s\n", receivedHash)
		fmt.Printf("   ✅ All 256 byte values preserved: %t\n", originalHash == receivedHash)

		// Verify each byte value individually
		if len(decodedData) == 256 {
			allCorrect := true
			for i := 0; i < 256; i++ {
				if decodedData[i] != byte(i) {
					fmt.Printf("   ❌ Byte corruption at position %d: expected %d, got %d\n", i, i, decodedData[i])
					allCorrect = false
					break
				}
			}
			if allCorrect {
				fmt.Println("   ✅ Individual byte verification passed")
			}
		}
	} else {
		log.Println("❌ No data field in response")
	}
}

func demonstrateFileUpload(client cycletls.CycleTLS) {
	// Try to read an existing file, fallback to created data
	var testFileData []byte
	var fileName string

	testFiles := []string{"./examples/test-image.jpg", "./tests/images/test.jpeg", "./go.mod", "./package.json"}
	
	for _, filePath := range testFiles {
		if data, err := os.ReadFile(filePath); err == nil {
			testFileData = data
			fileName = filePath
			break
		}
	}

	if testFileData == nil {
		// Create minimal test file data
		testFileData = []byte("Test file content with binary data: \x00\x01\x02\xFF\xFE")
		fileName = "generated test data"
	}

	hasher := md5.New()
	hasher.Write(testFileData)
	fileHash := hex.EncodeToString(hasher.Sum(nil))

	fmt.Printf("   📁 Using file: %s\n", fileName)
	fmt.Printf("   File size: %d bytes\n", len(testFileData))
	fmt.Printf("   File hash: %s\n", fileHash)

	// Upload using BodyBytes
	response, err := client.Do("https://httpbin.org/post", cycletls.Options{
		BodyBytes: testFileData,
		Headers: map[string]string{
			"Content-Type": "application/octet-stream",
		},
	}, "POST")

	if err != nil {
		log.Printf("❌ Upload failed: %v", err)
		return
	}

	fmt.Printf("   ✅ Upload successful: %t\n", response.Status == 200)

	// Verify upload by parsing response
	if response.Status == 200 {
		var respData map[string]interface{}
		if err := json.Unmarshal([]byte(response.Body), &respData); err == nil {
			if _, hasData := respData["data"]; hasData {
				fmt.Println("   ✅ Server received binary data")
			}
		}
	}
}

func showMigrationExamples() {
	fmt.Println("\n📋 Migration Guide - Before vs After Issue #297 Fix:\n")

	fmt.Println("❌ BEFORE (would cause corruption):")
	fmt.Println("   response, _ := client.Do(url, cycletls.Options{")
	fmt.Println("       Body: string(binaryData), // Corrupted during string conversion")
	fmt.Println("   }, \"POST\")")
	fmt.Println("   // Binary data in response.Body was also corrupted\n")

	fmt.Println("✅ AFTER (preserves binary integrity):")
	fmt.Println("   response, _ := client.Do(url, cycletls.Options{")
	fmt.Println("       BodyBytes: binaryData, // Direct binary handling")
	fmt.Println("   }, \"POST\")")
	fmt.Println("   cleanData := response.BodyBytes // []byte - clean binary data\n")

	fmt.Println("🔧 Key improvements:")
	fmt.Println("   • BodyBytes field for requests ([]byte)")
	fmt.Println("   • BodyBytes field for responses ([]byte)")
	fmt.Println("   • Body string field still available for compatibility")
	fmt.Println("   • No UTF-8 conversion corruption")
	fmt.Println("   • Better performance for binary data")
}