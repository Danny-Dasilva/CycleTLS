# CycleTLS



<div align="center">
	<img src="docs/media/Banner.png" alt="CycleTLS"/>
	<br>
	
Accepting Community Support and PR's

	
	

![build](https://github.com/Danny-Dasilva/CycleTLS/actions/workflows/test_golang.yml/badge.svg)
[![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg)](http://godoc.org/github.com/Danny-Dasilva/CycleTLS/cycletls) 
[![license](https://img.shields.io/github/license/Danny-Dasilva/CycleTLS.svg)](https://github.com/Danny-Dasilva/CycleTLS/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Danny-Dasilva/CycleTLS/cycletls)](https://goreportcard.com/report/github.com/Danny-Dasilva/CycleTLS/cycletls)
[![npm version](https://img.shields.io/npm/v/cycletls.svg)](https://www.npmjs.org/package/cycletls)
 <a href="https://discord.gg/gsmxMHrwhu">
  <img src="https://img.shields.io/discord/1100945880888115200?logo=discord"
      alt="chat on Discord"></a>
</div>

If you have a API change or feature request feel free to open an [Issue](https://github.com/Danny-Dasilva/CycleTLS/issues/new/choose)



# 🚀 Features

- [High-performance](#-performance) Built-in goroutine pool used for handling asynchronous requests
- Custom header ordering via [fhttp](https://github.com/useflyent/fhttp)
- Proxy support | Socks4, Socks5, Socks5h
- Ja3 Token configuration
- HTTP/3 and QUIC support
- WebSocket client
- Server-Sent Events (SSE)
- Connection reuse
- JA4 fingerprinting


Table of contents
=================


* [Table of contents](#table-of-contents)
* [Installation](#installation)
* [Usage](#usage)
	* [QuickStart JS](#example-cycletls-request-for-typescript-and-javascript)
	* [Quickstart Golang](#example-cycletls-request-for-golang)
	* [Initializing CycleTLS](#creating-an-instance)
	* [API/Methods](#cycletls-alias-methods)
	* [Request Config](#cycletls-request-config)
	* [Response Schema](#cycletls-response-schema)
	* [Multiple Requests Example](#multiple-requests-example-for-typescript-and-javascript)
	* [Streaming Responses](#streaming-responses-axios-style)
* [Local Setup](#dev-setup)
* [QA](#questions)
* [LICENSE](#license)

## Dependencies

```
node ^v18.0
golang ^v1.21x
```

## Installation

Node Js

```bash
$ npm install cycletls
```

Golang

```bash
$ go get github.com/Danny-Dasilva/CycleTLS/cycletls 
```

# Usage 

## Example CycleTLS Request for Typescript and Javascript

You can run this test in `tests/simple.test.ts`

```js
const initCycleTLS = require('cycletls');
// Typescript: import initCycleTLS from 'cycletls';

(async () => {
  // Initiate CycleTLS
  const cycleTLS = await initCycleTLS();

  // Send request
  const response = await cycleTLS('https://ja3er.com/json', {
    body: '',
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    proxy: 'http://username:password@hostname.com:443', 
  }, 'get');

  // Parse response as JSON
  const data = await response.json();
  console.log(data);

  // Cleanly exit CycleTLS
  cycleTLS.exit();

})();

```

## JA4 TLS Fingerprinting

JA4 is an enhanced TLS fingerprinting method that provides more detailed client identification:

### JavaScript Example
```js
const initCycleTLS = require('cycletls');

(async () => {
  const cycleTLS = await initCycleTLS();

  // Firefox JA4 fingerprint
  const response = await cycleTLS('https://tls.peet.ws/api/all', {
    ja4: 't13d1717h2_5b57614c22b0_f2748d6cd58d'
  });

  const data = await response.json();
  console.log('JA4:', data.tls.ja4);
  console.log('JA4_r:', data.tls.ja4_r);
  console.log('TLS Version:', data.tls.tls_version_negotiated);
  
  cycleTLS.exit();
})();
```

### Golang JA4 Example
```go
package main

import (
    "log"
    "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
    client := cycletls.Init()
    defer client.Close()

    // Chrome JA4 fingerprint
    response, err := client.Do("https://tls.peet.ws/api/all", cycletls.Options{
        Ja4: "t13d1517h2_8daaf6152771_7e51fdad25f2",
        UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36",
    }, "GET")
    
    if err != nil {
        log.Fatal(err)
    }
    log.Println("Response with JA4:", response.Status)
}
```

## HTTP/2 Fingerprinting

HTTP/2 fingerprinting allows you to mimic specific browser HTTP/2 implementations:

### JavaScript Example
```js
const initCycleTLS = require('cycletls');

(async () => {
  const cycleTLS = await initCycleTLS();

  // Firefox HTTP/2 fingerprint
  const response = await cycleTLS('https://tls.peet.ws/api/all', {
    http2Fingerprint: '1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s'
  });

  const data = await response.json();
  console.log('HTTP/2 Fingerprint:', data.http2.akamai_fingerprint);
  console.log('Settings:', data.http2.sent_frames[0].settings);
  
  cycleTLS.exit();
})();
```

### Golang HTTP/2 Example
```go
package main

import (
    "log"
    "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
    client := cycletls.Init()
    defer client.Close()

    // Firefox HTTP/2 fingerprint
    response, err := client.Do("https://tls.peet.ws/api/all", cycletls.Options{
        HTTP2Fingerprint: "1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s",
        UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0",
    }, "GET")
    
    if err != nil {
        log.Fatal(err)
    }
    log.Println("Response with HTTP/2 fingerprint:", response.Status)
}
```

### Common Browser HTTP/2 Fingerprints

| Browser | HTTP/2 Fingerprint | Description |
|---------|-------------------|-------------|
| Firefox | `1:65536;2:0;4:131072;5:16384\|12517377\|0\|m,p,a,s` | Smaller window size, MPAS priority |
| Chrome | `1:65536;2:0;4:6291456;6:262144\|15663105\|0\|m,a,s,p` | Larger window size, MASP priority |

### Combined Fingerprinting Example
```js
const initCycleTLS = require('cycletls');

(async () => {
  const cycleTLS = await initCycleTLS();

  // Complete Firefox browser fingerprint
  const response = await cycleTLS('https://tls.peet.ws/api/all', {
    ja4: 't13d1717h2_5b57614c22b0_f2748d6cd58d',
    http2Fingerprint: '1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0'
  });

  const data = await response.json();
  console.log('Complete fingerprint applied successfully');
  console.log('JA4:', data.tls.ja4);
  console.log('HTTP/2:', data.http2.akamai_fingerprint);
  
  cycleTLS.exit();
})();
```

## Streaming Responses (Axios-style)

CycleTLS supports axios-compatible streaming responses for real-time data processing:

### Basic Streaming Example

```js
const initCycleTLS = require('cycletls');

(async () => {
  const cycleTLS = await initCycleTLS();

  // Get streaming response
  const response = await cycleTLS.get('https://httpbin.org/stream/3', {
    headers: { Authorization: `Bearer your_token_here` },
    responseType: 'stream'
  });

  const stream = response.data;

  stream.on('data', data => {
    console.log('Received chunk:', data.toString());
  });

  stream.on('end', () => {
    console.log("stream done");
    cycleTLS.exit();
  });

  stream.on('error', (error) => {
    console.error('Stream error:', error);
    cycleTLS.exit();
  });
})();
```

### Advanced Streaming with Error Handling

```js
const initCycleTLS = require('cycletls');

(async () => {
  const cycleTLS = await initCycleTLS();

  try {
    const response = await cycleTLS.get('https://httpbin.org/drip?numbytes=100&duration=2', {
      responseType: 'stream',
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    });

    console.log('Status:', response.status);
    console.log('Headers:', response.headers);

    const chunks = [];
    
    response.data.on('data', (chunk) => {
      chunks.push(chunk);
      console.log(`Received ${chunk.length} bytes`);
    });

    response.data.on('end', () => {
      console.log('Stream complete');
      const fullData = Buffer.concat(chunks);
      console.log('Total received:', fullData.length, 'bytes');
      cycleTLS.exit();
    });

    response.data.on('error', (error) => {
      console.error('Stream error:', error);
      cycleTLS.exit();
    });

  } catch (error) {
    console.error('Request failed:', error);
    cycleTLS.exit();
  }
})();
```

### Non-Streaming Responses (Default Behavior)

For non-streaming responses, CycleTLS works exactly as before:

```js
// These return buffered responses (existing behavior)
const jsonResponse = await cycleTLS.get('https://httpbin.org/json', {
  responseType: 'json' // or omit for default JSON parsing
});
const jsonData = await jsonResponse.json();
console.log(jsonData); // Parsed JSON object

const textResponse = await cycleTLS.get('https://httpbin.org/html', {
  responseType: 'text'
});
const textData = await textResponse.text();
console.log(textData); // String content
```

## Example CycleTLS Request for Golang

```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init()
	defer client.Close()

	response, err := client.Do("https://ja3er.com/json", cycletls.Options{
		Body : "",
		Ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		EnableConnectionReuse: true, // Enable connection reuse for better performance
	  }, "GET");
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response)
}

```
#### Example using your own custom http.Client

<details>

```go
import (
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	http "github.com/Danny-Dasilva/fhttp" // note this is a drop-in replacement for net/http
)

func main() {
	ja3 := "771,52393-52392-52244-52243-49195-49199-49196-49200-49171-49172-156-157-47-53-10,65281-0-23-35-13-5-18-16-30032-11-10,29-23-24,0"
	ua := "Chrome Version 57.0.2987.110 (64-bit) Linux"

 	 cycleClient := &http.Client{
 		Transport:     cycletls.NewTransport(ja3, ua),
 	 }

	resp, err := cycleClient.Get("https://tls.peet.ws/")
	...
}
```
</details>

## Creating an instance

In order to create a `cycleTLS` instance, you can run the following:

#### JavaScript

```js
// The initCycleTLS function spawns a Golang process that handles all requests concurrently via goroutine loops. 
const initCycleTLS = require('cycletls');
// import initCycleTLS from 'cycletls';

// Async/Await method
const cycleTLS = await initCycleTLS();
// With optional configuration
const cycleTLS = await initCycleTLS({ port: 9118, timeout: 30000 });
// .then method
initCycleTLS().then((cycleTLS) => {});

```
#### Golang

```go
import (
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

//The `Init` function initializes golang channels to process requests. 
client := cycletls.Init()
```


## CycleTLS Alias Methods

The following methods exist in CycleTLS

**cycleTLS(url, [config])**

**cycleTLS.get(url, [config])**

**cycleTLS.delete(url, [config])**

**cycleTLS.head(url, [config])**

**cycleTLS.options(url, [config])**

**cycleTLS.post(url, [config])**

**cycleTLS.put(url, config)**

**cycleTLS.patch(url, [config])**

Url is not optional, config is optional

## CycleTLS Request Config

```js
{
  // URL for the request (required if not specified as an argument)
  url: "https://example.com"
  // Method for the request ("head" | "get" | "post" | "put" | "delete" | "trace" | "options" | "connect" | "patch")
  method: "get" // Default method
  // Custom headers to send
  headers: { "Authorization": "Bearer someexampletoken" }
  // Custom cookies to send
  Cookies: [{
    "name": "key",
    "value": "val",
    "path":  "/docs",
    "domain":  "google.com",
                "expires": "Mon, 02-Jan-2022 15:04:05 EST"
    "maxAge": 90,
    "secure": false,
    "httpOnly": true,
    "sameSite": "Lax"		
  }],
  // Body to send with request (must be a string - cannot pass an object)
  body: '',
  // JA3 token to send with request
  ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
  // JA4 token for enhanced fingerprinting
  ja4: 't13d1516h2_8daaf6152771_02713d6af862',
  // User agent for request
  userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
  // Proxy to send request through (supports http, socks4, socks5, socks5h)
  proxy: 'http://username:password@hostname.com:443',
  // Amount of seconds before request timeout (default: 7)
  timeout: 2,
  // Toggle if CycleTLS should follow redirects
  disableRedirect: true,
  // Custom header order to send with request (This value will overwrite default header order)
  headerOrder: ["cache-control", "connection", "host"],
  // Toggle if CycleTLS should skip verify certificate (If InsecureSkipVerify is true, TLS accepts any certificate presented by the server and any host name in that certificate.)
  insecureSkipVerify: false	
  // Forces CycleTLS to do a http1 handshake
  forceHTTP1: false
  // Forces HTTP/3 protocol
  forceHTTP3: false
  // Enable connection reuse across requests
  enableConnectionReuse: true
  // HTTP/2 fingerprint
  http2Fingerprint: '1:65536;4:131072;5:16384|12517377|3:0:0:201,5:0:0:101,7:0:0:1,9:0:7:1,11:0:3:1,13:0:0:241|m,p,a,s'
  // QUIC fingerprint for HTTP/3
  quicFingerprint: '16030106f2010006ee03039a2b98d81139db0e128ea09eff...'
}

```

## CycleTLS Response Schema

```js
{
  // Status code returned from server (Number)
  status: 200,
  // Body returned from the server (String)
  body: "",
  // Headers returned from the server (Object)
  headers: {
    "some": "header",
    ...
  },
  // FinalUrl returned from the server (String). This field is useful when redirection is active.
  finalUrl: "https://final.url/"	
}

```



## Multiple Requests Example for Typescript and Javascript

If CycleTLS is being used by in a JavaScript environment, CycleTLS will spawn a Golang process to handle requests. This Golang process handles requests `concurrently` in a worker pool. Due to this, CycleTLS returns response objects as soon as they are made available 
(in other terms, CycleTLS processes requests as they are received, but responses are returned asynchronously so they will NOT be returned in the order requested)

If you are using CycleTLS in JavaScript, it is necessary to exit out of the instance to prevent zombie processes. The example below shows one way to approach cleanly exiting CycleTLS if you need to process multiple requests (note: keep in mind that calling the `exit()` function will kill any requests in progress). If your workflow requires requests running the entire time the process runs, modules such as [exit-hook](https://www.npmjs.com/package/exit-hook) could serve as an alternative solution to cleanly exiting CycleTLS.

```js
const initCycleTLS = require("cycletls");
// Typescript: import initCycleTLS from 'cycletls';

// Defining JA3 token and user agent
const ja3 = "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
const userAgent = "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0";

// Defining multiple requests
const requestDict = {
  "https://httpbin.org/user-agent": {
    ja3: ja3,
    userAgent: userAgent,
  },
  "http://httpbin.org/post": {
    body: '{"field":"POST-VAL"}',
    method: "POST",
  },
  "http://httpbin.org/cookies": {
    cookies: [
      {
        name: "example1",
        value: "aaaaaaa",
        expires: "Mon, 02-Jan-2022 15:04:05 EST",
      },
    ],
  },
};

// Anonymous async function
(async () => {
  // Initiate CycleTLS
  const cycleTLS = await initCycleTLS();

  // Create promises for all requests
  const promises = Object.entries(requestDict).map(async ([url, params]) => {
    const response = await cycleTLS(
      url, {
        body: params.body ?? "",
        ja3: params.ja3 ?? ja3,
        userAgent: params.userAgent ?? userAgent,
        headers: params.headers,
        cookies: params.cookies,
      }, params.method ?? "GET");

    // Parse response based on content type
    const data = await response.json();
    console.log(url, data);
    return { url, data };
  });

  // Wait for all requests to complete
  await Promise.all(promises);
  
  // Cleanly exit CycleTLS
  cycleTLS.exit();
})();
```

## Multiple Requests Example for Golang

The general expectation for golang packages is to expect the user to implement a worker pool or any other form of goroutine/asynchronous processing. This package includes a built in Queue method that leverages a worker pool/channels for long running asynchronous requests against a set of urls.

```go
package main

import (
	"log"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// Static variables
var (
	ja3       = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0"
	userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36"
)

// RequestConfig holds the configuration for each request.
type RequestConfig struct {
	URL     string
	Method  string
	Options cycletls.Options
}

func main() {
	client := cycletls.Init(true) // Initialize with worker pool

	// Define the requests
	requests := []RequestConfig{
		{
			URL:    "http://httpbin.org/delay/4",
			Method: "GET",
			Options: cycletls.Options{
				Ja3:       ja3,
				UserAgent: userAgent,
			},
		},
		{
			URL:    "http://httpbin.org/post",
			Method: "POST",
			Options: cycletls.Options{
				Body:      `{"field":"POST-VAL"}`,
				Ja3:       ja3,
				UserAgent: userAgent,
			},
		},
		{
			URL:    "http://httpbin.org/cookies",
			Method: "GET",
			Options: cycletls.Options{
				Ja3:       ja3,
				UserAgent: userAgent,
				Cookies: []cycletls.Cookie{
					{
						Name:  "example1",
						Value: "aaaaaaa",
					},
				},
			},
		},
	}

	// Queue the requests
	for _, req := range requests {
		client.Queue(req.URL, req.Options, req.Method)
	}

	// Asynchronously read responses as soon as they are available
	// They will return as soon as they are processed
	// e.g. Delay 3 will be returned last
	for i := 0; i < len(requests); i++ {
		response := <-client.RespChan
		log.Println("Response:", response)
	}

	// Close the client
	client.Close()
}

```

# Dev Setup

If you would like to compile CycleTLS on your own, use the following commands:

Set module-aware mode
`go env -w  GO111MODULE=off`

Install golang dependencies
`go get github.com/Danny-Dasilva/CycleTLS/cycletls`

install npm packages (this command handles the above)

`npm install`

### To recompile index.ts in the src folder

`npm run build`

### To recompile Golang files in the golang folder

All

`npm run build:go`

Windows

`npm run build:go:windows:amd64`

Linux

`npm run build:go:linux:amd64`

Mac

`npm run build:go:mac:arm64`


You can view the available compile options within the `package.json`

## Questions

### How do I set Cookies
<details>

There are two simple ways to interface with cookies 
### Javascript Simple Cookie Configuration

```js
const initCycleTLS = require("cycletls");
(async () => {
  // Initiate cycleTLS
  const cycleTLS = await initCycleTLS();
  const response = await cycleTLS("https://httpbin.org/cookies", {
    cookies: {
      cookie1: "value1",
      cookie2: "value2",
    },
  });
  
  const data = await response.json();
  console.log(data);
  /* Expected
  {
    "cookies": {
      "cookie1": "value1",
      "cookie2": "value2"
    }
  }
  */
  cycleTLS.exit();
})();
```

In this simple example you can set the cookie `name` and `value` within an object


### Javascript Complex Cookie Configuration

If you wish to have more fine grained control over cookie parameters you have access to the full underlying Go struct

here are the following values you can set

```ts
export interface Cookie {
  name: string;
  value: string;
  path?: string;
  domain?: string;
  expires?: string;
  rawExpires?: string;
  maxAge?: number;
  secure?: boolean;
  httpOnly?: boolean;
  sameSite?: string;
  unparsed?: string;
}
```

you can use them in a request as follows

```js
const initCycleTLS = require("cycletls");
(async () => {
  // Initiate cycleTLS
  const cycleTLS = await initCycleTLS();
  const complexCookies = [
    {
      name: "cookie1",
      value: "value1",
      domain: "httpbin.org",
    },
    {
      name: "cookie2",
      value: "value2",
      domain: "httpbin.org",
    },
  ];

  const response = await cycleTLS("https://httpbin.org/cookies", {
    cookies: complexCookies,
  });

  const data = await response.json();
  console.log(data);
  /* Expected
  {
    "cookies": {
      "cookie1": "value1",
      "cookie2": "value2"
    }
  }
  */
  cycleTLS.exit();
})();
```


### Golang Configure Cookies
```golang
package main

import (
    "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
    resp, err := client.Do("https://httpbin.org/cookies", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Cookies: []cycletls.Cookie{{Name: "cookie1", Value: "value1"},
			{Name: "cookie2", Value: "value2"}},
    }, "GET")
    if err != nil {
      log.Print("Request Failed: " + err.Error())
    }
    log.Println(resp.Body)
    /* Expected
    {
      "cookies": {
        "cookie1": "value1", 
        "cookie2": "value2"
      }
      }
    */
    
    // Alternatively if you want access to values within a map
    log.Println(resp.JSONBody())
    /* Expected
    map[cookies:map[cookie1:value1 cookie2:value2]]
    */
}

```


Feel free to open an [Issue](https://github.com/Danny-Dasilva/CycleTLS/issues/new/choose) with a feature request for specific file type support. 
</details>


### How do I use CookieJar in CycleTLS?

<details>


### CookieJar in JS

```js
const initCycleTLS = require("cycletls");

const tough = require("tough-cookie");
const Cookie = tough.Cookie;

(async () => {
  // Initiate cycleTLS and CookieJar
  const cycleTLS = await initCycleTLS();
  const cookieJar = new tough.CookieJar();

  // Capture a set cookie
  const firstResponse = await cycleTLS.get(
    "https://httpbin.org/cookies/set?freeform=test",
    {
      disableRedirect: true,
    }
  );
  
  // Now use the processCookies function to add the cookies from the response headers to the cookie jar
  await processCookies(
    firstResponse,
    "https://httpbin.org/cookies/set?freeform=test",
    cookieJar
  );
  // Now send a second to verify we have our cookies
  const secondResponse = await cycleTLS.get("https://httpbin.org/cookies", {
    headers: {
      cookie: await cookieJar.getCookieString("https://httpbin.org/cookies"),
    },
  });
  
  // Verify cookies were set
  const data = await secondResponse.json();
  console.log(data)
  /* Expected
  {
    "cookies": {
      "freeform": "test"
    }
  }
  */
  cycleTLS.exit();
})();

async function processCookies(response, url, cookieJar) {
  if (response.headers["Set-Cookie"] instanceof Array) {
    response.headers["Set-Cookie"].map(
      async (cookieString) => await cookieJar.setCookie(cookieString, url)
    );
  } else {
    await cookieJar.setCookie(response.headers["Set-Cookie"], url);
  }
}
```


### CookieJar in Golang

```go
package main

import (
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"log"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

func main() {
	client := cycletls.Init()
	jar, err := cookiejar.New(nil)
  if err != nil {
      log.Fatal(err)
  }
	// First request to set cookie
	firstResponse, err := client.Do("https://httpbin.org/cookies/set?a=1&b=2&c=3", cycletls.Options{
		Body: "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		DisableRedirect: true,
	},
		 "GET")
	if err != nil {
		log.Fatal(err)
	}
	firstURL, _ := url.Parse(firstResponse.FinalUrl)
  jar.SetCookies( firstURL, firstResponse.Cookies)


	// Second request to verify cookies, including the cookies from the first response
	secondResponse, err := client.Do("https://httpbin.org/cookies", cycletls.Options{
    Body: "",
    Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
    UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
    Headers: map[string]string{
        "Cookie": getHeadersFromJar(jar, firstURL),
    },
	}, "GET")
	if err != nil {
	    log.Fatal(err)
	}

	log.Println("Second Response body:", secondResponse.Body)
}


func getHeadersFromJar(jar *cookiejar.Jar, url *url.URL) string {
    cookies := jar.Cookies(url)
    var cookieStrs []string
    for _, cookie := range cookies {
        cookieStrs = append(cookieStrs, cookie.Name+"="+cookie.Value)
    }
    return strings.Join(cookieStrs, "; ")
}

```

</details>

### How do I send multipart/form-data in CycleTLS

<details>

### Javascript Text form-data
```js
const initCycleTLS = require("cycletls");
const FormData = require('form-data');

(async () => {
  const cycleTLS = await initCycleTLS();

  const formData = new FormData();
  formData.append("key1", "value1");
  formData.append("key2", "value2");
  
  const response = await cycleTLS('http://httpbin.org/post', {
      body: formData,
      headers: formData.getHeaders(), // Use formData.getHeaders() for proper content-type
  }, 'post');

  const data = await response.json();
  console.log(data);

  cycleTLS.exit();
})();

```


### Javascript File form-data
```js
const initCycleTLS = require("cycletls");
const FormData = require('form-data');
const fs = require('fs');

(async () => {
  const cycleTLS = await initCycleTLS();

  const formData = new FormData();
  const fileStream = fs.createReadStream("../go.mod");
  formData.append('file', fileStream);

  const response = await cycleTLS('http://httpbin.org/post', {
      body: formData,
      headers: formData.getHeaders(), // Use formData.getHeaders() for proper content-type
  }, 'post');

  const data = await response.json();
  console.log(data);

  cycleTLS.exit();
})();

```

### Golang Text form-data
```golang
package main

import (
	"bytes"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"log"
	"mime/multipart"
)

func main() {
	client := cycletls.Init()

	// Prepare a buffer to write our multipart form
	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)

	// Add form fields
	multipartWriter.WriteField("key1", "value1")
	multipartWriter.WriteField("key2", "value2")

	contentType := multipartWriter.FormDataContentType()
	// Close the writer before making the request
	multipartWriter.Close()

	response, err := client.Do("http://httpbin.org/post", cycletls.Options{
		Body: requestBody.String(),
		Headers: map[string]string{
			"Content-Type": contentType,
		},
	}, "POST")

	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}

	log.Println(response.Body)
}
```


### Golang file upload form-data
```golang
package main

import (
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"os"
)

func main() {
  client := cycletls.Init()

  // Prepare a buffer to write our multipart form
  var requestBody bytes.Buffer
  multipartWriter := multipart.NewWriter(&requestBody)

  // Add a file
  fileWriter, err := multipartWriter.CreateFormFile("fieldname", "filename")
  if err != nil {
      log.Fatal("CreateFormFile Error: ", err)
  }

  // Open the file that you want to upload
  file, err := os.Open("path/to/your/file")
  if err != nil {
      log.Fatal("File Open Error: ", err)
  }
  defer file.Close()

  // Copy the file to the multipart writer
  _, err = io.Copy(fileWriter, file)
  if err != nil {
      log.Fatal("File Copy Error: ", err)
  }

  // Close the writer before making the request
  contentType := multipartWriter.FormDataContentType()
  multipartWriter.Close()

  response, err := client.Do("http://httpbin.org/post", cycletls.Options{
      Body: requestBody.String(),
      Headers: map[string]string{
          "Content-Type": contentType,
      },
  }, "POST")

  if err != nil {
      log.Print("Request Failed: " + err.Error())
  }

  log.Println(response.Body)
}
```


If requested encoding helpers can be added to the repo for golang 
</details>

### How do I send a application/x-www-form-urlencoded Post request

<details>

### Javascript application/x-www-form-urlencoded form
```js
const initCycleTLS = require("cycletls");
(async () => {
  const cycleTLS = await initCycleTLS();

  const urlEncodedData = new URLSearchParams();
  urlEncodedData.append('key1', 'value1');
  urlEncodedData.append('key2', 'value2');

  const response = await cycleTLS('http://httpbin.org/post', {
      body: urlEncodedData,
      headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
      },
  }, 'post');

  const data = await response.json();
  console.log(data);

  cycleTLS.exit();
})();

```
### Golang application/x-www-form-urlencoded form
```golang
package main

import (
    "log"
	  "net/url"
    "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {

	client := cycletls.Init()

	// Prepare form data
	form := url.Values{}
	form.Add("key1", "value1")
	form.Add("key2", "value2")

	response, err := client.Do("http://httpbin.org/post", cycletls.Options{
		Body: form.Encode(),
		Headers: map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		},
	}, "POST")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response.Body)
}

```
</details>

### How do I download images?

<details>

Images with a `Content-Type` header of the following types are base 64 encoded. 

**Supported Image Types**
* `image/svg+xml`
* `image/webp`
* `image/jpeg`
* `image/png`
* `application/pdf`

To write them to a file you can use the below methods

### Javascript Image Write to File
```js
const initCycleTLS = require("cycletls");
var fs = require("fs");

//Function to write image to a file
const writeImage = (filename, data) => {
  let writeStream = fs.createWriteStream(filename);

  // write some data with a base64 encoding
  writeStream.write(data, "base64");
  writeStream.on("finish", () => {
    console.log(`wrote to file ${filename}`);
  });
  
  // close the stream
  writeStream.end();
};

(async () => {
  const cycleTLS = await initCycleTLS();
  // try {

  const jpegImage = await cycleTLS("http://httpbin.org/image/jpeg", {
    ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
    userAgent:
      "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
  });
  // Get image data and write to file
  const imageData = await jpegImage.text(); // Image data is base64 encoded
  writeImage("test.jpeg", imageData);

  cycleTLS.exit();
})();

```
### Golang Image Write to File
```golang
package main

import (
    "encoding/base64"
    "os"
    "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {

    client := cycletls.Init()
    response, err := client.Do("http://httpbin.org/image/jpeg", cycletls.Options{
      Body:      "",
      Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
      UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
    }, "GET")
    // Decode Base64
    dec, err := base64.StdEncoding.DecodeString(response.Body)
    if err != nil {
        panic(err)
    }
    //create file to write
    f, err := os.Create("test.jpeg")
    if err != nil {
        panic(err)
    }
    defer f.Close()
    //write b64 to file
    if _, err := f.Write(dec); err != nil {
        panic(err)
    }
    if err := f.Sync(); err != nil {
        panic(err)
    }
}

```


Additional file type support is planned.

Feel free to open an [Issue](https://github.com/Danny-Dasilva/CycleTLS/issues/new/choose) with a feature request for specific file type support. 
</details>

### How do I use Connection Reuse?

<details>

Connection reuse allows you to reuse TLS connections across multiple requests to the same host, reducing handshake overhead and improving performance.

### Golang Connection Reuse

```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	// Initialize without worker pool for better connection management
	client := cycletls.Init(false)
	defer client.Close()

	// Enable connection reuse in the options
	options := cycletls.Options{
		Ja3:                   "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent:             "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		EnableConnectionReuse: true, // Enable connection reuse
	}

	// First request - establishes connection
	resp1, err := client.Do("https://httpbin.org/get", options, "GET")
	if err != nil {
		log.Fatal("First request failed: ", err)
	}
	log.Println("First request status:", resp1.Status)

	// Second request - reuses connection
	resp2, err := client.Do("https://httpbin.org/headers", options, "GET")
	if err != nil {
		log.Fatal("Second request failed: ", err)
	}
	log.Println("Second request status:", resp2.Status)

	// Connection is reused for requests to the same host
}
```

</details>

### How do I use HTTP/3 and QUIC?

<details>

CycleTLS now supports HTTP/3 over QUIC protocol with custom QUIC fingerprinting.

### Golang HTTP/3 Basic Usage

```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init()
	defer client.Close()

	// Force HTTP/3
	response, err := client.Do("https://cloudflare-quic.com/", cycletls.Options{
		ForceHTTP3:         true,
		UserAgent:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		InsecureSkipVerify: true,
	}, "GET")

	if err != nil {
		log.Fatal("Request failed: ", err)
	}

	log.Println("Response over HTTP/3:", response.Status)
}
```

### Golang QUIC Fingerprinting

```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init()
	defer client.Close()

	// Custom QUIC fingerprint
	quicFingerprint := "16030106f2010006ee03039a2b98d81139db0e128ea09eff6874549c219b543fb6dbaa7e4dbfe9e31602c620ce04c4026f019442affade7fed8ba66e022e186f77f1c670fd992f33c0143f120020aaaa130113021303c02bc02fc02cc030cca9cca8c013c014009c009d002f0035010006851a1a00000010000e000c02683208687474702f312e31002b000706dada03040303..."

	response, err := client.Do("https://cloudflare-quic.com/", cycletls.Options{
		QUICFingerprint:    quicFingerprint,
		ForceHTTP3:         true,
		UserAgent:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		InsecureSkipVerify: true,
	}, "GET")

	if err != nil {
		log.Fatal("Request failed: ", err)
	}

	log.Println("Response with QUIC fingerprint:", response.Status)
}
```

### Golang HTTP/3 Transport Direct Usage

```go
package main

import (
	"crypto/tls"
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	http "github.com/Danny-Dasilva/fhttp"
)

func main() {
	// Create TLS config
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	// Create HTTP/3 transport
	transport := cycletls.NewHTTP3Transport(tlsConfig)

	// Create request
	req, err := http.NewRequest("GET", "https://cloudflare-quic.com/", nil)
	if err != nil {
		log.Fatal("Failed to create request: ", err)
	}

	// Send request
	resp, err := transport.RoundTrip(req)
	if err != nil {
		log.Fatal("Request failed: ", err)
	}
	defer resp.Body.Close()

	log.Println("Direct HTTP/3 response:", resp.Status)
}
```

</details>

### How do I use WebSocket support?

<details>

CycleTLS provides a WebSocket client that supports custom TLS fingerprinting.

### JavaScript WebSocket Example
```js
const initCycleTLS = require('cycletls');

(async () => {
  const cycleTLS = await initCycleTLS();

  // WebSocket connection with TLS fingerprinting
  const wsResponse = await cycleTLS.ws('wss://echo.websocket.org', {
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    headers: {
      'Sec-WebSocket-Protocol': 'echo-protocol'
    }
  });

  // Check connection status
  if (wsResponse.status === 101) {
    console.log('WebSocket upgrade successful');
    console.log('Response headers:', wsResponse.headers);
  }

  cycleTLS.exit();
})();
```

### Golang WebSocket Example

```go
package main

import (
	"log"
	"net/http"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"github.com/gorilla/websocket"
	utls "github.com/refraction-networking/utls"
)

func main() {
	// Create TLS config
	tlsConfig := &utls.Config{
		InsecureSkipVerify: true,
	}

	// Create headers
	headers := make(http.Header)
	headers.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")

	// Create WebSocket client
	wsClient := cycletls.NewWebSocketClient(tlsConfig, headers)

	// Connect to WebSocket server
	conn, resp, err := wsClient.Connect("wss://echo.websocket.org/")
	if err != nil {
		log.Fatal("WebSocket connection failed: ", err)
	}
	defer conn.Close()

	log.Println("WebSocket connected, status:", resp.StatusCode)

	// Send message
	testMessage := "Hello, WebSocket!"
	if err := conn.WriteMessage(websocket.TextMessage, []byte(testMessage)); err != nil {
		log.Fatal("Failed to send message: ", err)
	}

	// Read response
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		log.Fatal("Failed to read message: ", err)
	}

	log.Printf("Received message type %d: %s\n", messageType, string(message))
}
```

### Golang WebSocket Response Wrapper

```go
package main

import (
	"log"
	"net/http"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"github.com/gorilla/websocket"
	utls "github.com/refraction-networking/utls"
)

func main() {
	// Setup WebSocket client
	tlsConfig := &utls.Config{
		InsecureSkipVerify: true,
	}

	headers := make(http.Header)
	headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	wsClient := cycletls.NewWebSocketClient(tlsConfig, headers)

	// Connect
	conn, _, err := wsClient.Connect("wss://echo.websocket.org/")
	if err != nil {
		log.Fatal("Connection failed: ", err)
	}

	// Create response wrapper
	wsResponse := &cycletls.WebSocketResponse{
		Conn: conn,
	}
	defer wsResponse.Close()

	// Send message using wrapper
	if err := wsResponse.Send(websocket.TextMessage, []byte("Hello!")); err != nil {
		log.Fatal("Send failed: ", err)
	}

	// Receive message using wrapper
	messageType, message, err := wsResponse.Receive()
	if err != nil {
		log.Fatal("Receive failed: ", err)
	}

	log.Printf("Received: %s (type: %d)
", string(message), messageType)
}
```


</details>

### How do I use Server-Sent Events (SSE)?

<details>

CycleTLS supports Server-Sent Events for real-time data streaming from servers.

### JavaScript SSE Example

```js
const initCycleTLS = require('cycletls');

(async () => {
  const cycleTLS = await initCycleTLS();

  // SSE connection with TLS fingerprinting
  const sseResponse = await cycleTLS.sse('https://example.com/events', {
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    headers: {
      'Accept': 'text/event-stream',
      'Cache-Control': 'no-cache'
    }
  });

  // Parse real-time events
  const eventData = await sseResponse.text();
  console.log('SSE events:', eventData);

  cycleTLS.exit();
})();
```

### JavaScript SSE with Streaming 

```js
const initCycleTLS = require('cycletls');

(async () => {
  const cycleTLS = await initCycleTLS();

  // SSE with streaming for real-time processing
  const response = await cycleTLS.get('https://example.com/events', {
    responseType: 'stream',
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    headers: {
      'Accept': 'text/event-stream',
      'Cache-Control': 'no-cache'
    }
  });

  // Process SSE stream in real-time
  const stream = response.data;
  let buffer = '';

  stream.on('data', (chunk) => {
    buffer += chunk.toString();
    const lines = buffer.split('
');
    
    // Process complete lines, keep incomplete line in buffer
    buffer = lines.pop() || '';
    
    for (const line of lines) {
      if (line.startsWith('data:')) {
        const eventData = line.substring(5).trim();
        console.log('Event data:', eventData);
      } else if (line.startsWith('event:')) {
        const eventType = line.substring(6).trim();
        console.log('Event type:', eventType);
      }
    }
  });

  stream.on('end', () => {
    console.log('SSE stream ended');
    cycleTLS.exit();
  });

  stream.on('error', (error) => {
    console.error('SSE stream error:', error);
    cycleTLS.exit();
  });
})();
```

### Golang SSE Client Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	fhttp "github.com/Danny-Dasilva/fhttp"
)

func main() {
	// Create HTTP client
	httpClient := &fhttp.Client{
		Timeout: 30 * time.Second,
	}

	// Create headers
	headers := make(fhttp.Header)
	headers.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	headers.Set("Accept", "text/event-stream")

	// Create SSE client
	sseClient := cycletls.NewSSEClient(httpClient, headers)

	// Connect to SSE server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sseResp, err := sseClient.Connect(ctx, "http://localhost:3333/events")
	if err != nil {
		log.Fatal("SSE connection failed: ", err)
	}
	defer sseResp.Close()

	// Read events
	eventCount := 0
	for eventCount < 5 {
		event, err := sseResp.NextEvent()
		if err != nil {
			log.Printf("Error reading event: %v\n", err)
			break
		}
		
		if event != nil {
			eventCount++
			fmt.Printf("Event #%d:\n", eventCount)
			fmt.Printf("  Type: %s\n", event.Event)
			fmt.Printf("  Data: %s\n", event.Data)
			fmt.Printf("  ID: %s\n", event.ID)
		}
	}
}
```

### Golang SSE with Browser Configuration

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	// Create browser configuration
	browser := cycletls.Browser{
		UserAgent:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		JA3:                "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		InsecureSkipVerify: true,
	}

	// Connect to SSE endpoint
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := browser.SSEConnect(ctx, "http://127.0.0.1:3333/events")
	if err != nil {
		log.Fatal("SSE connection failed: ", err)
	}
	defer response.Close()

	// Process events
	for {
		event, err := response.NextEvent()
		if err != nil {
			log.Printf("Event stream ended: %v\n", err)
			break
		}
		
		if event != nil && event.Data != "" {
			fmt.Printf("Received event: %s\n", event.Data)
			
			// Break after receiving specific event
			if event.Data == "done" {
				break
			}
		}
	}
}
```

</details>

### How do I use JA4 fingerprinting?

<details>

JA4 is an enhanced TLS fingerprinting method that provides additional client identification capabilities.

### Golang JA4 Fingerprinting

```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init()
	defer client.Close()

	// Use both JA3 and JA4 fingerprints
	response, err := client.Do("https://tls.peet.ws/api/clean", cycletls.Options{
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		Ja4:       "t13d1516h2_8daaf6152771_02713d6af862", // JA4 fingerprint
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
	}, "GET")

	if err != nil {
		log.Fatal("Request failed: ", err)
	}

	log.Println("Response with JA4:", response.Status)
}
```

</details>

### How do I set/force HTTP1

<details>

In golang set `ForceHTTP1` in Options

```go
package main

import (
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"log"
)

func main() {
	client := cycletls.Init()
	response, err := client.Do("https://tls.peet.ws/api/all", cycletls.Options{
		ForceHTTP1: true,
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response.Body,) //You can verify the HTTP_Version in the response

}

```


In JS/TS set `forceHTTP1` in Options

```js
const initCycleTLS = require('cycletls');
// Typescript: import initCycleTLS from 'cycletls';

(async () => {
  const cycleTLS = await initCycleTLS();

  const response = await cycleTLS('https://ja3er.com/json', {
     body: '',
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent:
      'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    forceHTTP1: true, // Set this field to force HTTP/1.1
  });

  const data = await response.json();
  console.log(data);
  // You can verify the HTTP_Version in the response
  cycleTLS.exit();

})();

```

</details>

### Cross Compiling for other platforms
<details>

Natively the 3 Operating System types `linux`, `darwin` , `windows`  should cover most use cases.
	
You can use the built in Golang cross compiling commands `go build` to compile for another operating system. 

As an example for linux arm you need to pass in the `GOOS` and `GOARCH` arguments

```bash
$ GOOS=linux GOARCH=arm go build -o ./dist/index ./golang && chmod +x ./dist/index
```

With the above command you can simply run `./index` and CycleTLS should function as intended.

Use this [gist](https://gist.github.com/asukakenji/f15ba7e588ac42795f421b48b8aede63) for different Operating Systems that support cross-compilation and feel free to open an [Issue](https://github.com/Danny-Dasilva/CycleTLS/issues/new/choose) with a feature request for your specific operating system use case. 

</details>

## LICENSE
### GPL3 LICENSE SYNOPSIS

**_TL;DR_*** Here's what the GPL3 license entails:

```markdown
1. Anyone can copy, modify and distribute this software.
2. You have to include the license and copyright notice with each and every distribution.
3. You can use this software privately.
4. You can use this software for commercial purposes.
5. Source code MUST be made available when the software is distributed.
6. Any modifications of this code base MUST be distributed with the same license, GPLv3.
7. This software is provided without warranty.
8. The software author or license can not be held liable for any damages inflicted by the software.
```

More information on about the [LICENSE can be found here](http://choosealicense.com/licenses/gpl-3.0/)
