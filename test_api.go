package main

import (
	"fmt"
	"time"
	// "encoding/json"
	"./cycletls"
	"runtime"
)

var FirefoxAuto = cycletls.Browser{
	JA3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
	UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:86.0) Gecko/20100101 Firefox/86.0",
}

// func main() {

   
// 	cycleTLS := cycletls.Init()

// 	msg := cycleTLS.Call("http://localhost:8080", cycletls.Request{
// 		Body : "",
// 		Ja3: "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
// 		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36",
// 	  }, "GET");

// 	fmt.Println("yeet", msg)


// }

type response struct {
	Status  int
	Body    string
	Headers map[string]string
}

type myTLSResponse struct {
	RequestID string
	Response  response
}

func main() {
    runtime.GOMAXPROCS(runtime.NumCPU())

	start := time.Now()
    defer func() {
        fmt.Println("Execution Time: ", time.Since(start))
    }()
	reqs := 10000
   
	client := cycletls.Init(true)


	go func() {
	for i := 0; i < reqs; i++ {
	client.Queue("http://localhost:8081", cycletls.Options{
		Body : "",
		Ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
	  }, "GET");
	}
	}()
	var (
	conns int64

    )
	
	for conns < int64(reqs) {

		select {
		case elem := <-client.RespChan:
			_ = elem
			conns++
		}
    }
}
