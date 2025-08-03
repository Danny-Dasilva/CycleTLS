package cycletls

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	nhttp "net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"

	http "github.com/Danny-Dasilva/fhttp"
	"github.com/gorilla/websocket"
)

// Options sets CycleTLS client options
type Options struct {
	URL                string            `json:"url"`
	Method             string            `json:"method"`
	Headers            map[string]string `json:"headers"`
	Body               string            `json:"body"`
	
	// TLS fingerprinting options
	Ja3                string            `json:"ja3"`
	Ja4                string            `json:"ja4"`
	HTTP2Fingerprint   string            `json:"http2Fingerprint"`
	QUICFingerprint    string            `json:"quicFingerprint"`
	
	// Browser identification
	UserAgent          string            `json:"userAgent"`
	
	// Connection options
	Proxy              string            `json:"proxy"`
	Cookies            []Cookie          `json:"cookies"`
	Timeout            int               `json:"timeout"`
	DisableRedirect    bool              `json:"disableRedirect"`
	HeaderOrder        []string          `json:"headerOrder"`
	OrderAsProvided    bool              `json:"orderAsProvided"` //TODO
	InsecureSkipVerify bool              `json:"insecureSkipVerify"`
	
	// Protocol options
	ForceHTTP1         bool              `json:"forceHTTP1"`
	ForceHTTP3         bool              `json:"forceHTTP3"`
	Protocol           string            `json:"protocol"` // "http1", "http2", "http3", "websocket", "sse"
}

type cycleTLSRequest struct {
	RequestID string  `json:"requestId"`
	Options   Options `json:"options"`
}

// rename to request+client+options
type fullRequest struct {
	req     *http.Request
	client  http.Client
	options cycleTLSRequest
}

// CycleTLS creates full request and response
type CycleTLS struct {
	ReqChan  chan fullRequest
	RespChan chan []byte
}

var activeRequests = make(map[string]context.CancelFunc)
var activeRequestsMutex sync.Mutex
var debugLogger = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)

