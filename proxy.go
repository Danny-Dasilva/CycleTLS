package main

import (
	"compress/zlib"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"time"

	"bytes"
	"compress/gzip"
	"net/url"
	"strings"

	"github.com/Carcraftz/cclient"
	"github.com/andybalholm/brotli"

	http "github.com/Carcraftz/fhttp"

	tls "github.com/Carcraftz/utls"
)

//var client http.Client



func main() {
	// Ensure page URL header is provided
	pageURL := "https://www.coinbase.com"
	

	// Ensure user agent header is provided
	userAgent := "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36"
	
	Body := ""
	Method := "GET"


	allowRedirect := true
	
	timeout, err := strconv.Atoi("7")
	if err != nil {
		//default timeout of 6
		timeout = 6
	}
	
	// Change JA3
	var tlsClient tls.ClientHelloID
	if strings.Contains(strings.ToLower(userAgent), "chrome") {
		tlsClient = tls.HelloChrome_Auto
	} else if strings.Contains(strings.ToLower(userAgent), "firefox") {
		tlsClient = tls.HelloFirefox_Auto
	} else {
		tlsClient = tls.HelloIOS_Auto
	}
	client, err := cclient.NewClient(tlsClient, "", allowRedirect, time.Duration(timeout))
	if err != nil {
		log.Fatal(err)
	}

	


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
		log.Println("err")
		return
	}
	defer resp.Body.Close()

	//req.Close = true

	
	//forward decoded response body
	encoding := resp.Header["Content-Encoding"]
	body, err := ioutil.ReadAll(resp.Body)
	finalres := ""
	if err != nil {
		panic(err)
	}
	finalres = string(body)
	if len(encoding) > 0 {
		if encoding[0] == "gzip" {
			unz, err := gUnzipData(body)
			if err != nil {
				panic(err)
			}
			finalres = string(unz)
		} else if encoding[0] == "deflate" {
			unz, err := enflateData(body)
			if err != nil {
				panic(err)
			}
			finalres = string(unz)
		} else if encoding[0] == "br" {
			unz, err := unBrotliData(body)
			if err != nil {
				panic(err)
			}
			finalres = string(unz)
		} else {
			fmt.Println("UNKNOWN ENCODING: " + encoding[0])
			finalres = string(body)
		}
	} else {
		finalres = string(body)
	}
	log.Println(finalres)
}

func gUnzipData(data []byte) (resData []byte, err error) {
	gz, _ := gzip.NewReader(bytes.NewReader(data))
	defer gz.Close()
	respBody, err := ioutil.ReadAll(gz)
	return respBody, err
}
func enflateData(data []byte) (resData []byte, err error) {
	zr, _ := zlib.NewReader(bytes.NewReader(data))
	defer zr.Close()
	enflated, err := ioutil.ReadAll(zr)
	return enflated, err
}
func unBrotliData(data []byte) (resData []byte, err error) {
	br := brotli.NewReader(bytes.NewReader(data))
	respBody, err := ioutil.ReadAll(br)
	return respBody, err
}
