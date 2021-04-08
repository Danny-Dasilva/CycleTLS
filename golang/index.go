package main

import (
	"encoding/json"
	"flag"
	"github.com/gorilla/websocket"
	// "io/ioutil"
	"runtime"
	"strings"
	"log"
	"net/http"
	"io/ioutil"
	"net/url"
	"os"
	"fmt"

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





type response struct {
	Status  int
	Body    string
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

// /////////////////////
func process(job []byte, i int, link chan<- []byte) {

	message := job
	mytlsrequest := new(myTLSRequest)
	e := json.Unmarshal(message, &mytlsrequest)
	if e != nil {
		log.Print(e)
	}


	var Default = Browser{
		JA3:       mytlsrequest.Options.Ja3,
		UserAgent:  mytlsrequest.Options.UserAgent,
	}
	fmt.Println(Default)



	client, err := NewClient(Default, mytlsrequest.Options.Proxy)
	// client, err := cclient.NewClient(tls.HelloChrome_Auto)
	if err != nil {
		log.Fatal(err)
	}



	req, err := http.NewRequest(strings.ToUpper(mytlsrequest.Options.Method), mytlsrequest.Options.URL, strings.NewReader(mytlsrequest.Options.Body))
	if err != nil {
		log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
	}

	for k, v := range mytlsrequest.Options.Headers {
		if k != "host" {
			req.Header.Set(k, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
		
	}

	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
		
	}

	

	Response := response{resp.StatusCode, string(bodyBytes)}

	reply := myTLSResponse{mytlsrequest.RequestID, Response}

	data, err := json.Marshal(reply)
	if err != nil {
		log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
		
	}
	
	link <- data
	
	
	

	
	
}

var greeting *websocket.Conn

func worker(jobChan <-chan  []byte, i int, link chan<- []byte) {
	for job := range jobChan {
		process(job,i, link)
	}
}
var m = map[string][]byte{}

func main() {
	flag.Parse()
	log.SetFlags(0)

	runtime.GOMAXPROCS(runtime.NumCPU())


	websocketAddress := getWebsocketAddr()

	c, _, err := websocket.DefaultDialer.Dial(websocketAddress, nil)
	if err != nil {
		log.Print(err)
		return
	}
	greeting = c

	workerCount := 100
	// make a channel with a capacity of 100.
	jobChan := make(chan []byte, 100) // Or jobChan := make(chan int)
	// done := make(chan bool)
	link := make(chan []byte)
	// start the worker
	for i:=0; i<workerCount; i++ {
		go worker(jobChan, i, link)
	}
	
	ch := make(chan []byte)
	
    go func(ch chan  []byte) {
        for {
            _, message, err := c.ReadMessage()
            if err != nil { 
                close(ch)
                return
            }
            ch <- message
        }
        
       
    }(ch)


	for {


	


		select {
        case stdin, ok := <-ch:
            if !ok {
                break 
            } else {
				jobChan <- stdin
            }
        default:
            // Do something when there is nothing read from stdin
        }
	
		
		
		select {
        case message := <-link:
			err = c.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Print("Request_Id_On_The_Left" )
				
			}
        default:

        }

	
	}
}




