package main

import (
    "log"
	"fmt"
	"io/ioutil"
    "./cycletls"
)


// ChromeAuto mocks Chrome 78
var ChromeAuto = cycletls.Browser{
	JA3:       "769,47–53–5–10–49161–49162–49171–49172–50–56–19–4,0–10–11,23–24–25,0",
	UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36",
}

// SafariAuto mocks Safari 604.1
var SafariAuto = cycletls.Browser{
	JA3:       "771,4865-4866-4867-49196-49195-49188-49187-49162-49161-52393-49200-49199-49192-49191-49172-49171-52392-157-156-61-60-53-47-49160-49170-10,65281-0-23-13-5-18-16-11-51-45-43-10-21,29-23-24-25,0",
	UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 13_1_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.1 Mobile/15E148 Safari/604.1",
}

var FirefoxAuto = cycletls.Browser{
	JA3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
	UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:86.0) Gecko/20100101 Firefox/86.0",
}


func main() {

    client, err := cycletls.NewClient(FirefoxAuto)
    // client, err := cclient.NewClient(tls.HelloChrome_Auto)
    if err != nil {
        log.Fatal(err)
    }


    resp, err := client.Get("https://ja3er.com/json")
    // resp, err := client.Get("https://api.ipify.org/?format=json")
	if err != nil {
        fmt.Println(err)
		log.Fatal(err)
    } else {
        fmt.Println(resp.StatusCode)
        body, _ := ioutil.ReadAll(resp.Body)
        fmt.Println(string(body))
    }
}

