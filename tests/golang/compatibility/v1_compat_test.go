package main

import (
	"fmt"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	// Test v1 default behavior - using Init() for chan Response  
	client := cycletls.Init(true)
	
	// Queue a request
	go func() {
		client.Queue("https://tls.peet.ws/api/clean", cycletls.Options{
			Body:      "",
			Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
			UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		}, "GET")
	}()
	
	// V1 default pattern: receive from RespChan which provides Response objects
	select {
	case response := <-client.RespChan:
		// This is the v1 default pattern - getting a Response struct directly
		fmt.Printf("V1 Default - Status: %d\n", response.Status)
		fmt.Printf("V1 Default - RequestID: %s\n", response.RequestID)
		fmt.Printf("V1 Default - FinalURL: %s\n", response.FinalUrl)
		
		// Users can access all Response fields directly
		if response.Status == 200 {
			fmt.Println("V1 default test: SUCCESS - Response received as struct")
		}
	}
	
	client.Close()
	fmt.Println("V1 default test completed")
}