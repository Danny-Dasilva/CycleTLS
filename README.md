# CClient

Fixes TLS and stuff.

# Example

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