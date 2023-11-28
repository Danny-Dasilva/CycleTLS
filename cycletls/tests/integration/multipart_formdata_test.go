//go:build integration
// +build integration

package cycletls_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"os"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

type HttpBinResponse struct {
	Args    map[string]string `json:"args"`
	Data    string            `json:"data"`
	Files   map[string]string `json:"files"`
	Form    map[string]string `json:"form"`
	Headers map[string]string `json:"headers"`
	Json    interface{}       `json:"json"`
	Origin  string            `json:"origin"`
	URL     string            `json:"url"`
}

func TestMultipartFormDataMixed(t *testing.T) {
	client := cycletls.Init()

	// Prepare a buffer to write our multipart form
	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)

	// Add form fields
	err := multipartWriter.WriteField("key1", "value1")
	if err != nil {
		t.Fatal("Error adding form field: ", err)
	}
	err = multipartWriter.WriteField("key2", "value2")
	if err != nil {
		t.Fatal("Error adding form field: ", err)
	}

	// Add a file
	fileWriter, err := multipartWriter.CreateFormFile("test_file", "../../go.mod")
	if err != nil {
		t.Fatal("CreateFormFile Error: ", err)
	}

	// Open the file that you want to upload
	file, err := os.Open("../../go.mod")
	if err != nil {
		t.Fatal("File Open Error: ", err)
	}
	defer file.Close()

	// Copy the file to the multipart writer
	_, err = io.Copy(fileWriter, file)
	if err != nil {
		t.Fatal("File Copy Error: ", err)
	}

	// Close the writer before making the request
	contentType := multipartWriter.FormDataContentType()
	multipartWriter.Close()

	response, err := client.Do("http://httpbin.org/post", cycletls.Options{
		Body: requestBody.String(),
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		InsecureSkipVerify: true,
	}, "POST")
	if err != nil {
		t.Fatal("Request Failed: ", err)
	}

	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}
	var respData HttpBinResponse
	err = json.Unmarshal([]byte(response.Body), &respData)
	if err != nil {
		t.Fatal("Unmarshal Error: ", err)
	}
	if _, ok := respData.Files["test_file"]; !ok {
		t.Fatal("Expected file 'filetype.csv' in response, but it was not found")
	}

	expectedForm := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	for key, expectedValue := range expectedForm {
		if value, ok := respData.Form[key]; !ok || value != expectedValue {
			t.Fatalf("Expected form field %s to be %s, got %s", key, expectedValue, value)
		}
	}
}

func TestMultipartFormDataUpload(t *testing.T) {
	client := cycletls.Init()

	// Prepare a buffer to write our multipart form
	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)

	// Add a file
	fileWriter, err := multipartWriter.CreateFormFile("test_file", "../../go.mod")
	if err != nil {
		t.Fatal("CreateFormFile Error: ", err)
	}

	// Open the file that you want to upload
	file, err := os.Open("../../go.mod")
	if err != nil {
		t.Fatal("File Open Error: ", err)
	}
	defer file.Close()

	// Copy the file to the multipart writer
	_, err = io.Copy(fileWriter, file)
	if err != nil {
		t.Fatal("File Copy Error: ", err)
	}

	// Close the writer before making the request
	contentType := multipartWriter.FormDataContentType()
	multipartWriter.Close()

	response, err := client.Do("http://httpbin.org/post", cycletls.Options{
		Body: requestBody.String(),
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		InsecureSkipVerify: true,
	}, "POST")
	if err != nil {
		t.Fatal("Request Failed: ", err)
	}

	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}
	var respData HttpBinResponse
	err = json.Unmarshal([]byte(response.Body), &respData)
	if err != nil {
		t.Fatal("Unmarshal Error: ", err)
	}
	if _, ok := respData.Files["test_file"]; !ok {
		t.Fatal("Expected file 'filetype.csv' in response, but it was not found")
	}
}

func TestMultipartFormDataText(t *testing.T) {
	client := cycletls.Init()

	// Prepare a buffer to write our multipart form
	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)

	// Add form fields
	err := multipartWriter.WriteField("key1", "value1")
	if err != nil {
		t.Fatal("Error adding form field: ", err)
	}
	err = multipartWriter.WriteField("key2", "value2")
	if err != nil {
		t.Fatal("Error adding form field: ", err)
	}
	// Close the writer before making the request
	contentType := multipartWriter.FormDataContentType()
	multipartWriter.Close()

	response, err := client.Do("http://httpbin.org/post", cycletls.Options{
		Body: requestBody.String(),
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		InsecureSkipVerify: true,
	}, "POST")
	if err != nil {
		t.Fatal("Request Failed: ", err)
	}

	if response.Status != 200 {
		t.Fatalf("Expected status code %d, got %d", 200, response.Status)
	}
	var respData HttpBinResponse
	err = json.Unmarshal([]byte(response.Body), &respData)
	if err != nil {
		t.Fatal("Unmarshal Error: ", err)
	}

	expectedForm := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	for key, expectedValue := range expectedForm {
		if value, ok := respData.Form[key]; !ok || value != expectedValue {
			t.Fatalf("Expected form field %s to be %s, got %s", key, expectedValue, value)
		}
	}
}
