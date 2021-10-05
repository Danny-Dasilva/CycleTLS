// +build integration

package cycletls_test

import (
	//"fmt"
	// "encoding/json"
	"log"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

type CycleTLSOptions struct {
	Ja3Hash      string `json:"ja3_hash"`
	Ja3          string `json:"ja3"`
	UserAgent    string `json:"User-Agent"`
	HTTPResponse int
}

type Ja3erResp struct {
	Ja3Hash   string `json:"ja3_hash"`
	Ja3       string `json:"ja3"`
	UserAgent string `json:"User-Agent"`
}

var CycleTLSResults = []CycleTLSOptions{
	{"bc6c386f480ee97b9d9e52d472b772d8", // HelloChrome_58
		"769,49200-49196-49192-49188-49172-49162-165-163-161-159-107-106-105-104-57-56-55-54-136-135-134-133-49202-49198-49194-49190-49167-49157-157-61-53-132-49199-49195-49191-49187-49171-49161-164-162-160-158-103-64-63-62-51-50-49-48-154-153-152-151-69-68-67-66-49201-49197-49193-49189-49166-49156-156-60-47-150-65-7-49169-49159-49164-49154-5-4-49170-49160-22-19-16-13-49165-49155-10-255,0-11-10-35-13-15,23-25-28-27-24-26-22-14-13-11-12-9-10,0-1-2",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:88.0) Gecko/20100101 Firefox/88.0 (count: 6603, last seen: 2021-10-05 10:18:25)",
		200},
	{"bc6c386f480ee97b9d9e52d472b772d8", // HelloChrome_62
		"769,52244-52243-52245-49195-49199-158-49162-49172-57-49161-49171-51-49159-49169-156-53-47-5-4-10-255,0-23-35-13-5-13172-18-16-30032-11-10,23-24-25,0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/62.0.3165.0 Safari/537.36",
		200},
	{"b4918ee98d0f0deb4e48563ca749ef10", // HelloChrome_70
		"771,49200-49196-49192-49188-49172-49162-165-163-161-159-107-106-105-104-57-56-55-54-136-135-134-133-49202-49198-49194-49190-49167-49157-157-61-53-132-49199-49195-49191-49187-49171-49161-164-162-160-158-103-64-63-62-51-50-49-48-154-153-152-151-69-68-67-66-49201-49197-49193-49189-49166-49156-156-60-47-150-65-7-49169-49159-49164-49154-5-4-49170-49160-22-19-16-13-49165-49155-10-255,0-11-10-35-13-15,23-25-28-27-24-26-22-14-13-11-12-9-10,0-1-2",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/70.0.3538.102 Safari/537.36",
		200},
	{"66918128f1b9b03303d77c6f2eefd128", // HelloChrome_72
		"771,49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53-10,18-16-30032-11-10-65281-0-23-35-13-5,29-23-24,0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/72.0.3626.96 Safari/537.36",
		200},
	{"b32309a26951912be7dba376398abc3b", // HelloChrome_83
		"771,49200-49196-49202-49198-49199-49195-49201-49197-163-159-162-158-49192-49188-49172-49162-49194-49190-49167-49157-107-106-57-56-49191-49187-49171-49161-49193-49189-49166-49156-103-64-51-50-49170-49160-49165-49155-136-135-69-68-22-19-157-156-61-53-60-47-132-65-10-49169-49159-49164-49154-5-255,0-11-10-35-13-15,25-24-23,0-1-2",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
		200},
	{"0ffee3ba8e615ad22535e7f771690a28", // HelloFirefox_55
		"771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21-41,29-23-24,0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:55.0) Gecko/20100101 Firefox/55.0",
		200},
	{"0ffee3ba8e615ad22535e7f771690a28", // HelloFirefox_56
		"771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-41,29-23-24,0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:56.0) Gecko/20100101 Firefox/56.0",
		200},
	{"b20b44b18b853ef29ab773e921b03422", // HelloFirefox_63
		"771,52244-52243-52245-49195-49199-158-49162-49172-57-49161-49171-51-49159-49169-156-53-47-5-4-10-255,0-23-35-13-5-13172-18-16-30032-11-10,23-24,0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:63.0) Gecko/20100101 Firefox/63.0",
		200},
	{"b20b44b18b853ef29ab773e921b03422", // HelloFirefox_65
		"771,52244-52243-52245-49195-49199-158-49162-49172-57-49161-49171-51-49159-49169-156-53-47-5-4-10-255,0-23-35-13-5-13172-18-16-30032-11-10,23-24,0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:65.0) Gecko/20100101 Firefox/65.0",
		200},
	{"a69708a64f853c3bcc214c2c5faf84f3", // HelloIOS_11_1
		"771,52393-52392-49195-49199-49196-49200-49171-49172-156-157-47-53-10,65281-0-23-35-13-5-18-16-30032-11-10-27,29-23-24,0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A356 Safari/604.1",
		200},
	{"5c118da645babe52f060d0754256a73c", // HelloIOS_12_1
		"771,52244-52243-49195-49199-158-49162-49172-57-49161-49171-51-156-53-47-10,65281-0-23-35-13-5-13172-18-16-30032-11-10,23-24,0",
		"MozaaaaaaaaaaaaaaaaaaaabKit/602.1.50 (KHTML, like Gecko) Version/12.0 Mobile/14A5335d Safari/602.1.50",
		200},
	{"5c118da645babe52f060d0754256a73c", // HelloIOS_12_1
		"771,52244-52243-52245-49172-49162-57-56-53-49170-49160-22-19-10-49199-49195-49171-49161-162-158-51-50-156-47-49169-5-4-255,0-35-13172,25,0",
		"Mozbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.0 Mobile/14A5335d Safari/602.1.50",
		200},
	{"5c118da645babe52f060d0754256a73c", // HelloIOS_12_1
		"771,49200-49196-49202-49198-49199-49195-49201-49197-163-159-162-158-49192-49188-49172-49162-49194-49190-49167-49157-107-106-57-56-49191-49187-49171-49161-49193-49189-49166-49156-103-64-51-50-49170-49160-49165-49155-136-135-69-68-22-19-157-156-61-53-60-47-132-65-10-49169-49159-49164-49154-5-255,0-11-10-35-13-15-21,14-13-25-11-12-24-9-10-22-23-8-6-7-20-21-4-5-18-19-1-2-3-15-16-17,0-1-2",
		"Mozilla/5cccccccccccccccccccccccc0 (KHTML, like Gecko) Version/12.0 Mobile/14A5335d Safari/602.1.50",
		200},
	{"5c118da645babe52f060d0754256a73c", // HelloIOS_12_1
		"771,52244-52243-52245-49195-49199-158-49162-49172-57-49161-49171-51-156-53-47-10-255,0-23-35-13-5-13172-18-16-30032-11-10,23-24,0",
		"asd1.50 cccccccccdddddddddddddddddddddddddd12.0 Mobile/14A5335d Safari/602.1.50",
		200},
}

