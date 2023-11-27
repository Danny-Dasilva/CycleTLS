package cycletls_test

import (
	"encoding/json"
	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
	"net/http/cookiejar"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestCookieHandling(t *testing.T) {
	client := cycletls.Init()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	// First request to check no cookies
	firstResponse, err := client.Do("https://httpbin.org/cookies", cycletls.Options{Body: "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0"}, "GET")
	if err != nil || len(firstResponse.Cookies) != 0 {
		t.Fatalf("Expected no cookies, got %v", firstResponse.Cookies)
	}

	// Second request to set a single cookie
	secondURL := "https://httpbin.org/cookies/set?freeform=test"
	secondResponse, err := client.Do(secondURL, cycletls.Options{Body: "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0", DisableRedirect: true}, "GET")
	if err != nil || secondResponse.Status != 302 {
		t.Fatalf("Expected status 302, got %d", secondResponse.Status)
	}

	// Add cookies to jar and prepare for the next request
	u, _ := url.Parse(secondURL)
	jar.SetCookies(u, secondResponse.Cookies)
	cookieHeader := getHeadersFromJar(jar, u)

	// Third request to verify the cookie
	thirdResponse, _ := client.Do("https://httpbin.org/cookies", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Headers:   map[string]string{"Cookie": cookieHeader},
	}, "GET")
	var cookieData struct {
		Cookies map[string]string `json:"cookies"`
	}
	if err := json.Unmarshal([]byte(thirdResponse.Body), &cookieData); err != nil {
		t.Fatal(err)
	}

	expectedCookies := map[string]string{"freeform": "test"}
	if !reflect.DeepEqual(cookieData.Cookies, expectedCookies) {
		t.Fatalf("Expected cookies %v, got %v", expectedCookies, cookieData.Cookies)
	}

	// Fourth request to set additional cookies
	fourthURL := "https://httpbin.org/cookies/set?a=1&b=2&c=3"
	fourthResponse, _ := client.Do(fourthURL, cycletls.Options{Body: "",
		Ja3:             "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent:       "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		DisableRedirect: true}, "GET")

	// Add new cookies to jar
	jar.SetCookies(u, fourthResponse.Cookies)
	cookieHeader = getHeadersFromJar(jar, u)

	// Fifth request to verify all cookies
	fifthResponse, err := client.Do("https://httpbin.org/cookies", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Headers:   map[string]string{"Cookie": cookieHeader},
	}, "GET")
	if err != nil {
		t.Fatal(err)
	}

	// Unmarshal the response body to verify cookies
	var cookieData2 struct {
		Cookies map[string]string `json:"cookies"`
	}
	if err := json.Unmarshal([]byte(fifthResponse.Body), &cookieData2); err != nil {
		t.Fatal(err)
	}

	expectedCookies = map[string]string{"a": "1", "b": "2", "c": "3", "freeform": "test"}
	if !reflect.DeepEqual(cookieData2.Cookies, expectedCookies) {
		t.Fatalf("Expected cookies %v, got %v", expectedCookies, cookieData.Cookies)
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
