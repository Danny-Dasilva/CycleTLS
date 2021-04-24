package main

import (
	"log"
	"time"
	"./cycletls"
	"runtime"
	"encoding/json"
	"net/http"
	"github.com/simplereach/timeutils"
)

// type time struct {
// 	Time timeutils.Time `json:"time"`
// }

func time(input string) {
	const layout = "Mon, 02-Jan-2006 15:04:05 MST"
  
	k, err := time.Parse(layout, "Wed, 11-Feb-2015 11:06:39 MST")
	return k

}
func main() {
    runtime.GOMAXPROCS(runtime.NumCPU())

	start := time.Now()
    defer func() {
        log.Println("Execution Time: ", time.Since(start))
    }()
	client := cycletls.Init()

	var d data
	jStr := `{"time":"Wed, 11-Feb-2015 11:06:39 GMT"}`
	_ = json.Unmarshal([]byte(jStr), &d)
	// log.Println(d.Time, "aaaaa")
	const layout = "Mon, 02-Jan-2006 15:04:05 MST"
  
	k, err := time.Parse(layout, "Wed, 11-Feb-2015 11:06:39 MST")
	
	log.Println(k, err, "ahasldkhlkfjhas")
	

	response := client.Do("https://httpbin.org/headers", cycletls.Options{
		Body : "",
		Ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		// Headers: map[string]string{"Cookie": "valu=5"},
		
		Cookies: []http.Cookie{ 
			http.Cookie {
				Name: "a", 
				Value: "multiple", 
				Path: "/",
				Domain: ".google.com",
				// Expires: "Sun, 24-Apr-2022 01:12:52 GMT",
				MaxAge: 1,
				HttpOnly: true,
				Secure: true,
				// SameSite: true,
				Raw: "the n word",
				Unparsed: []string{"SIDCC=AJi4QfFj1sVJKiKsz1vn2htKtZ-wb8YQLcYVAZuFxL1qYRQuoMBjWD1Hpp0sciheirRElEX3Ow; expires=Sun, 24-Apr-2022 01:12:52 GMT; path=/; domain=.google.com; priority=high"},
			},
			http.Cookie {
				Name: "yaaah", 
				Value: "b-option", 
			},
		},
		
	  }, "GET");
	
	log.Println(response)
}
