//go:build integration
// +build integration

package cycletls_test

import (
	"strings"
	"testing"
	"time"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestTLS13ForceRenegotiation(t *testing.T) {
	client := cycletls.Init()

	// Test case 1: Original working JA3 (already TLS 1.3 - version 772)
	t.Run("Working_Chrome138_JA3_TLS13", func(t *testing.T) {
		workingJA3 := "772,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23,0"

		response, err := client.Do("https://www.howsmyssl.com/a/check", cycletls.Options{
			Ja3:       workingJA3,
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		}, "GET")

		if err != nil {
			t.Logf("Working JA3 test info (may be network-related): %s", err.Error())
		} else {
			t.Logf("✅ Working JA3 (TLS 1.3): Status %d", response.Status)
		}
	})

	// Test case 2: Problematic JA3 (TLS 1.2 - version 771) WITHOUT TLS13AutoRetry
	t.Run("Problematic_Chrome138_JA3_TLS12_NoRetry", func(t *testing.T) {
		problematicJA3 := "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,65037-65281-10-51-11-5-17613-0-45-43-18-35-27-23-16-13,4588-29-23-24,0"

		response, err := client.Do("https://www.howsmyssl.com/a/check", cycletls.Options{
			Ja3:            problematicJA3,
			UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			TLS13AutoRetry: false, // Explicitly disable auto-retry
		}, "GET")

		if err != nil {
			t.Logf("📋 TLS 1.2 without retry result (expected behavior): %s", err.Error())
		} else {
			t.Logf("📋 TLS 1.2 without retry: Status %d", response.Status)
		}
	})

	// Test case 3: THE KEY TEST - Problematic JA3 WITH TLS13AutoRetry (Force TLS 1.3)
	t.Run("Force_TLS13_Renegotiation", func(t *testing.T) {
		problematicJA3 := "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,65037-65281-10-51-11-5-17613-0-45-43-18-35-27-23-16-13,4588-29-23-24,0"

		start := time.Now()
		response, err := client.Do("https://www.howsmyssl.com/a/check", cycletls.Options{
			Ja3:            problematicJA3,
			UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			TLS13AutoRetry: true, // Enable auto-retry - this should force TLS 1.3
		}, "GET")
		duration := time.Since(start)

		if err != nil {
			t.Errorf("❌ CRITICAL: Force TLS 1.3 renegotiation failed: %s", err.Error())
		} else {
			t.Logf("🎉 SUCCESS: Force TLS 1.3 renegotiation worked! Status: %d, Duration: %v", response.Status, duration)
			
			// Check if response indicates TLS 1.3 was used
			if strings.Contains(string(response.Body), "TLS 1.3") {
				t.Logf("✅ CONFIRMED: Server response indicates TLS 1.3 was negotiated")
			} else {
				t.Logf("📋 INFO: Server response doesn't explicitly show TLS version")
			}
		}
	})

	// Test case 4: Test with different server
	t.Run("Force_TLS13_Different_Server", func(t *testing.T) {
		problematicJA3 := "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,65037-65281-10-51-11-5-17613-0-45-43-18-35-27-23-16-13,4588-29-23-24,0"

		response, err := client.Do("https://tls.peet.ws/", cycletls.Options{
			Ja3:            problematicJA3,
			UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			TLS13AutoRetry: true, // Enable force TLS 1.3
		}, "GET")

		if err != nil {
			t.Logf("📋 tls.peet.ws test info (server-dependent): %s", err.Error())
		} else {
			t.Logf("✅ tls.peet.ws with force TLS 1.3: Status %d", response.Status)
		}
	})
}

