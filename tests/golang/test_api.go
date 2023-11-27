package main

import (
	"fmt"
	"time"
	// "encoding/json"
	"./cycletls"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	start := time.Now()
	defer func() {
		fmt.Println("Execution Time: ", time.Since(start))
	}()
	reqs := 2

	client := cycletls.Init(true)

	go func() {
		for i := 0; i < reqs; i++ {
			client.Queue("https://tls.peet.ws/api/clean", cycletls.Options{
				Body:      "",
				Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
				UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
			}, "GET")
		}
	}()
	var (
		conns int64
	)

	for conns < int64(reqs) {

		select {
		case elem := <-client.RespChan:
			_ = elem
			fmt.Println(elem, "elem")
			conns++
		}
	}

	client.Close()
	fmt.Println("closed")
}
