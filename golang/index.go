package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Options sets CycleTLS client options
type Options struct {
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	Ja3             string            `json:"ja3"`
	UserAgent       string            `json:"userAgent"`
	Proxy           string            `json:"proxy"`
	Cookies         []Cookie          `json:"cookies"`
	Timeout         int               `json:"timeout"`
	DisableRedirect bool              `json:"disableRedirect"`
}

type cycleTLSRequest struct {
	RequestID string  `json:"requestId"`
	Options   Options `json:"options"`
}

//rename to request+client+options
type fullRequest struct {
	req     *http.Request
	client  http.Client
	options cycleTLSRequest
}

//TODO: rename this response struct
type respData struct {
	Status  int
	Body    string
	Headers map[string]string
}

//Response contains Cycletls response data
type Response struct {
	RequestID string
	Response  respData
}

//CycleTLS creates full request and response
type CycleTLS struct {
	ReqChan  chan fullRequest
	RespChan chan Response
}

func getWebsocketAddr() string {
	port, exists := os.LookupEnv("WS_PORT")

	var addr *string

	if exists {
		addr = flag.String("addr", "localhost:"+port, "http service address")
	} else {
		addr = flag.String("addr", "localhost:9112", "http service address")
	}
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/"}

	return u.String()
}

// ready Request
func processRequest(request cycleTLSRequest) (result fullRequest) {

	var browser = browser{
		JA3:       request.Options.Ja3,
		UserAgent: request.Options.UserAgent,
		Cookies:   request.Options.Cookies,
	}

	client, err := newClient(
		browser,
		request.Options.Timeout,
		request.Options.DisableRedirect,
		request.Options.UserAgent,
		request.Options.Proxy,
	)
	if err != nil {
		log.Fatal(err)
	}
	req, err := http.NewRequest(strings.ToUpper(request.Options.Method), request.Options.URL, strings.NewReader(request.Options.Body))
	if err != nil {
		log.Print(request.RequestID + "Request_Id_On_The_Left" + err.Error())
		return
	}
	for k, v := range request.Options.Headers {
		if k != "host" {
			req.Header.Set(k, v)
		}
	}
	return fullRequest{req: req, client: client, options: request}

}

func dispatcher(res fullRequest) (response Response, err error) {
	resp, err := res.client.Do(res.req)
	if err != nil {

		parsedError := parseError(err)

		headers := make(map[string]string)
		respData := respData{parsedError.StatusCode, parsedError.ErrorMsg + "-> \n" + string(err.Error()), headers}

		return Response{res.options.RequestID, respData}, nil //normally return error here
		// return response, err

	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Parse Bytes" + err.Error())
		return response, err
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

	respData := respData{resp.StatusCode, string(bodyBytes), headers}

	return Response{res.options.RequestID, respData}, nil

}

// Queue queues request in worker pool
func (client CycleTLS) Queue(URL string, options Options, Method string) {

	options.URL = URL
	options.Method = Method
	//TODO add timestamp to request
	opt := cycleTLSRequest{"n", options}
	response := processRequest(opt)
	client.ReqChan <- response
}

// Do creates a single request
func (client CycleTLS) Do(URL string, options Options, Method string) (response Response, err error) {

	options.URL = URL
	options.Method = Method
	opt := cycleTLSRequest{"cycleTLSRequest", options}

	res := processRequest(opt)
	response, err = dispatcher(res)
	if err != nil {
		log.Print("Request Failed: " + err.Error())
		return response, err
	}

	return response, nil
}

//TODO rename this

// Init starts the worker pool or returns a empty cycletls struct
func Init(workers ...bool) CycleTLS {

	if len(workers) > 0 && workers[0] {
		reqChan := make(chan fullRequest)
		respChan := make(chan Response)
		go workerPool(reqChan, respChan)
		log.Println("Worker Pool Started")

		return CycleTLS{ReqChan: reqChan, RespChan: respChan}
	}
	return CycleTLS{}

}

// Close closes channels
func (client CycleTLS) Close() {
	close(client.ReqChan)
	close(client.RespChan)

}

// Worker Pool
func workerPool(reqChan chan fullRequest, respChan chan Response) {
	//MAX
	for i := 0; i < 100; i++ {
		go worker(reqChan, respChan)
	}
}

// Worker
func worker(reqChan chan fullRequest, respChan chan Response) {
	for res := range reqChan {
		response, err := dispatcher(res)
		if err != nil {
			log.Print("Request Failed: " + err.Error())
		}
		respChan <- response
	}
}

func readSocket(reqChan chan fullRequest, c *websocket.Conn) {
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Print("Socket Error", err)
			continue
		}
		request := new(cycleTLSRequest)

		err = json.Unmarshal(message, &request)
		if err != nil {
			log.Print("Unmarshal Error", err)
			return
		}

		reply := processRequest(*request)

		reqChan <- reply
	}
}

func writeSocket(respChan chan Response, c *websocket.Conn) {
	for {
		select {
		case r := <-respChan:
			message, err := json.Marshal(r)
			if err != nil {
				log.Print("Marshal Json Failed" + err.Error())
				continue
			}
			err = c.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Print("Socket WriteMessage Failed" + err.Error())
				continue
			}

		}

	}
}

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())

	start := time.Now()
	defer func() {
		log.Println("Execution Time: ", time.Since(start))
	}()

	websocketAddress := getWebsocketAddr()
	c, _, err := websocket.DefaultDialer.Dial(websocketAddress, nil)
	if err != nil {
		log.Print(err)
		return
	}

	reqChan := make(chan fullRequest)
	respChan := make(chan Response)
	go workerPool(reqChan, respChan)

	go readSocket(reqChan, c)
	//run as main thread
	writeSocket(respChan, c)

}
