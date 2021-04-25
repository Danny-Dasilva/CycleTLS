package cycletls

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



type Options struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Ja3     string            `json:"ja3"`
	UserAgent     string      `json:"userAgent"`
	Proxy   string            `json:"proxy"`  
	Cookies []Cookie     `json:"cookies"`  
}


type cycleTLSRequest struct {
	RequestID string `json:"requestId"`
	Options   Options `json:"options"`
}


//rename to request+client+options
type fullRequest struct {
    req *http.Request
    client http.Client
	options cycleTLSRequest
}

type Response struct {
	Status  int
	Body    string
	Headers map[string]string
}

type cycleTLSResponse struct {
	RequestID string
	Response  Response
}
type cycleTLS struct {
	ReqChan chan fullRequest
    RespChan chan cycleTLSResponse
}
// Time wraps time.Time overriddin the json marshal/unmarshal to pass
// timestamp as integer
type Time struct {
	time.Time
}


// A Cookie represents an HTTP cookie as sent in the Set-Cookie header of an
// HTTP response or the Cookie header of an HTTP request.
//
// See https://tools.ietf.org/html/rfc6265 for details.
//Stolen from Net/http/cookies 
type Cookie struct {
	Name  string           `json:"name"` 
	Value string		   `json:"value"` 

	Path       string      `json:"path"` // optional
	Domain     string      `json:"domain"` // optional
	Expires    time.Time   `json:"expires"` // optional
	RawExpires string      `json:"rawExpires"`// for reading cookies only

	// MaxAge=0 means no 'Max-Age' attribute specified.
	// MaxAge<0 means delete cookie now, equivalently 'Max-Age: 0'
	// MaxAge>0 means Max-Age attribute present and given in seconds
	MaxAge   int           `json:"maxAge"`
	Secure   bool          `json:"secure"`
	HttpOnly bool          `json:"httpOnly"`
	SameSite http.SameSite `json:"sameSite"`
	Raw      string
	Unparsed []string      `json:"unparsed"` // Raw text of unparsed attribute-value pairs
	Time Time `json:"time"`
}



// UnmarshalJSON implements json.Unmarshaler inferface.
func (t *Time) UnmarshalJSON(buf []byte) error {
	// Try to parse the timestamp integer
	ts, err := strconv.ParseInt(string(buf), 10, 64)
	if err == nil {
		if len(buf) == 19 {
			t.Time = time.Unix(ts/1e9, ts%1e9)
		} else {
			t.Time = time.Unix(ts, 0)
		}
		return nil
	}
	// Try the default unmarshal
	if err := json.Unmarshal(buf, &t.Time); err == nil {
		return nil
	}
	str := strings.Trim(string(buf), `"`)
	if str == "null" || str == "" {
		return nil
	}
	// Try to manually parse the data
	tt, err := ParseDateString(str)
	if err != nil {
		return err
	}
	t.Time = tt
	return nil
}



// ParseDateString takes a string and passes it through Approxidate
// Parses into a time.Time
func ParseDateString(dt string) (time.Time, error) {
	
	const layout = "Mon, 02-Jan-2006 15:04:05 MST"
  
	return time.Parse(layout, dt)
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
   
	var browser = Browser{
		JA3:        request.Options.Ja3,
		UserAgent:  request.Options.UserAgent,
		Cookies:    request.Options.Cookies,
	}
	
	client, err := NewClient(browser, request.Options.Proxy)
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




func dispatcher(res fullRequest) (response cycleTLSResponse){ 
	resp, err := res.client.Do(res.req)
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Parse Bytes" + err.Error())
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
	
	Response := Response{resp.StatusCode, string(bodyBytes), headers}

	return cycleTLSResponse{res.options.RequestID, Response}

}

func (client cycleTLS) Queue(URL string, options Options, Method string) {

	options.URL = URL

	opt := cycleTLSRequest{"n", options}
	response := processRequest(opt)
	client.ReqChan <-response
	return 
}

func (client cycleTLS) Do(URL string, options Options, Method string) (response cycleTLSResponse) {

	options.URL = URL

	opt := cycleTLSRequest{"n", options}
	
	res := processRequest(opt)
	response = dispatcher(res)

	return 
}

func Init(workers ...bool) *cycleTLS {
	
	if len(workers) > 0 && workers[0] == true {
		reqChan := make(chan fullRequest)
    	respChan := make(chan cycleTLSResponse)
		go workerPool(reqChan, respChan)
		log.Println("Worker Pool Started")

		return &cycleTLS{ReqChan : reqChan, RespChan : respChan}
	} else {
		return &cycleTLS{}
	}
    
}


func (client cycleTLS) Close() {
	close(client.ReqChan)
	close(client.RespChan)

}

// Worker Pool
func workerPool(reqChan chan fullRequest, respChan chan cycleTLSResponse) {
	//MAX
    for i := 0; i < 100; i++ {
        go worker(reqChan, respChan)
    }
}


// Worker
func worker(reqChan chan fullRequest, respChan chan cycleTLSResponse) {
    for res := range reqChan {	
        response := dispatcher(res)
        respChan <- response
    }
}


func readSocket(reqChan chan fullRequest, c *websocket.Conn) {
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Print(err)
			continue
		}
		request := new(cycleTLSRequest)

		err = json.Unmarshal(message, &request)
		if err != nil {
			log.Print(err)
			return
		}

		reply := processRequest(*request)

		reqChan <- reply
	}
}




func writeSocket(respChan chan cycleTLSResponse, c *websocket.Conn) {
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
			
        default:
        }

		// if respChan == nil {
		// 	break
		// }
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
    respChan := make(chan cycleTLSResponse)
    go workerPool(reqChan, respChan)
    
	go readSocket(reqChan, c)
	//run as main thread
	writeSocket(respChan, c)

}
