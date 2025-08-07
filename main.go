package main

import (
	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls" // imports locally per go.mod
	"log"
	"runtime"
	"time"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	start := time.Now()
	defer func() {
		log.Println("Execution Time: ", time.Since(start))
	}()
	client := cycletls.Init()
	response, err := client.Do("https://www.google.com", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17613,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response.Status)
}
