package main

import (
	"encoding/json"
	"fmt"
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	// Test v2 performance pattern - using WithRawBytes() option for chan []byte
	client := cycletls.Init(cycletls.WithRawBytes())
	
	// Queue a request
	go func() {
		client.Queue("https://tls.peet.ws/api/clean", cycletls.Options{
			Body:      "",
			Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
			UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		}, "GET")
	}()
	
	// Performance pattern: receive from RespChanV2 which provides []byte
	select {
	case respBytes := <-client.RespChanV2:
		// This is the performance pattern - getting raw bytes for better performance
		var response cycletls.Response
		if err := json.Unmarshal(respBytes, &response); err != nil {
			log.Fatal("Failed to unmarshal response:", err)
		}
		
		fmt.Printf("V2 Performance - Status: %d\n", response.Status)
		fmt.Printf("V2 Performance - RequestID: %s\n", response.RequestID)
		fmt.Printf("V2 Performance - FinalURL: %s\n", response.FinalUrl)
		
		if response.Status == 200 {
			fmt.Println("Performance test: SUCCESS - Response received as bytes")
		}
	
	// Alternative: can still use v1 pattern with WithRawBytes() option
	case response := <-client.RespChan:
		fmt.Printf("V1 Fallback - Status: %d\n", response.Status)
		fmt.Printf("V1 Fallback - RequestID: %s\n", response.RequestID)
		fmt.Printf("V1 Fallback - FinalURL: %s\n", response.FinalUrl)
		
		if response.Status == 200 {
			fmt.Println("V1 fallback test: SUCCESS - Response received as struct")
		}
	}
	
	client.Close()
	fmt.Println("Performance test completed")
}