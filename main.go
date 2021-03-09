package main

import (
    "log"
	"fmt"
	"io/ioutil"
    "github.com/refraction-networking/utls"
    "./cclient"
)


func main() {
    client, err := cclient.NewClient(tls.HelloChrome_Auto)
    if err != nil {
        log.Fatal(err)
    }

    resp, err := client.Get("https://ja3er.com/json")
	if err != nil {
        fmt.Println(err)
		log.Fatal(err)
    } else {
        fmt.Println(resp.StatusCode)
        body, _ := ioutil.ReadAll(resp.Body)
        fmt.Println(string(body))
    }
}

