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
	response, err := client.Do("https://google.com", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response.Response.Body)

}
