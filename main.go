package main

import (
	"log"
	"time"
	"./cycletls"
	"runtime"
	// "net/http"
)



func main() {
    runtime.GOMAXPROCS(runtime.NumCPU())

	start := time.Now()
    defer func() {
        log.Println("Execution Time: ", time.Since(start))
    }()
	client := cycletls.Init()
	response, err := client.Do("https://ja3er.com/json", cycletls.Options{
		Body : "",
		Ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		// Headers: map[string]string{"Cookie": "valu=5"},
		
		
	}, "GET");
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response)

	response, err = client.Do("https://http2.pro/api/v1", cycletls.Options{
		Body : "",
		Ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		// Headers: map[string]string{"Cookie": "valu=5"},
		
		
	}, "GET");
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response)
}
