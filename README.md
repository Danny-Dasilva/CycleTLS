# CycleTLS

Currently in Active development 

todo: 
- [X] fix TS api 
- [X] redo-socket request api in go(now fully async)
- [X] test requests 
- [ ] cleanup async code // close port and exit once process is finished
- [ ] test all error checks(wrong https proxy, incorrect JA3 and so on)
- [ ] write test files for golang
- [ ] concurrency and speedtests
- [ ] release for npm
- [ ] Add support for cookies

### Dev Setup

`npm install --dev`

`npm run build`

if windows

`npm run build:windows`

if linux

`npm run build:linux`

if mac

`npm run build:mac:`

# Example for Golang

```go
package main

import (
    "log"
    "github.com/Danny-Dasilva/cycletls"
)
var FirefoxAuto = cycletls.Browser{
	JA3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
	UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:86.0) Gecko/20100101 Firefox/86.0",
}

func main() {

    // Optional client, err := cycletls.NewClient(FirefoxAuto, "https://username:password@hostname.com:443")
    client, err := cycletls.NewClient(FirefoxAuto,)
    if err != nil {
        log.Fatal(err)
    }

    resp, err := client.Get("https://www.google.com/")
    if err != nil {
        log.Fatal(err)
    }
    resp.Body.Close()

    log.Println(resp.Status)
}
```


# Example for TS 

this is in tests/main.ts

see run.sh script for local testing

```ts
// const initCycleTLS = require('./src/index');
import initCycleTLS from '../dist/index.js'
// Typescript: import initCycleTLS from 'cycletls';

(async () => {
  const cycleTLS = await initCycleTLS();

  const response = await cycleTLS('https://ja3er.com/json', {
    body: '',
    ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0',
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36',
    proxy: 'https://username:password@hostname.com:443',
  });
  console.log(response)
})();

```
