//go:build integration
// +build integration

package main

import (
	//"fmt"
	"log"
	// "net/http"
    // "net/url"
    // "strings"
	// "io/ioutil"
	// "strconv"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)


func main() {
	client := cycletls.Init()
	formData := cycletls.FormData{
		Fields: []cycletls.FormField{
			{Name: "CustTel", Value: "A"},
			{Name: "CustTel", Value: "1111111111"},
			{Name: "Size", Value: "small"},
			{Name: "Delivery", Value: "11:15"},
			{Name: "Comments", Value: "example test paragraph"},
		},
	}
	resp, err := client.Do("https://httpbin.org/post", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Headers:     map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
		Form:   formData,
	}, "POST")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	
	log.Println(resp)
	 // Create the form data
	// data := url.Values{}
	// data.Add("key2", "value2")
	// for _, field := range formData.Fields {
	// 	data.Set(field.Name, field.Value)
	// }

	//  // Encode the form data
	//  encodedData := data.Encode()
	 

	//  // Create the HTTP request
	//  req, err := http.NewRequest("POST", "https://httpbin.org/post", strings.NewReader(encodedData))
	//  if err != nil {
	// 	 // handle error
	//  }
	//  req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	//  req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
	//  // Send the request
	//  client := &http.Client{}
	//  resp, err := client.Do(req)
	//  if err != nil {
	// 	 // handle error
	// }

	// defer resp.Body.Close()

	// log.Println(resp)
	// body, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	panic(err)
	// }

	// log.Println(string(body))

}
