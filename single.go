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

	response := client.Do("https://httpbin.org/cookies", cycletls.Options{
		Body : "",
		Ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		// Headers: map[string]string{"Cookie": "valu=5"},
		
		Cookies: []cycletls.Cookie{ 
			cycletls.Cookie{
				Name: "a", 
				Value: "multiple", 
				Path: "/",
				Domain: ".google.com",
				Expires: time.Now(),
				MaxAge: 1,
				HttpOnly: true,
				Secure: true,
				// SameSite: true,
			},
			cycletls.Cookie{
				Name: "test2", 
				Value: "b-option", 
				Expires: time.Now(),
			},
		},
	}, "GET");
	
	log.Println(response)
}
