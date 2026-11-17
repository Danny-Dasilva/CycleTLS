package cycletls

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func encodeWith(t *testing.T, coding string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	var w io.WriteCloser
	var err error
	switch coding {
	case "gzip":
		w = gzip.NewWriter(&buf)
	case "deflate":
		w = zlib.NewWriter(&buf)
	case "deflate-raw":
		w, err = flate.NewWriter(&buf, flate.DefaultCompression)
	case "br":
		w = brotli.NewWriter(&buf)
	case "zstd":
		w, err = zstd.NewWriter(&buf)
	default:
		t.Fatalf("unknown coding %q", coding)
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestNewDecodedBody(t *testing.T) {
	want := []byte(`{"ok":true,"filler":"` + strings.Repeat("cycletls ", 4000) + `"}`)

	tests := []struct {
		name   string
		header []string
		wire   []string // codings applied to the payload, in order
	}{
		{"gzip", []string{"gzip"}, []string{"gzip"}},
		{"x-gzip", []string{"x-gzip"}, []string{"gzip"}},
		{"deflate as zlib", []string{"deflate"}, []string{"deflate"}},
		{"deflate as raw stream", []string{"deflate"}, []string{"deflate-raw"}},
		{"br", []string{"br"}, []string{"br"}},
		{"zstd", []string{"zstd"}, []string{"zstd"}},
		{"mixed-case value", []string{"GZip"}, []string{"gzip"}},
		{"padded value", []string{"  br "}, []string{"br"}},
		{"identity then gzip", []string{"identity, gzip"}, []string{"gzip"}},
		{"two codings in one value", []string{"gzip, br"}, []string{"gzip", "br"}},
		{"two codings in two values", []string{"deflate", "zstd"}, []string{"deflate", "zstd"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wire := want
			for _, coding := range tc.wire {
				wire = encodeWith(t, coding, wire)
			}

			readers := map[string]io.Reader{
				"whole body":         bytes.NewReader(wire),
				"one byte at a time": iotest.OneByteReader(bytes.NewReader(wire)),
			}
			for name, r := range readers {
				body, ok := newDecodedBody(r, tc.header)
				if !ok {
					t.Fatalf("%s: newDecodedBody did not decode %v", name, tc.header)
				}
				got, err := io.ReadAll(body)
				if err != nil {
					t.Fatalf("%s: read: %v", name, err)
				}
				body.Close()
				if !bytes.Equal(got, want) {
					t.Fatalf("%s: got %d bytes starting %q, want %d bytes", name, len(got), got[:min(len(got), 16)], len(want))
				}
			}
		})
	}
}

func TestNewDecodedBodyPassThrough(t *testing.T) {
	plain := []byte(`{"ok":true}`)

	t.Run("no Content-Encoding", func(t *testing.T) {
		for _, header := range [][]string{nil, {}, {""}, {"identity"}} {
			body, ok := newDecodedBody(bytes.NewReader(plain), header)
			if ok {
				t.Fatalf("header %v: want ok=false", header)
			}
			if got, _ := io.ReadAll(body); !bytes.Equal(got, plain) {
				t.Fatalf("header %v: body changed to %q", header, got)
			}
		}
	})

	t.Run("unknown coding leaves the body alone", func(t *testing.T) {
		gz := encodeWith(t, "gzip", plain)
		body, ok := newDecodedBody(bytes.NewReader(gz), []string{"gzip, compress"})
		if ok {
			t.Fatal("want ok=false for a coding we cannot decode")
		}
		if got, _ := io.ReadAll(body); !bytes.Equal(got, gz) {
			t.Fatal("body should be untouched")
		}
	})

	t.Run("body labelled gzip that is not gzip", func(t *testing.T) {
		body, _ := newDecodedBody(bytes.NewReader(plain), []string{"gzip"})
		got, err := io.ReadAll(body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("got %q, want the original body", got)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		for _, coding := range []string{"gzip", "deflate", "br", "zstd"} {
			body, _ := newDecodedBody(bytes.NewReader(nil), []string{coding})
			got, err := io.ReadAll(body)
			if err != nil || len(got) != 0 {
				t.Fatalf("%s: got %q, err %v", coding, got, err)
			}
		}
	})
}

func TestNewDecodedBodyReportsCorruptStream(t *testing.T) {
	gz := encodeWith(t, "gzip", []byte(strings.Repeat("payload ", 1000)))
	body, ok := newDecodedBody(bytes.NewReader(gz[:len(gz)/2]), []string{"gzip"})
	if !ok {
		t.Fatal("want ok=true")
	}
	if _, err := io.ReadAll(body); err == nil {
		t.Fatal("truncated gzip stream should return an error")
	}
}
