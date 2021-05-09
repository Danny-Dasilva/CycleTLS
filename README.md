# CycleTLS

Currently in Active development see Projects Tab

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

`npm install --dev`

`npm run build`

if windows

`npm run build:windows`

if linux

`npm run build:linux`

if mac

`npm run build:mac:`
