package cycletls

import (
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
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

func zstdEncode(t *testing.T, src []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(src); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUnZstdDataValid(t *testing.T) {
	enc := zstdEncode(t, []byte("hello zstd"))

	got, err := unZstdData(enc)
	if err != nil {
		t.Fatalf("valid zstd: %v", err)
	}
	if string(got) != "hello zstd" {
		t.Fatalf("got %q", got)
	}
}

func TestDecompressBodyZstd(t *testing.T) {
	enc := zstdEncode(t, []byte("hello zstd"))

	for _, ev := range []string{"zstd", "ZSTD", " zstd "} {
		if got := DecompressBody(enc, []string{ev}, nil); string(got) != "hello zstd" {
			t.Errorf("encoding %q: got %q", ev, got)
		}
	}
}

func TestDecompressBodyZstdInvalidReturnsOriginal(t *testing.T) {
	in := []byte("not zstd")
	if got := DecompressBody(in, []string{"zstd"}, nil); !bytes.Equal(got, in) {
		t.Errorf("should return original body on error, got %q", got)
	}
}

func TestUnZstdDataCapsSize(t *testing.T) {
	enc := zstdEncode(t, make([]byte, maxDecompressedBody+1))
	if _, err := unZstdData(enc); err == nil {
		t.Fatal("expected size limit error")
	}
}
