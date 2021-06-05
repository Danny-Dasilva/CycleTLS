<div align="center">
	<img src="docs/media/Banner.svg" alt="CycleTLS"/>
	<br>
	
Currently a WIP and in Active development. See the ![Projects](https://github.com/Danny-Dasilva/CycleTLS/projects/1) Tab for more info 


More documentation coming soon, ![Changelog](https://github.com/Danny-Dasilva/CycleTLS/blob/main/%20CHANGELOG.md) provided as well

	
	
[![build](https://github.com/Danny-Dasilva/CycleTLS/actions/workflows/tests.yml/badge.svg?)](https://github.com/Danny-Dasilva/CycleTLS/actions/workflows/tests.yml) 
[![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?)](http://godoc.org/github.com/Danny-Dasilva/CycleTLS/cycletls) 
[![license](https://img.shields.io/github/license/Danny-Dasilva/CycleTLS.svg?)](https://github.com/Danny-Dasilva/CycleTLS/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Danny-Dasilva/CycleTLS/cycletls?)](https://goreportcard.com/report/github.com/Danny-Dasilva/CycleTLS/cycletls)
[![npm version](https://img.shields.io/npm/v/cycletls.svg?style=flat-square?)](https://www.npmjs.org/package/cycletls)
</div>

<!-- [![install size](https://packagephobia.now.sh/badge?p=cycletls)](https://packagephobia.now.sh/result?p=cycletls) -->
 <!-- [![Build Status](http://img.shields.io/travis/mmatczuk/go-http-tunnel.svg?branch=master)](https://travis-ci.com/Danny-Dasilva/CycleTLS/cycletls)  -->




If you have a API change or feature request feel free to open an Issue


## Dependencies

```
node ^v8.0
golang ^v1.14
```

## Installation

```bash
$ npm install cycletls
```


# Example for TS/JS

this is in tests/main.ts

see run.sh script for local testing

```ts

const initCycleTLS = require('cycletls');
// Typescript: import initCycleTLS from 'cycletls';

(async () => {
  const cycleTLS = await initCycleTLS();

    const response = cycleTLS('https://ja3er.com/json', {
      body: '',
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      proxy: 'http://username:password@hostname.com:443'
    }, 'get');

    response.then((out) => {
      console.log(out)
    })
	
    cycleTLS.exit()

})();

```


# Example for Golang

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



### Dev Setup

`npm install --dev`

`npm run build`

Windows

`npm run build:windows`

Linux

`npm run build:linux`

Mac

`npm run build:mac:`


