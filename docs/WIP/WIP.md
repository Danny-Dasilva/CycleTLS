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

- [High-performance](#-performance) event-loop under networking model of multiple threads/goroutines
-  Built-in goroutine pool powered by the library [ants](https://github.com/panjf2000/ants)
OR instead do an introduction or something

## Dependencies

```
node ^v8.0
golang ^v1.14
```



## Installation

```bash
$ npm install cycletls
```

Table of contents
=================


* [gh-md-toc](#gh-md-toc)
* [Table of contents](#table-of-contents)
* [Installation](#installation)
* [Usage](#usage)
	* [STDIN](#stdin)
	* [Local files](#local-files)
	* [Remote files](#remote-files)
	* [Multiple files](#multiple-files)
	* [Combo](#combo)
	* [Auto insert and update TOC](#auto-insert-and-update-toc)
	* [GitHub token](#github-token)
	* [TOC generation with Github Actions](#toc-generation-with-github-actions)
* [Tests](#tests)
* [Dependency](#dependency)
* [Docker](#docker)
	* [Local](#local)
	* [Public](#public)



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
      proxy: 'https://username:password@hostname.com:443'
    });

    response.then((out) => {
      console.log(out)
    })

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
```
docker build -t my_first_image .

docker run --name test my_first_image

docker exec -it my_first_image

docker run --name test \
--rm -it --privileged -p 6006:6006 \
my_first_image

docker run --name testing \
--rm -it --privileged -p 6006:6006 \
--mount type=bind,src=${DETECT_DIR},dst=/models/research/object_detection/images \
my_first_image


docker system prune -a

```
`npm install --dev`

`npm run build`

if windows

`npm run build:windows`

if linux

`npm run build:linux`

if mac

`npm run build:mac:`



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