// ready Request
func processRequest(request cycleTLSRequest) (result fullRequest) {
	ctx, cancel := context.WithCancel(context.Background())

	var browser = Browser{
		// TLS fingerprinting options
		JA3:                request.Options.Ja3,
		JA4:                request.Options.Ja4,
		HTTP2Fingerprint:   request.Options.HTTP2Fingerprint,
		QUICFingerprint:    request.Options.QUICFingerprint,
		
		// Browser identification
		UserAgent:          request.Options.UserAgent,
		
		// Connection options
		Cookies:            request.Options.Cookies,
		InsecureSkipVerify: request.Options.InsecureSkipVerify,
		ForceHTTP1:         request.Options.ForceHTTP1,
		ForceHTTP3:         request.Options.ForceHTTP3,
		
		// Header ordering
		HeaderOrder:        request.Options.HeaderOrder,
	}

	// Handle protocol-specific clients
	if request.Options.Protocol == "websocket" {
		// WebSocket requests are handled separately and will be implemented later
		log.Fatal("WebSocket support is not yet fully implemented")
	} else if request.Options.Protocol == "sse" {
		// SSE requests are handled separately and will be implemented later
		log.Fatal("SSE support is not yet fully implemented")
	} else if request.Options.Protocol == "http3" || request.Options.ForceHTTP3 {
		// HTTP/3 requests are handled separately and will be implemented later
		log.Fatal("HTTP/3 support is not yet fully implemented")
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

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(request.Options.Method), request.Options.URL, strings.NewReader(request.Options.Body))
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
	headerOrder := parseUserAgent(request.Options.UserAgent).HeaderOrder

	//ordering the pseudo headers and our normal headers
	req.Header = http.Header{
		http.HeaderOrderKey:  headerorderkey,
		http.PHeaderOrderKey: headerOrder,
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

	activeRequestsMutex.Lock()
	activeRequests[request.RequestID] = cancel
	activeRequestsMutex.Unlock()

	return fullRequest{req: req, client: client, options: request}
}

// func dispatcher(res fullRequest) (response Response, err error) {
// 	defer res.client.CloseIdleConnections()
// 	finalUrl := res.options.Options.URL
// 	resp, err := res.client.Do(res.req)
// 	if err != nil {

// 		parsedError := parseError(err)

// 		headers := make(map[string]string)
// 		var cookies []*nhttp.Cookie
// 		return Response{RequestID: res.options.RequestID, Status: parsedError.StatusCode, Body: parsedError.ErrorMsg + "-> \n" + string(err.Error()), Headers: headers, Cookies: cookies, FinalUrl: finalUrl}, nil //normally return error here

// 	}
// 	defer resp.Body.Close()

// 	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
// 		finalUrl = resp.Request.URL.String()
// 	}

// 	encoding := resp.Header["Content-Encoding"]
// 	content := resp.Header["Content-Type"]
// 	bodyBytes, err := io.ReadAll(resp.Body)

// 	if err != nil {
// 		log.Print("Parse Bytes" + err.Error())
// 		return response, err
// 	}

// 	Body := DecompressBody(bodyBytes, encoding, content)
// 	headers := make(map[string]string)

// 	for name, values := range resp.Header {
// 		if name == "Set-Cookie" {
// 			headers[name] = strings.Join(values, "/,/")
// 		} else {
// 			for _, value := range values {
// 				headers[name] = value
// 			}
// 		}
// 	}
// 	cookies := convertFHTTPCookiesToNetHTTPCookies(resp.Cookies())
// 	return Response{
// 		RequestID: res.options.RequestID,
// 		Status:    resp.StatusCode,
// 		Body:      Body,
// 		Headers:   headers,
// 		Cookies:   cookies,
// 		FinalUrl:  finalUrl,
// 	}, nil

// }

// // Queue queues request in worker pool
// func (client CycleTLS) Queue(URL string, options Options, Method string) {

// 	options.URL = URL
// 	options.Method = Method
// 	//TODO add timestamp to request
// 	opt := cycleTLSRequest{"Queued Request", options}
// 	response := processRequest(opt)
// 	client.ReqChan <- response
// }

// // Do creates a single request
// func (client CycleTLS) Do(URL string, options Options, Method string) (response Response, err error) {

// 	options.URL = URL
// 	options.Method = Method
// 	// Set default values if not provided
// 	if options.Ja3 == "" {
// 		options.Ja3 = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,18-35-65281-45-17513-27-65037-16-10-11-5-13-0-43-23-51,29-23-24,0"
// 	}
// 	if options.UserAgent == "" {
// 		// Mac OS Chrome 121
// 		options.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"
// 	}
// 	opt := cycleTLSRequest{"cycleTLSRequest", options}

// 	res := processRequest(opt)
// 	response, err = dispatcher(res)
// 	if err != nil {
// 		log.Print("Request Failed: " + err.Error())
// 		return response, err
// 	}

// 	return response, nil
// }

// Init starts the worker pool or returns a empty cycletls struct
// func Init(workers ...bool) CycleTLS {
// 	if len(workers) > 0 && workers[0] {
// 		reqChan := make(chan fullRequest)
// 		respChan := make(chan Response)
// 		go workerPool(reqChan, respChan)
// 		log.Println("Worker Pool Started")

// 		return CycleTLS{ReqChan: reqChan, RespChan: respChan}
// 	}
// 	return CycleTLS{}

// }

// // Close closes channels
// func (client CycleTLS) Close() {
// 	close(client.ReqChan)
// 	close(client.RespChan)

// }

// // Worker Pool
// func workerPool(reqChan chan fullRequest, respChan chan Response) {
// 	//MAX
// 	for i := 0; i < 100; i++ {
// 		go worker(reqChan, respChan)
// 	}
// }

// // Worker
// func worker(reqChan chan fullRequest, respChan chan Response) {
// 	for res := range reqChan {
// 		response, err := dispatcher(res)
// 		if err != nil {
// 			log.Print("Request Failed: " + err.Error())
// 		}
// 		respChan <- response
// 	}
// }

func dispatcherAsync(res fullRequest, chanWrite chan []byte) {
	defer func() {
		activeRequestsMutex.Lock()
		delete(activeRequests, res.options.RequestID)
		activeRequestsMutex.Unlock()
	}()

	// Extract host from URL for connection reuse tracking
	urlObj, _ := url.Parse(res.options.Options.URL)
	hostPort := urlObj.Host
	if !strings.Contains(hostPort, ":") {
		if urlObj.Scheme == "https" {
			hostPort = hostPort + ":443"  // Default HTTPS port
		} else {
			hostPort = hostPort + ":80"   // Default HTTP port
		}
	}
	
	// Don't close connections when finished - they'll be reused for the same host
	// Instead, tell the roundtripper to keep this connection but close others
	defer func() {
		// Use type assertion to access the roundTripper
		if transport, ok := res.client.Transport.(*roundTripper); ok {
			transport.CloseIdleConnections(hostPort)
		}
	}()
	
	finalUrl := res.options.Options.URL

	resp, err := res.client.Do(res.req)

	if err != nil {
		parsedError := parseError(err)

		{
			var b bytes.Buffer
			var requestIDLength = len(res.options.RequestID)

			b.WriteByte(byte(requestIDLength >> 8))
			b.WriteByte(byte(requestIDLength))
			b.WriteString(res.options.RequestID)
			b.WriteByte(0)
			b.WriteByte(5)
			b.WriteString("error")
			b.WriteByte(byte(parsedError.StatusCode >> 8))
			b.WriteByte(byte(parsedError.StatusCode))

			var message = parsedError.ErrorMsg + "-> \n" + string(err.Error())
			var messageLength = len(res.options.RequestID)

			b.WriteByte(byte(messageLength >> 8))
			b.WriteByte(byte(messageLength))
			b.WriteString(message)

			chanWrite <- b.Bytes()
		}

		return
	}

	defer resp.Body.Close()

	// Update finalUrl if redirect occurred
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		finalUrl = resp.Request.URL.String()
	}

	{
		var b bytes.Buffer
		var headerLength = len(resp.Header)
		var requestIDLength = len(res.options.RequestID)
		var finalUrlLength = len(finalUrl)

		b.WriteByte(byte(requestIDLength >> 8))
		b.WriteByte(byte(requestIDLength))
		b.WriteString(res.options.RequestID)
		b.WriteByte(0)
		b.WriteByte(8)
		b.WriteString("response")
		b.WriteByte(byte(resp.StatusCode >> 8))
		b.WriteByte(byte(resp.StatusCode))

		// Write finalUrl length and value
		b.WriteByte(byte(finalUrlLength >> 8))
		b.WriteByte(byte(finalUrlLength))
		b.WriteString(finalUrl)

		// Write headers
		b.WriteByte(byte(headerLength >> 8))
		b.WriteByte(byte(headerLength))

		for name, values := range resp.Header {
			var nameLength = len(name)
			var valuesLength = len(values)

			b.WriteByte(byte(nameLength >> 8))
			b.WriteByte(byte(nameLength))
			b.WriteString(name)
			b.WriteByte(byte(valuesLength >> 8))
			b.WriteByte(byte(valuesLength))

			for _, value := range values {
				var valueLength = len(value)

				b.WriteByte(byte(valueLength >> 8))
				b.WriteByte(byte(valueLength))
				b.WriteString(value)
			}
		}

		chanWrite <- b.Bytes()
	}

	{
		bufferSize := 8192
		chunkBuffer := make([]byte, bufferSize)

	loop:
		for {
			select {
			case <-res.req.Context().Done():
				debugLogger.Printf("Request %s was canceled during processing", res.options.RequestID)
				break loop

			default:
				n, err := resp.Body.Read(chunkBuffer)

				if res.req.Context().Err() != nil {
					debugLogger.Printf("Request %s was canceled during body read", res.options.RequestID)
					break loop
				}

				if err != nil && err != io.EOF {
					log.Printf("Read error: %s", err.Error())
					break loop
				}

				if n == 0 {
					break loop
				}

				var b bytes.Buffer
				requestIDLength := len(res.options.RequestID)
				bodyChunkLength := n

				b.WriteByte(byte(requestIDLength >> 8))
				b.WriteByte(byte(requestIDLength))
				b.WriteString(res.options.RequestID)
				b.WriteByte(0)
				b.WriteByte(4)
				b.WriteString("data")
				b.WriteByte(byte(bodyChunkLength >> 24))
				b.WriteByte(byte(bodyChunkLength >> 16))
				b.WriteByte(byte(bodyChunkLength >> 8))
				b.WriteByte(byte(bodyChunkLength))
				b.Write(chunkBuffer[:n])

				chanWrite <- b.Bytes()

				if err == io.EOF {
					break loop
				}
			}
		}
	}

	{
		var b bytes.Buffer
		requestIDLength := len(res.options.RequestID)

		b.WriteByte(byte(requestIDLength >> 8))
		b.WriteByte(byte(requestIDLength))
		b.WriteString(res.options.RequestID)
		b.WriteByte(0)
		b.WriteByte(3)
		b.WriteString("end")

		chanWrite <- b.Bytes()
	}
}

