# CycleTLS



<div align="center">
	<img src="docs/media/Banner.svg" alt="CycleTLS"/>
	<br>
	
Currently a WIP and in Active development. See the ![Projects](https://github.com/Danny-Dasilva/CycleTLS/projects/1) Tab for more info

	
	

![build](https://github.com/Danny-Dasilva/CycleTLS/actions/workflows/test_golang.yml/badge.svg)
[![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg)](http://godoc.org/github.com/Danny-Dasilva/CycleTLS/cycletls) 
[![license](https://img.shields.io/github/license/Danny-Dasilva/CycleTLS.svg)](https://github.com/Danny-Dasilva/CycleTLS/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Danny-Dasilva/CycleTLS/cycletls)](https://goreportcard.com/report/github.com/Danny-Dasilva/CycleTLS/cycletls)
[![npm version](https://img.shields.io/npm/v/axios.svg?style=flat-square)](https://www.npmjs.org/package/cycletls)
</div>

If you have a API change or feature request feel free to open an Issue



# 🚀 Features

- [High-performance](#-performance) Built-in goroutine pool used for handling asynchronous requests
- Custom header ordering via [fhttp](https://github.com/useflyent/fhttp)
- Proxy support
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
* [LICENSE](#license)



For any feature requests or API change requests, please feel free to open an issue.


## Dependencies

```
node ^v14.0
golang ^v1.16x
```

## Installation

```bash
$ npm install cycletls
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
    proxy: 'http://username:password@hostname.com:443'
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

**cycleTLS([url], config)**

**cycleTLS.get([url], config)**

**cycleTLS.delete([url], config)**

**cycleTLS.head([url], config)**

**cycleTLS.options([url], config)**

**cycleTLS.post([url], config)**

**cycleTLS.put([url], config)**

**cycleTLS.patch([url], config)**

If URL is not passed, one must be specified in the config.

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
  disableRedirect: true
  // Custom header order to send with request (This value will overwrite default header order)
  headerOrder: ["cache-control", "connection", "host"]
}
);

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
  }
}
);

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



# Dev Setup

If you would like to compile CycleTLS on your own, use the following commands:

Set module-aware mode

`go env -w  GO111MODULE=auto`

Install golang dependencies

`go get github.com/Danny-Dasilva/CycleTLS/cycletls`

install npm packages

`npm install`

### To recompile index.ts in the src folder

`npm run build`

### To recompile Golang files in the golang folder
Windows

`npm run build:windows`

Linux

`npm run build:linux`

Mac

`npm run build:mac:`


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