package cycletls

import (
	"bytes"
	"encoding/binary"
	nhttp "net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// parsedFrames is what a dispatcher wrote for one request, split by frame type.
type parsedFrames struct {
	status  int
	headers map[string][]string
	body    []byte
	ended   bool
	errors  []string
}

// parseDispatcherFrames reads the frames a dispatcher wrote. Every frame starts
// with the request ID and a method name, each behind a 2-byte length.
func parseDispatcherFrames(t *testing.T, frames [][]byte) parsedFrames {
	t.Helper()
	var out parsedFrames

	for _, frame := range frames {
		r := bytes.NewReader(frame)
		readString := func() string {
			var n uint16
			if err := binary.Read(r, binary.BigEndian, &n); err != nil {
				t.Fatalf("short frame %q: %v", frame, err)
			}
			buf := make([]byte, n)
			if _, err := r.Read(buf); err != nil && n > 0 {
				t.Fatalf("short frame %q: %v", frame, err)
			}
			return string(buf)
		}
		readU16 := func() int {
			var n uint16
			if err := binary.Read(r, binary.BigEndian, &n); err != nil {
				t.Fatalf("short frame %q: %v", frame, err)
			}
			return int(n)
		}

		_ = readString() // request ID
		switch method := readString(); method {
		case "response":
			out.status = readU16()
			_ = readString() // final URL
			out.headers = make(map[string][]string)
			for i, count := 0, readU16(); i < count; i++ {
				name := readString()
				for j, values := 0, readU16(); j < values; j++ {
					out.headers[name] = append(out.headers[name], readString())
				}
			}
		case "data":
			var n uint32
			if err := binary.Read(r, binary.BigEndian, &n); err != nil {
				t.Fatalf("short data frame: %v", err)
			}
			chunk := make([]byte, n)
			if _, err := r.Read(chunk); err != nil && n > 0 {
				t.Fatalf("short data frame: %v", err)
			}
			out.body = append(out.body, chunk...)
		case "end":
			out.ended = true
		case "error":
			_ = readU16() // status code
			out.errors = append(out.errors, readString())
		default:
			t.Fatalf("unexpected %q frame: %q", method, frame)
		}
	}
	return out
}

// headerNamed returns the values of a header whatever its letter case.
func headerNamed(headers map[string][]string, name string) []string {
	var values []string
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			values = append(values, v...)
		}
	}
	return values
}

// Both dispatchers stream the response body to the Node client. A body with a
// Content-Encoding has to arrive decoded, without the two headers that
// described the compressed bytes. Issue #414.
func TestDispatchersDecodeResponseBody(t *testing.T) {
	// Larger than one 8 KB read, so the decoded body spans several data frames.
	payload := []byte(`{"ok":true,"filler":"` + strings.Repeat("cycletls ", 4000) + `"}`)

	type decodeCase struct {
		name     string
		method   string
		encoding string // Content-Encoding header value sent by the server
		body     []byte // bytes on the wire
		// wantBody is what the client must receive.
		wantBody []byte
		// wantEncoding is the Content-Encoding the client must still see.
		wantEncoding []string
		// keepsLength is true when Content-Length must reach the client.
		keepsLength bool
	}

	gzipBody := encodeWith(t, "gzip", payload)
	cases := []decodeCase{
		{name: "gzip", encoding: "gzip", body: gzipBody, wantBody: payload},
		{name: "br", encoding: "br", body: encodeWith(t, "br", payload), wantBody: payload},
		{name: "zstd", encoding: "zstd", body: encodeWith(t, "zstd", payload), wantBody: payload},
		{name: "deflate", encoding: "deflate", body: encodeWith(t, "deflate", payload), wantBody: payload},
		{name: "mixed_case", encoding: "GZip", body: gzipBody, wantBody: payload},
		{
			name:     "gzip_then_br",
			encoding: "gzip, br",
			body:     encodeWith(t, "br", gzipBody),
			wantBody: payload,
		},
		{name: "identity", body: payload, wantBody: payload, keepsLength: true},
		{
			// A coding we can't decode passes through with its headers.
			name:         "unknown_coding",
			encoding:     "compress",
			body:         gzipBody,
			wantBody:     gzipBody,
			wantEncoding: []string{"compress"},
			keepsLength:  true,
		},
		{
			// HEAD has no body: the headers must still describe the resource.
			name:         "head",
			method:       "HEAD",
			encoding:     "gzip",
			body:         gzipBody,
			wantBody:     nil,
			wantEncoding: []string{"gzip"},
			keepsLength:  true,
		},
	}

	byPath := make(map[string]decodeCase, len(cases))
	for _, tc := range cases {
		byPath["/"+tc.name] = tc
	}

	srv := httptest.NewUnstartedServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		tc, ok := byPath[r.URL.Path]
		if !ok {
			w.WriteHeader(nhttp.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if tc.encoding != "" {
			w.Header().Set("Content-Encoding", tc.encoding)
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(tc.body)))
		if r.Method != nhttp.MethodHead {
			_, _ = w.Write(tc.body)
		}
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	t.Cleanup(clearAllConnections)

	for _, path := range dispatchPaths {
		for _, tc := range cases {
			t.Run(path.name+"/"+tc.name, func(t *testing.T) {
				method := tc.method
				if method == "" {
					method = "GET"
				}

				chanWrite := make(chan []byte, 64)
				path.start(cycleTLSRequest{
					RequestID: "decode-test",
					Options: Options{
						URL:                   srv.URL + "/" + tc.name,
						Method:                method,
						Headers:               map[string]string{"Accept-Encoding": "gzip, deflate, br, zstd"},
						InsecureSkipVerify:    true,
						EnableConnectionReuse: true,
					},
				}, chanWrite)

				var frames [][]byte
				timeout := time.After(10 * time.Second)
			collect:
				for {
					select {
					case f, ok := <-chanWrite:
						if !ok {
							break collect
						}
						frames = append(frames, f)
					case <-timeout:
						t.Fatalf("%s dispatcher did not return", path.name)
					}
				}

				got := parseDispatcherFrames(t, frames)
				if len(got.errors) > 0 {
					t.Fatalf("error frames: %q", got.errors)
				}
				if got.status != 200 {
					t.Fatalf("status = %d, want 200", got.status)
				}
				if !got.ended {
					t.Error("no end frame")
				}
				if !bytes.Equal(got.body, tc.wantBody) {
					t.Errorf("body is %d bytes, want %d decoded bytes (starts %q)",
						len(got.body), len(tc.wantBody), firstBytes(got.body, 16))
				}

				gotEncoding := headerNamed(got.headers, "Content-Encoding")
				if len(gotEncoding) != len(tc.wantEncoding) || (len(gotEncoding) > 0 && gotEncoding[0] != tc.wantEncoding[0]) {
					t.Errorf("Content-Encoding = %q, want %q", gotEncoding, tc.wantEncoding)
				}
				gotLength := headerNamed(got.headers, "Content-Length")
				if tc.keepsLength && len(gotLength) == 0 {
					t.Error("Content-Length was removed from a body that was not decoded")
				}
				if !tc.keepsLength && len(gotLength) > 0 {
					t.Errorf("Content-Length = %q on a decoded body, want it removed", gotLength)
				}
			})
		}
	}
}

func firstBytes(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}
