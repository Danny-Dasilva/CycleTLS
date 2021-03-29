package main

import (
	"encoding/json"
	"flag"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"strings"
	"log"
	"net/http"
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
		UserAgent     string            `json:"userAgent"`
		Proxy   string            `json:"proxy"`
	} `json:"options"`
}


// ChromeAuto mocks Chrome 78
var ChromeAuto = Browser{
	JA3:       "769,47–53–5–10–49161–49162–49171–49172–50–56–19–4,0–10–11,23–24–25,0",
	UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36",
}

// SafariAuto mocks Safari 604.1
var SafariAuto = Browser{
	JA3:       "771,4865-4866-4867-49196-49195-49188-49187-49162-49161-52393-49200-49199-49192-49191-49172-49171-52392-157-156-61-60-53-47-49160-49170-10,65281-0-23-13-5-18-16-11-51-45-43-10-21,29-23-24-25,0",
	UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 13_1_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.1 Mobile/15E148 Safari/604.1",
}

var FirefoxAuto = Browser{
	JA3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
	UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:86.0) Gecko/20100101 Firefox/86.0",
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

func main() {
	flag.Parse()
	log.SetFlags(0)

	websocketAddress := getWebsocketAddr()

	c, _, err := websocket.DefaultDialer.Dial(websocketAddress, nil)
	if err != nil {
		log.Print(err)
		return
	}

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
		fmt.Println(Default)



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

		resp, err := client.Do(req)
		if err != nil {
			log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
			continue
		}

		defer resp.Body.Close()
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
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

		Response := response{resp.StatusCode, string(bodyBytes), headers}

		reply := myTLSResponse{mytlsrequest.RequestID, Response}

		data, err := json.Marshal(reply)
		if err != nil {
			log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
			continue
		}

		err = c.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
			continue
		}


		// resp, err := client.Get(mytlsrequest.Options.URL)
		// // resp, err := client.Get("https://api.ipify.org/?format=json")
		// if err != nil {
		// 	fmt.Println(err)
		// 	log.Fatal(err)
		// } else {
		// 	fmt.Println(resp.StatusCode)
		// 	// body, _ := ioutil.ReadAll(resp.Body)
		// 	// fmt.Println(string(body))
		// }

	

	

		// defer resp.Body.Close()
		// bodyBytes, err := ioutil.ReadAll(resp.Body)
		// if err != nil {
		// 	log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
		// 	continue
		// }

		// headers := make(map[string]string)

		// for name, values := range resp.Header {
		// 	if name == "Set-Cookie" {
		// 		headers[name] = strings.Join(values, "/,/")
		// 	} else {
		// 		for _, value := range values {
		// 			headers[name] = value
		// 		}
		// 	}
		// }
		// fmt.Println(bodyBytes)
		// Response := response{resp.StatusCode, string(bodyBytes), headers}

		// reply := myTLSResponse{mytlsrequest.RequestID, Response}

		// data, err := json.Marshal(reply)
		// if err != nil {
		// 	log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
		// 	continue
		// }
		
		// err = c.WriteMessage(websocket.TextMessage, data)
		// if err != nil {
		// 	log.Print(mytlsrequest.RequestID + "Request_Id_On_The_Left" + err.Error())
		// 	continue
		// }
	}
}

