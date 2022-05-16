package main

import (
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
    "io/ioutil"
	"path"
	"path/filepath"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
    "strings"
	

	"github.com/andybalholm/brotli"
)

func gUnzipData(data []byte) (resData []byte, err error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return []byte{}, err
	}
	defer gz.Close()
	respBody, err := ioutil.ReadAll(gz)
	return respBody, err
}
func enflateData(data []byte) (resData []byte, err error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return []byte{}, err
	}
	defer zr.Close()
	enflated, err := ioutil.ReadAll(zr)
	return enflated, err
}
func unBrotliData(data []byte) (resData []byte, err error) {
	br := brotli.NewReader(bytes.NewReader(data))
	respBody, err := ioutil.ReadAll(br)
	return respBody, err
}
func DecompressBody(Body []byte, encoding []string, content []string) (parsedBody string) {
	if len(encoding) > 0 {
		if encoding[0] == "gzip" {
			unz, err := gUnzipData(Body)
			if err != nil {
				return string(Body)
			}
			return string(unz)
		} else if encoding[0] == "deflate" {
			unz, err := enflateData(Body)
			if err != nil {
				return string(Body)
			}
			return string(unz)
		} else if encoding[0] == "br" {
			unz, err := unBrotliData(Body)
			if err != nil {
				return string(Body)
			}
			return string(unz)
		}
	} else if len(content) > 0 {
		decodingTypes := map[string]bool{
			"image/svg+xml": true,
			"image/webp":    true,
			"image/jpeg":    true,
			"image/png":     true,
		}
		if decodingTypes[content[0]] {
			return base64.StdEncoding.EncodeToString(Body)
		}
	}
	parsedBody = string(Body)
	return parsedBody

}
// func main() {
//   fileDir, _ := os.Getwd()
//   fileName := "README.md"
//   filePath := path.Join(fileDir, fileName)

//   file, _ := os.Open(filePath)
//   defer file.Close()

//   body := &bytes.Buffer{}
//   writer := multipart.NewWriter(body)
//   part, _ := writer.CreateFormFile("file", filepath.Base(file.Name()))
//   io.Copy(part, file)
//   writer.Close()

//   r, _ := http.NewRequest("POST", "http://httpbin.org/post", body)
//   r.Header.Add("Content-Type", writer.FormDataContentType())
//   client := &http.Client{}
//   resp, err := client.Do(r)
//   if err != nil {
//     log.Println("err")
//     }
//   bodyBytes, err := ioutil.ReadAll(resp.Body)
//   encoding := resp.Header["Content-Encoding"]
//   content := resp.Header["Content-Type"]

//   Body := DecompressBody(bodyBytes, encoding, content)
//   log.Println(Body)

// }

func main() {
    fileDir, _ := os.Getwd()
    fileName := "README.md"
    filePath := path.Join(fileDir, fileName)
    file, _ := os.Open(filePath)
    _=filepath.Base(file.Name())

    defer file.Close()
  
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    fw, err := writer.CreateFormField("name")
    if err != nil {
    }
    _, err = io.Copy(fw, strings.NewReader("John"))
    if err != nil {
        log.Println("err")
    }
    ///////////////
    fw, err = writer.CreateFormField("age")
    if err != nil {
    }
    _, err = io.Copy(fw, strings.NewReader("23"))
    if err != nil {
        log.Println("err")
    }
    //////////
    writer.Close()
  
    r, _ := http.NewRequest("POST", "http://httpbin.org/post", body)
    r.Header.Add("Content-Type", writer.FormDataContentType())
    client := &http.Client{}
    resp, err := client.Do(r)
    if err != nil {
      log.Println("err")
      }
    bodyBytes, err := ioutil.ReadAll(resp.Body)
    encoding := resp.Header["Content-Encoding"]
    content := resp.Header["Content-Type"]
  
    Body := DecompressBody(bodyBytes, encoding, content)
    log.Println(Body)
  
  }