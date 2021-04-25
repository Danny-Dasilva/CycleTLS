package main

import (
	"log"
	"time"
	"./cycletls"
	"runtime"
)
func main() {
    runtime.GOMAXPROCS(runtime.NumCPU())

	start := time.Now()
    defer func() {
        log.Println("Execution Time: ", time.Since(start))
    }()
	client := cycletls.Init()

	// response := client.Do("https://www.newegg.com/product/api/MoreBuyingOptions?ParentItem=14-932-411&PageNum=1&PageSize=100&PageIndex=1", cycletls.Options{
	// 	Body : "",
	// 	Ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
	// 	UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
	//   }, "GET");

	response := client.Do("https://www.newegg.com/product/api/MoreBuyingOptions?ParentItem=14-932-411&PageNum=1&PageSize=100&PageIndex=1", cycletls.Options{
		Body : "",
		Ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
	  }, "GET");
	
	log.Println(response)
}
