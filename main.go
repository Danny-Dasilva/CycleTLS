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
	response, err := client.Do("https://ja3er.com/json", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865,0-23-65281-10-11-35-16-5-51-43,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:91.0) Gecko/20100101 Firefox/91.0",
		// Headers: map[string]string{"Cookie": "valu=5"},

	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response)

}