// {"ja3_hash":"aa7744226c695c0b2e440419848cf700", "ja3": "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0", "User-Agent": "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0"}
func TestExtensions(t *testing.T) {
	client := cycletls.Init()
	for _, options := range CycleTLSResults {

		response, err := client.Do("https://ja3er.com/json", cycletls.Options{
			Ja3:       options.Ja3,
			UserAgent: options.UserAgent,
		}, "GET")
		if err != nil {
			t.Fatal("Unmarshal Error")
		}

		// if response.Response.Status != options.HTTPResponse {
		// 	t.Fatal("Expected Result Not given")
		// } else {
		// 	log.Println("ja3er: ", response.Response.Status)
		// }
		// ja3resp := new(Ja3erResp)
		log.Println(response.Response.Body)
		// err = json.Unmarshal([]byte(response.Response.Body), &ja3resp)
		// if err != nil {
		// 	t.Fatal("Unmarshal Error")
		// }

		// if ja3resp.Ja3Hash != options.Ja3Hash {
		// 	t.Fatal("Expected {} Got {} for Ja3Hash", options.Ja3Hash, ja3resp.Ja3Hash)
		// }
		// if ja3resp.Ja3 != options.Ja3 {
		// 	t.Fatal("Expected {} Got {} for Ja3", options.Ja3, ja3resp.Ja3)
		// }
		// if ja3resp.UserAgent != options.UserAgent {
		// 	t.Fatal("Expected {} Got {} for UserAgent", options.UserAgent, ja3resp.UserAgent)
		// }

	}
}
