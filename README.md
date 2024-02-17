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
* [Local Setup](#dev-setup)
* [QA](#questions)
* [LICENSE](#license)

## Dependencies

```
node ^v16.0
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

  console.log(response);

  // Cleanly exit CycleTLS
  cycleTLS.exit();

})();

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

	response, err := client.Do("https://ja3er.com/json", cycletls.Options{
		Body : "",
		Ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
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
  // User agent for request
  userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
  // Proxy to send request through (must be in the same format)
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

// Defining custom JA3 token and user agenton multiple requests,
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

// Promises array of requests
const promises = [];

// Anonymous async function
(async () => {
  // Initiate CycleTLS
  const cycleTLS = await initCycleTLS();

  // Loop through requestDict (Object) defined above
  for (const url in requestDict) {
    // Fetch configs from requestDict (Object)
    const params = requestDict[url];

    // Send request (note: no waiting)
    const response = cycleTLS(
      url, {
        body: params.body ?? "", //?? is just setting defaults in this case
        ja3: params.ja3 ?? ja3,
        userAgent: params.userAgent ?? userAgent,
        headers: params.headers,
        cookies: params.cookies,
      }, params.method ?? "GET");

    // console.log the response object
    response.then((out) => {
      console.log(url, out);
    });

    // Push request to promise array
    promises.push(response);
  }

  // Wait for all requests to execute successfully
  Promise.all(promises).then(() => {
    // Cleanly exit CycleTLS one all requests have been received
    cycleTLS.exit();
  });
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
  console.log(response.body);
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

  console.log(response.body);
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
    
    //Altenatively if you want access to values within a map
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
  
  //verify cookies were set
  console.log(secondResponse.body)
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
      headers: {
          'Content-Type': 'multipart/form-data',
      },
  }, 'post');

  console.log(response);

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
      headers: {
          'Content-Type': 'multipart/form-data',
      },
  }, 'post');

  console.log(response);

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

  console.log(response);

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
  //Write Image
  writeImage("test.jpeg", jpegImage.body);

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
    forceHTTP1: false, //Set this field
  });

  console.log(response);
  //You can verify the HTTP_Version in the response
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
