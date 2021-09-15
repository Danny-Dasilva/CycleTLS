package main

import (
	"./cycletls"
	"log"
	"runtime"
	"time"
	// "net/http"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	start := time.Now()
	defer func() {
		log.Println("Execution Time: ", time.Since(start))
	}()
	client := cycletls.Init()
	response, err := client.Do("https://example.com", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53-10,65281-0-23-35-13-5-18-16-11-51-45-43-10-27-21,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		// Headers: map[string]string{"Cookie": "valu=5"},

	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response)

}
