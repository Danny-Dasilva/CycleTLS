package cclient

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"testing"

	tls "github.com/refraction-networking/utls"
)

type JA3Response struct {
	JA3Hash   string `json:"ja3_hash"`
	JA3       string `json:"ja3"`
	UserAgent string `json:"User-Agent"`
}

func readAndClose(r io.ReadCloser) ([]byte, error) {
	readBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return readBytes, r.Close()
}

const Chrome83Hash = "b32309a26951912be7dba376398abc3b"

var client, _ = NewClient(tls.HelloChrome_83) // cannot throw an error because there is no proxy

func TestCClient_JA3(t *testing.T) {
	resp, err := client.Get("https://ja3er.com/json")
	if err != nil {
		t.Fatal(err)
	}

	respBody, err := readAndClose(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var ja3Response JA3Response
	if err := json.Unmarshal(respBody, &ja3Response); err != nil {
		t.Fatal(err)
	}

	if ja3Response.JA3Hash != Chrome83Hash {
		t.Error("unexpected JA3 hash; expected:", Chrome83Hash, "| got:", ja3Response.JA3Hash)
	}
}

func TestCClient_HTTP2(t *testing.T) {
	resp, err := client.Get("https://www.google.com")
	if err != nil {
		t.Fatal(err)
	}

	_, err = readAndClose(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.ProtoMajor != 2 || resp.ProtoMinor != 0 {
		t.Error("unexpected response proto; expected: HTTP/2.0 | got: ", resp.Proto)
	}
}