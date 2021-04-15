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
	mytlsrequest myTLSRequest
}

type response struct {
	Status  int
	Body    string
	Headers map[string]string
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
func dispatcher(ch chan Result, socket *websocket.Conn) {
    // defer close(ch)
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
		
		
        ch <- Result{req: req, client: client, mytlsrequest: *mytlsrequest}
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
			fmt.Println(name, values)
			if name == "Set-Cookie" {
				headers[name] = strings.Join(values, "/,/")
			} else {
				for _, value := range values {
					headers[name] = value
				}
			}
		}

		Response := response{resp.StatusCode, string(bodyBytes), headers}
		//mytls request id
		reply := myTLSResponse{res.mytlsrequest.RequestID, Response}

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


    

	websocketAddress := getWebsocketAddr()

	c, _, err := websocket.DefaultDialer.Dial(websocketAddress, nil)
	if err != nil {
		log.Print(err)
		return
	}
	

	
    runtime.GOMAXPROCS(runtime.NumCPU())
    
	// runtime.GOMAXPROCS(1)
	reqChan := make(chan Result)
    respChan := make(chan []byte)
    // go dispatcher(reqChan)
    go workerPool(reqChan, respChan)
    
    // go dispatcher(reqChan, c)


	ch := make(chan Result)

	go func(ch chan Result) {
		for {
			_, message, err := c.ReadMessage()
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
			

			ch <- Result{req: req, client: client, mytlsrequest: *mytlsrequest}
		}
	}(ch)

	for {


	
		select {
        case stdin := <-ch:
			reqChan <- stdin
           
        default:
            // Do something when there is nothing read from stdin
        }
		
		select {
        case message := <-respChan:
			err = c.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Print("Request_Id_On_The_Left" )
				
			}
        default:
			
        }

	
	}
}



