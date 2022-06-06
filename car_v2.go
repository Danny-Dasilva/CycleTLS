package main

import (
    "log"

    tls "github.com/Carcraftz/utls"
    "github.com/Carcraftz/cclient"
    http "github.com/Carcraftz/fhttp"
	"net/url"
    "time"

    "strings"
)

func main() {

    pageURL := "https://www.coinbase.com"
	

	// // Ensure user agent header is provided
	// userAgent := "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36"
	
	Body := ""
	Method := "GET"

    timeout := 5
	allowRedirect := true

    client, err := cclient.NewClient(tls.HelloChrome_Auto,"",allowRedirect,time.Duration(timeout))
    if err != nil {
        log.Fatal(err)
    }

    // resp, err := client.Get("https://www.coinbase.com")
    // if err != nil {
    //     log.Fatal(err)
    // }
    // resp.Body.Close()

    req, err := http.NewRequest(Method, pageURL, strings.NewReader(Body))
	if err != nil {
		panic(err)
	}

	Headers := map[string]string{}
	//master header order, all your headers will be ordered based on this list and anything extra will be appended to the end
	//if your site has any custom headers, see the header order chrome uses and then add those headers to this list
	masterheaderorder := []string{
		"host",
		"connection",
		"cache-control",
		"device-memory",
		"viewport-width",
		"rtt",
		"downlink",
		"ect",
		"sec-ch-ua",
		"sec-ch-ua-mobile",
		"sec-ch-ua-full-version",
		"sec-ch-ua-arch",
		"sec-ch-ua-platform",
		"sec-ch-ua-platform-version",
		"sec-ch-ua-model",
		"upgrade-insecure-requests",
		"user-agent",
		"accept",
		"sec-fetch-site",
		"sec-fetch-mode",
		"sec-fetch-user",
		"sec-fetch-dest",
		"referer",
		"accept-encoding",
		"accept-language",
		"cookie",
	}
	headermap := make(map[string]string)
	//TODO: REDUCE TIME COMPLEXITY (This code is very bad)
	headerorderkey := []string{}
	for _, key := range masterheaderorder {
		for k, v := range Headers {
			lowercasekey := strings.ToLower(k)
			if key == lowercasekey {
				headermap[k] = v
				headerorderkey = append(headerorderkey, lowercasekey)
			}
		}

	}
	for k, v := range req.Header {
		if _, ok := headermap[k]; !ok {
			headermap[k] = v[0]
			headerorderkey = append(headerorderkey, strings.ToLower(k))
		}
	}

	//ordering the pseudo headers and our normal headers
	req.Header = http.Header{
		http.HeaderOrderKey:  headerorderkey,
		http.PHeaderOrderKey: {":method", ":authority", ":scheme", ":path"},
	}
	//set our Host header
	u, err := url.Parse(pageURL)
	if err != nil {
		panic(err)
	}
	//append our normal headers
	for k, v := range Headers {
		if k != "Content-Length" && !strings.Contains(k, "Poptls") {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("Host", u.Host)
	resp, err := client.Do(req)
	if err != nil {
		log.Println("erfr", err)
		return
	}
	defer resp.Body.Close()
    log.Println(resp.Status, resp.Body)
}