package cycletls

import (
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/andybalholm/brotli"
)

func TestUnBrotliDataValid(t *testing.T) {
	var buf bytes.Buffer
	w := brotli.NewWriter(&buf)
	if _, err := w.Write([]byte("hello cycle")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := unBrotliData(buf.Bytes())
	if err != nil {
		t.Fatalf("valid brotli: %v", err)
	}
	if string(got) != "hello cycle" {
		t.Fatalf("got %q", got)
	}

	decoded := DecompressBody(buf.Bytes(), []string{"br"}, nil)
	if string(decoded) != "hello cycle" {
		t.Fatalf("DecompressBody got %q", decoded)
	}
}

func TestUnBrotliDataInvalidDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("invalid brotli panicked: %v", r)
		}
	}()

	inputs := [][]byte{
		{},
		{0x00},
		[]byte("not brotli"),
		{0x06, 0x00},
		bytes.Repeat([]byte{0xff}, 64),
	}
	for _, in := range inputs {
		got, err := unBrotliData(in)
		out := DecompressBody(in, []string{"br"}, nil)
		if err != nil {
			if !bytes.Equal(out, in) {
				t.Errorf("DecompressBody should return original body on error, got %q for %v (unz=%q err=%v)", out, in, got, err)
			}
			continue
		}
		_ = got
	}
}

func TestReadLimitedCapsSize(t *testing.T) {
	r := bytes.NewReader(make([]byte, maxDecompressedBody+1))
	if _, err := readLimited(r); err == nil {
		t.Fatal("expected size limit error")
	}
}

func TestGUnzipDataValid(t *testing.T) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte("hello gzip")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := gUnzipData(buf.Bytes())
	if err != nil {
		t.Fatalf("valid gzip: %v", err)
	}
	if string(got) != "hello gzip" {
		t.Fatalf("got %q", got)
	}
}
