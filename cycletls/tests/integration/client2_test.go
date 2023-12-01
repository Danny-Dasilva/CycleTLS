package cycletls

import (
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	http "github.com/Danny-Dasilva/fhttp"
	utls "github.com/refraction-networking/utls"
	"io"
	"sync"
	"testing"
	"time"
)

const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36 OPR/48.0.2685.52"

func TestHttpClientBuilderWithProxyVisitingGoogle(t *testing.T) {

	browser := &cycletls.Browser{
		JA3:                "",
		UserAgent:          UserAgent,
		InsecureSkipVerify: true,
	}

	builder := cycletls.HttpClientBuilder{
		Browser:              browser,
		ClientHelloId:        &utls.HelloRandomized,
		ProxyUrl:             "http://localhost:8888", // fiddler, watch if max tunnels correct
		MaxConnectionPerHost: 4,
		MaxIdleConnections:   10,
		Timeout:              time.Second * 5,
	}

	client, err := builder.Build()
	if err != nil {
		t.Fatalf(err.Error())
	}

	wg := &sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func() {

			for j := 0; j < 10; j++ {
				req, _ := http.NewRequest("GET", "https://www.google.com", nil)
				resp, err := client.Do(req)
				if err != nil {
					panic(err)
				}

				data, err := io.ReadAll(resp.Body)
				if err != nil {
					panic(err)
				}
				_ = resp.Body.Close()

				println(resp.StatusCode)
				println(len(data))
			}

			wg.Done()
		}()
	}

	wg.Wait()

}

func TestJa3(t *testing.T) {

	browser := &cycletls.Browser{
		JA3:                "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,11-65281-35-0-51-18-10-13-23-16-5-43-27-65037-45-17513-21,29-23-24,3",
		UserAgent:          UserAgent,
		InsecureSkipVerify: true,
	}

	builder := cycletls.HttpClientBuilder{
		Browser:              browser,
		MaxConnectionPerHost: 4,
		MaxIdleConnections:   10,
	}

	client, err := builder.Build()
	if err != nil {
		panic(err)
	}

	// The second call would report unexpected EOF error, I think the remote server closed the connection
	// and I don't know how to get noticed of the server closing connection, and reopen a new connection.

	req, _ := http.NewRequest("GET", "https://tls.browserleaks.com/json", nil)
	req.Header = map[string][]string{"Connection": {"keep-alive"}}
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf(err.Error())
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(err.Error())
	}
	_ = resp.Body.Close()

	println(resp.StatusCode)
	println(string(data))

}

func TestHttpClientBuilderWithProxyVisitingTlsLeaks(t *testing.T) {

	browser := &cycletls.Browser{
		JA3:                "",
		UserAgent:          UserAgent,
		InsecureSkipVerify: true,
	}

	builder := cycletls.HttpClientBuilder{
		Browser:              browser,
		ClientHelloId:        &utls.HelloChrome_100,
		MaxConnectionPerHost: 4,
		MaxIdleConnections:   10,
	}

	client, err := builder.Build()
	if err != nil {
		panic(err)
	}

	// The second call would report unexpected EOF error, I think the remote server closed the connection
	// and I don't know how to get noticed of the server closing connection, and reopen a new connection.

	for i := 0; i < 10; i++ {
		req, _ := http.NewRequest("GET", "https://tls.browserleaks.com/json", nil)
		req.Header = map[string][]string{"Connection": {"keep-alive"}}
		resp, err := client.Do(req)

		if err != nil {
			t.Fatalf(err.Error())
		}

		_, err = io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf(err.Error())
		}
		_ = resp.Body.Close()

		println(resp.StatusCode)
	}

}
