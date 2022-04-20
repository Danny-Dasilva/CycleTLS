package cycletls

import (
	"encoding/json"
	"flag"
	http "github.com/Danny-Dasilva/fhttp"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	nhttp "net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
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
	HeaderOrder     []string          `json:"headerOrder"`
	OrderAsProvided bool              `json:"orderAsProvided"` //TODO
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

//Response contains Cycletls response data
type Response struct {
	RequestID string
	Status    int
	Body      string
	Headers   map[string]string
}

//JSONBody converts response body to json
func (re Response) JSONBody() map[string]interface{} {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(re.Body), &data)
	if err != nil {
		log.Print("Json Conversion failed " + err.Error() + re.Body)
	}
	return data
}

//CycleTLS creates full request and response
type CycleTLS struct {
	ReqChan  chan fullRequest
	RespChan chan Response
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
		log.Fatal(err)
	}
	headerorder := []string{}
	//master header order, all your headers will be ordered based on this list and anything extra will be appended to the end
	//if your site has any custom headers, see the header order chrome uses and then add those headers to this list
	if len(request.Options.HeaderOrder) > 0 {
		//lowercase headers
		for _, v := range request.Options.HeaderOrder {
			lowercasekey := strings.ToLower(v)
			headerorder = append(headerorder, lowercasekey)
		}
	} else {
		headerorder = append(headerorder,
			"host",
			"connection",
			"cache-control",
			"device-memory",
			"viewport-width",
			"rtt",
			"downlink",
			"ect",
			"sec-ch-ua",
			"sec-ch-ua-mobile",
			"sec-ch-ua-full-version",
			"sec-ch-ua-arch",
			"sec-ch-ua-platform",
			"sec-ch-ua-platform-version",
			"sec-ch-ua-model",
			"upgrade-insecure-requests",
			"user-agent",
			"accept",
			"sec-fetch-site",
			"sec-fetch-mode",
			"sec-fetch-user",
			"sec-fetch-dest",
			"referer",
			"accept-encoding",
			"accept-language",
			"cookie",
		)
	}

	headermap := make(map[string]string)
	//TODO: Shorten this
	headerorderkey := []string{}
	for _, key := range headerorder {
		for k, v := range request.Options.Headers {
			lowercasekey := strings.ToLower(k)
			if key == lowercasekey {
				headermap[k] = v
				headerorderkey = append(headerorderkey, lowercasekey)
			}
		}

	}

	//ordering the pseudo headers and our normal headers
	req.Header = http.Header{
		http.HeaderOrderKey:  headerorderkey,
		http.PHeaderOrderKey: {":method", ":authority", ":scheme", ":path"},
	}
	//set our Host header
	u, err := url.Parse(request.Options.URL)
	if err != nil {
		panic(err)
	}

	//append our normal headers
	for k, v := range request.Options.Headers {
		if k != "Content-Length" {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("Host", u.Host)
	req.Header.Set("user-agent", request.Options.UserAgent)
	return fullRequest{req: req, client: client, options: request}

}

func dispatcher(res fullRequest) (response Response, err error) {
	resp, err := res.client.Do(res.req)
	if err != nil {

		parsedError := parseError(err)

		headers := make(map[string]string)
		return Response{res.options.RequestID, parsedError.StatusCode, parsedError.ErrorMsg + "-> \n" + string(err.Error()), headers}, nil //normally return error here

	}
	defer resp.Body.Close()

	encoding := resp.Header["Content-Encoding"]
	content := resp.Header["Content-Type"]

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Parse Bytes" + err.Error())
		return response, err
	}

	Body := DecompressBody(bodyBytes, encoding, content)
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
	return Response{res.options.RequestID, resp.StatusCode, Body, headers}, nil

}

// Queue queues request in worker pool
func (client CycleTLS) Queue(URL string, options Options, Method string) {

	options.URL = URL
	options.Method = Method
	//TODO add timestamp to request
	opt := cycleTLSRequest{"Queued Request", options}
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
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return
			}
			log.Print("Socket Error", err)
			return
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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func WSEndpoint(w nhttp.ResponseWriter, r *nhttp.Request) {
	upgrader.CheckOrigin = func(r *nhttp.Request) bool { return true }

	// upgrade this connection to a WebSocket
	// connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		//Golang Received a non-standard request to this port, printing request
		var data map[string]interface{}
		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Print("Invalid Request: Body Read Error" + err.Error())
		}
		err = json.Unmarshal(bodyBytes, &data)
		if err != nil {
			log.Print("Invalid Request: Json Conversion failed ")
		}
		body, err := PrettyStruct(data)
		if err != nil {
			log.Print("Invalid Request:", err)
		}
		headers, err := PrettyStruct(r.Header)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(headers)
		log.Println(body)
	} else {
		reqChan := make(chan fullRequest)
		respChan := make(chan Response)
		go workerPool(reqChan, respChan)

		go readSocket(reqChan, ws)
		//run as main thread
		writeSocket(respChan, ws)

	}

}

func setupRoutes() {
	nhttp.HandleFunc("/", WSEndpoint)
}

func main() {
	port, exists := os.LookupEnv("WS_PORT")
	var addr *string
	if exists {
		addr = flag.String("addr", ":"+port, "http service address")
	} else {
		addr = flag.String("addr", ":9112", "http service address")
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	setupRoutes()
	log.Fatal(nhttp.ListenAndServe(*addr, nil))
}
