package main

import (
    "log"
	"fmt"
	"io/ioutil"
    "github.com/refraction-networking/utls"
    "./cclient"
)

type Browser struct {
	JA3       string
	UserAgent string
}

// ChromeAuto mocks Chrome 78
var ChromeAuto = Browser{
	JA3:       "769,47–53–5–10–49161–49162–49171–49172–50–56–19–4,0–10–11,23–24–25,0",
	UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36",
}

// SafariAuto mocks Safari 604.1
var SafariAuto = Browser{
	JA3:       "771,4865-4866-4867-49196-49195-49188-49187-49162-49161-52393-49200-49199-49192-49191-49172-49171-52392-157-156-61-60-53-47-49160-49170-10,65281-0-23-13-5-18-16-11-51-45-43-10-21,29-23-24-25,0",
	UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 13_1_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.1 Mobile/15E148 Safari/604.1",
}


func main() {

    client, err := cclient.NewClient(tls.HelloChrome_Auto, "https://nTnzbu6i:QFmCfns3sxI3LsT5yhrYORq791Onx3Aqd491p6bFgn0CXR6B2GM0boie7jKdxz41Th5VA-wSyEvMWRA6@usa.resi.proxies.group:14584")
    // client, err := cclient.NewClient(tls.HelloChrome_Auto)
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

