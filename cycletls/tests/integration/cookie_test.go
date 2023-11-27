package cycletls_test

import (
	//"fmt"
	"encoding/json"
	"log"
	"reflect"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func TestCookies(t *testing.T) {
	client := cycletls.Init()
	resp, err := client.Do("https://httpbin.org/cookies", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Cookies: []cycletls.Cookie{{Name: "cookie1", Value: "value1"},
			{Name: "cookie2", Value: "value2"}},
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}

	expected := `{
		"cookies": {
		  "cookie1": "value1", 
		  "cookie2": "value2"
		}
	  }`
	var data map[string]interface{}
	err = json.Unmarshal([]byte(expected), &data)
	if err != nil {
		log.Print("Json Conversion failed " + err.Error())
	}

	eq := reflect.DeepEqual(resp.JSONBody(), data)
	if eq {
		log.Println("They're equal.")
	} else {
		t.Fatalf("Expected %s Got %s, expected cookies not found", data, resp.JSONBody())
	}
}