func writeSocket(chanWrite chan []byte, wsSocket *websocket.Conn) {
	for buf := range chanWrite {
		err := wsSocket.WriteMessage(websocket.BinaryMessage, buf)

		if err != nil {
			log.Print("Socket WriteMessage Failed" + err.Error())
			continue
		}
	}
}

func readSocket(chanRead chan fullRequest, wsSocket *websocket.Conn) {
	for {
		_, message, err := wsSocket.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return
			}
			log.Print("Socket Error", err)
			return
		}
		var baseMessage map[string]interface{}
		if err := json.Unmarshal(message, &baseMessage); err != nil {
			log.Print("Unmarshal Error", err)
			return
		}
		if action, ok := baseMessage["action"]; ok {
			if action == "exit" {
				// Respond by sending a close frame and then close the connection.
				wsSocket.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "exit"))
				wsSocket.Close()
				return
			}
			if action == "cancel" {
				requestId, _ := baseMessage["requestId"].(string)
				activeRequestsMutex.Lock()
				if cancel, exists := activeRequests[requestId]; exists {
					cancel()
					delete(activeRequests, requestId)
				}
				activeRequestsMutex.Unlock()
				continue
			}
		}
		// (If there was no "action" field, process as usual)
		request := new(cycleTLSRequest)
		if err := json.Unmarshal(message, &request); err != nil {
			log.Print("Unmarshal Error", err)
			return
		}
		chanRead <- processRequest(*request)
	}
}

// Worker
func readProcess(chanRead chan fullRequest, chanWrite chan []byte) {
	for request := range chanRead {
		go dispatcherAsync(request, chanWrite)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WSEndpoint exports the main cycletls function as we websocket connection that clients can connect to
func WSEndpoint(w nhttp.ResponseWriter, r *nhttp.Request) {
	upgrader.CheckOrigin = func(r *nhttp.Request) bool { return true }

	// upgrade this connection to a WebSocket
	// connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		//Golang Received a non-standard request to this port, printing request
		var data map[string]interface{}
		bodyBytes, err := io.ReadAll(r.Body)
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
		chanRead := make(chan fullRequest)
		chanWrite := make(chan []byte)

		go readSocket(chanRead, ws)
		go readProcess(chanRead, chanWrite)

		// Run as main thread
		writeSocket(chanWrite, ws)
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
