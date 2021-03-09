# CClient

Fixes TLS and stuff.

# Example

```go
package main

import (
    "log"

    "github.com/refraction-networking/utls"
    "github.com/x04/cclient"
)

func main() {
    client, err := cclient.NewClient(tls.HelloChrome_Auto)
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