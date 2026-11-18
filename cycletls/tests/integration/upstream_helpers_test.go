package cycletls_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// Helpers shared by the integration tests. This file has no build tag because
// some of the tests that use it (binary_data, cookie, cookiejar, images) have
// none either and run under a plain `go test ./...`.

// upstreamBackoffs are the waits between attempts in doUpstream.
var upstreamBackoffs = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

// isUpstreamFlake reports whether a status means "the public host or the route
// to it is unavailable" rather than "cycletls returned a wrong answer".
//
//	408      request timed out
//	421      DNS or connection failure (cycletls error mapping)
//	429      rate limited; httpbin.org does this when CI runs share an address
//	495      connection reset or TLS handshake failure (cycletls error mapping)
//	5xx      upstream fault
func isUpstreamFlake(status int) bool {
	return status == 408 || status == 421 || status == 429 || status == 495 ||
		(status >= 500 && status < 600)
}

// doUpstream is client.Do for requests to shared public hosts such as
// httpbin.org. It retries with backoff while the host answers with a flake
// status and, if every attempt flakes, skips the test with the reason. Any
// other outcome is returned to the caller unchanged, so the test's own
// assertions still decide pass or fail.
//
// Statuses listed in expected are returned at once even if they are flake
// statuses; a timeout test, for example, passes 408.
//
// It may call t.Skipf, so call it only from the test goroutine.
func doUpstream(t *testing.T, client cycletls.CycleTLS, url string, opts cycletls.Options, method string, expected ...int) (cycletls.Response, error) {
	t.Helper()
	attempts := len(upstreamBackoffs) + 1
	var resp cycletls.Response
	var err error
	for i := 0; i < attempts; i++ {
		resp, err = client.Do(url, opts, method)
		if err != nil {
			return resp, err
		}
		for _, status := range expected {
			if resp.Status == status {
				return resp, nil
			}
		}
		if !isUpstreamFlake(resp.Status) {
			return resp, nil
		}
		t.Logf("attempt %d/%d: %s %s returned %d", i+1, attempts, method, url, resp.Status)
		if i < len(upstreamBackoffs) {
			time.Sleep(upstreamBackoffs[i])
		}
	}
	t.Skipf("upstream unavailable: %s %s returned %d on %d attempts; skipping", method, url, resp.Status, attempts)
	return resp, err
}

// newEchoServer starts a local server for tests that assert byte-exact
// behaviour. It answers a small part of the httpbin.org API:
//
//	POST /post        JSON with the request body in "data" as a base64 data
//	                  URL, plus "form" and "files" for multipart bodies
//	GET  /image/jpeg  tests/images/test.jpeg
//	GET  /image/png   tests/images/test.png
//
// The server is closed when the test ends.
func newEchoServer(t *testing.T) *httptest.Server {
	t.Helper()

	serveFile := func(w http.ResponseWriter, path, contentType string) {
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Write(data)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/image/jpeg", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, "../../../tests/images/test.jpeg", "image/jpeg")
	})
	mux.HandleFunc("/image/png", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, "../../../tests/images/test.png", "image/png")
	})
	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		form := map[string]string{}
		files := map[string]string{}
		data := ""
		contentType := r.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "multipart/form-data") {
			// Parse a copy so the raw body stays available above.
			parsed := r.Clone(r.Context())
			parsed.Body = io.NopCloser(bytes.NewReader(body))
			if err := parsed.ParseMultipartForm(32 << 20); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			for name, values := range parsed.MultipartForm.Value {
				if len(values) > 0 {
					form[name] = values[0]
				}
			}
			for name, headers := range parsed.MultipartForm.File {
				if len(headers) == 0 {
					continue
				}
				f, err := headers[0].Open()
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				content, _ := io.ReadAll(f)
				f.Close()
				files[name] = "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(content)
			}
		} else if len(body) > 0 {
			data = "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(body)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data":  data,
			"form":  form,
			"files": files,
		})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}
