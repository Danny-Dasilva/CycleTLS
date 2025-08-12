package cycletls_test

import (
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestCookieJar(t *testing.T) {
	client := cycletls.Init()
	defer client.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	// First request to set cookies
	firstResponse, err := client.Do("https://httpbin.org/cookies/set?a=1&b=2&c=3", cycletls.Options{
		Body:            "",
		Ja3:             "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		DisableRedirect: true,
	}, "GET")
	if err != nil {
		t.Fatal(err)
	}

	// Parse the URL and set cookies in the jar
	firstURL, _ := url.Parse(firstResponse.FinalUrl)
	jar.SetCookies(firstURL, firstResponse.Cookies)

	// Second request to verify cookies
	secondResponse, err := client.Do("https://httpbin.org/cookies", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		Headers: map[string]string{
			"Cookie": getHeadersFromJar(jar, firstURL),
		},
	}, "GET")
	if err != nil {
		t.Fatal(err)
	}

	// Check if the response contains the expected cookies
	expectedCookies := `"a": "1"`
	if !strings.Contains(secondResponse.Body, expectedCookies) {
		t.Errorf("Expected cookie 'a=1' not found in response body: %s", secondResponse.Body)
	}

	expectedCookies = `"b": "2"`
	if !strings.Contains(secondResponse.Body, expectedCookies) {
		t.Errorf("Expected cookie 'b=2' not found in response body: %s", secondResponse.Body)
	}

	expectedCookies = `"c": "3"`
	if !strings.Contains(secondResponse.Body, expectedCookies) {
		t.Errorf("Expected cookie 'c=3' not found in response body: %s", secondResponse.Body)
	}

}

func getHeadersFromJar(jar *cookiejar.Jar, url *url.URL) string {
	cookies := jar.Cookies(url)
	var cookieStrs []string
	for _, cookie := range cookies {
		cookieStrs = append(cookieStrs, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(cookieStrs, "; ")
}

func TestCookieJarMultipleDomains(t *testing.T) {
	client := cycletls.Init()
	defer client.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Set cookies for httpbin.org
	httpbinResponse, err := client.Do("https://httpbin.org/cookies/set?httpbin=test", cycletls.Options{
		Body:            "",
		Ja3:             "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		DisableRedirect: true,
	}, "GET")
	if err != nil {
		t.Fatal(err)
	}

	httpbinURL, _ := url.Parse(httpbinResponse.FinalUrl)
	jar.SetCookies(httpbinURL, httpbinResponse.Cookies)

	// Verify cookies are sent to the correct domain
	verifyResponse, err := client.Do("https://httpbin.org/cookies", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		Headers: map[string]string{
			"Cookie": getHeadersFromJar(jar, httpbinURL),
		},
	}, "GET")
	if err != nil {
		t.Fatal(err)
	}

	// Check if httpbin cookie is present
	if !strings.Contains(verifyResponse.Body, `"httpbin": "test"`) {
		t.Errorf("Expected cookie 'httpbin=test' not found in response body: %s", verifyResponse.Body)
	}
}

func TestCookieJarPersistence(t *testing.T) {
	client := cycletls.Init()
	defer client.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	// First request - set initial cookies
	firstResponse, err := client.Do("https://httpbin.org/cookies/set?session=12345", cycletls.Options{
		Body:            "",
		Ja3:             "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		DisableRedirect: true,
	}, "GET")
	if err != nil {
		t.Fatal(err)
	}

	firstURL, _ := url.Parse(firstResponse.FinalUrl)
	jar.SetCookies(firstURL, firstResponse.Cookies)

	// Second request - add more cookies
	secondResponse, err := client.Do("https://httpbin.org/cookies/set?user=alice", cycletls.Options{
		Body:            "",
		Ja3:             "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		DisableRedirect: true,
		Headers: map[string]string{
			"Cookie": getHeadersFromJar(jar, firstURL),
		},
	}, "GET")
	if err != nil {
		t.Fatal(err)
	}

	secondURL, _ := url.Parse(secondResponse.FinalUrl)
	jar.SetCookies(secondURL, secondResponse.Cookies)

	// Third request - verify all cookies are maintained
	finalResponse, err := client.Do("https://httpbin.org/cookies", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		Headers: map[string]string{
			"Cookie": getHeadersFromJar(jar, secondURL),
		},
	}, "GET")
	if err != nil {
		t.Fatal(err)
	}

	// Verify both cookies are present
	if !strings.Contains(finalResponse.Body, `"session": "12345"`) {
		t.Errorf("Expected cookie 'session=12345' not found in response body: %s", finalResponse.Body)
	}
	if !strings.Contains(finalResponse.Body, `"user": "alice"`) {
		t.Errorf("Expected cookie 'user=alice' not found in response body: %s", finalResponse.Body)
	}

}
