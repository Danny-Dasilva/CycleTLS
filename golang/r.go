package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"runtime"

	"github.com/gorilla/websocket"
	// "golang.org/x/text/message"

	// "strings"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)




type myTLSRequest struct {
	RequestID string `json:"requestId"`
	Options   struct {
		URL     string            `json:"url"`
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
		Ja3     string            `json:"ja3"`
		UserAgent     string       `json:"userAgent"`
		ID     int            		`json:"id"`
		Proxy   string            `json:"proxy"`
	} `json:"options"`
}



type Result struct {
    req *http.Request
    client http.Client
}

type response struct {
	Status  int
	Body    string
	// Headers map[string]string
}

type myTLSResponse struct {
	RequestID string
	Response  response
}

func getWebsocketAddr() string {
	port, exists := os.LookupEnv("WS_PORT")
	fmt.Println(port)
	var addr *string

	if exists {
		addr = flag.String("addr", "localhost:"+port, "http service address")
	} else {
		addr = flag.String("addr", "localhost:9112", "http service address")
	}

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/"}

	return u.String()
}





// Dispatcher
func dispatcher(reqChan chan Result, socket *websocket.Conn) {
    defer close(reqChan)
    for {
		_, message, err := socket.ReadMessage()
		if err != nil {
			log.Print(err)
			continue
		}
		

		mytlsrequest := new(myTLSRequest)
		e := json.Unmarshal(message, &mytlsrequest)
		if e != nil {
			log.Print(err)
			continue
		}


		var Default = Browser{
			JA3:       mytlsrequest.Options.Ja3,
			UserAgent:  mytlsrequest.Options.UserAgent,
		}
		



		client, err := NewClient(Default, mytlsrequest.Options.Proxy)
		// client, err := cclient.NewClient(tls.HelloChrome_Auto)
		if err != nil {
			log.Fatal(err)
		}

		

		req, err := http.NewRequest(strings.ToUpper(mytlsrequest.Options.Method), mytlsrequest.Options.URL, strings.NewReader(mytlsrequest.Options.Body))
		if err != nil {
			log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
			continue
		}

		for k, v := range mytlsrequest.Options.Headers {
			if k != "host" {
				req.Header.Set(k, v)
			}
		}
		

		res := new(Result)
        res.client = client
        res.req = req
        reqChan <- *res
    }
}



var greeting *websocket.Conn


// Worker Pool
func workerPool(reqChan chan Result, respChan chan []byte) {
	//MAX
    for i := 0; i < 100; i++ {
        go worker(reqChan, respChan)
    }
}

// Worker
func worker(reqChan chan Result, respChan chan []byte) {
    for res := range reqChan {
        resp, err := res.client.Do(res.req)
		if err != nil {
			log.Print("Request_Id_On_The_Left" + err.Error())
			continue
		}

		defer resp.Body.Close()
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Print("Request_Id_On_The_Left" + err.Error())
			continue
		}

		headers := make(map[string]string)

		for name, values := range resp.Header {
			if name == "Set-Cookie" {
				headers[name] = strings.Join(values, "/,/")
			} else {
				for _, value := range values {
					headers[name] = value
				}
			}
		}

		Response := response{resp.StatusCode, string(bodyBytes)}
		//mytls request id
		reply := myTLSResponse{"i", Response}

		data, err := json.Marshal(reply)
		if err != nil {
			log.Print("Request_Id_On_The_Left" + err.Error())
			continue
		}



        respChan <- data
    }
}

func main() {
	start := time.Now()
    defer func() {
        fmt.Println("Execution Time: ", time.Since(start))
    }()


    

	

	
    runtime.GOMAXPROCS(runtime.NumCPU())
    
	// runtime.GOMAXPROCS(1)
	reqChan := make(chan Result)
    respChan := make(chan []byte)
    // go dispatcher(reqChan)
    go workerPool(reqChan, respChan)
    
    // go dispatcher(reqChan, c)

    requests := 10000
	ch := make(chan Result)
	go func(ch chan Result) {
        for i := 0; i < requests; i++ {
		mytlsrequest := new(myTLSRequest)
				mytlsrequest.RequestID = string('t')
				mytlsrequest.Options.URL = "http://localhost:8080"
				mytlsrequest.Options.Method = "GET"
				mytlsrequest.Options.Headers = map[string]string{
											"user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36",

												}
				
				mytlsrequest.Options.Ja3 = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0"




		var Default = Browser{
			JA3:       mytlsrequest.Options.Ja3,
			UserAgent:  mytlsrequest.Options.UserAgent,
		}
		



		client, err := NewClient(Default, mytlsrequest.Options.Proxy)
		// client, err := cclient.NewClient(tls.HelloChrome_Auto)
		if err != nil {
			log.Fatal(err)
		}

		

		req, err := http.NewRequest(strings.ToUpper(mytlsrequest.Options.Method), mytlsrequest.Options.URL, strings.NewReader(mytlsrequest.Options.Body))
		if err != nil {
			log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
			continue
		}

		for k, v := range mytlsrequest.Options.Headers {
			if k != "host" {
				req.Header.Set(k, v)
			}
		}
		

		res := new(Result)
        res.client = client
        res.req = req
        ch <- *res
    }
	}(ch)

	
    var (
        conns int64

    )
    for stdin := range ch {
		
        reqChan <- stdin
    }

    for conns < int64(requests) {

        select {
        case message := <-respChan:
            // fmt.Println(message)
            _ = message
            conns++
			fmt.Println(conns)
        }
		fmt.Println(conns)
       
    }
}



