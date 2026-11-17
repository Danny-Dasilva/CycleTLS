package cycletls

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// parseContentEncoding flattens Content-Encoding header values into the list
// of codings in the order the server applied them. Names are lower-cased and
// "identity" is dropped. ok is false when a coding is not one we can decode.
func parseContentEncoding(values []string) (codings []string, ok bool) {
	for _, value := range values {
		for _, token := range strings.Split(value, ",") {
			coding := strings.ToLower(strings.TrimSpace(token))
			switch coding {
			case "", "identity":
				continue
			case "x-gzip":
				coding = "gzip"
			case "brotli":
				coding = "br"
			}
			switch coding {
			case "gzip", "deflate", "br", "zstd":
				codings = append(codings, coding)
			default:
				return nil, false
			}
		}
	}
	return codings, true
}

// contentDecoder decodes one content coding as the body is read, so a response
// can be decompressed chunk by chunk without buffering it.
type contentDecoder struct {
	src     *bufio.Reader
	coding  string
	r       io.Reader
	closeFn func()
}

// open picks the decoder on the first read, once the first body bytes are
// available to look at.
func (d *contentDecoder) open() io.Reader {
	head, _ := d.src.Peek(2)
	if len(head) == 0 {
		// Empty body: nothing to decode, let the source report EOF or its error.
		return d.src
	}

	switch d.coding {
	case "gzip":
		// A body labelled gzip that does not start with the gzip magic bytes
		// is passed through unchanged, as DecompressBody does.
		if len(head) < 2 || head[0] != 0x1f || head[1] != 0x8b {
			return d.src
		}
		gz, err := gzip.NewReader(d.src)
		if err != nil {
			return errReader{err}
		}
		d.closeFn = func() { gz.Close() }
		return gz
	case "deflate":
		// "deflate" means a zlib stream (RFC 9110), but some servers send a
		// raw DEFLATE stream. Tell them apart by the zlib header check.
		if len(head) == 2 && head[0]&0x0f == 8 && (uint16(head[0])<<8|uint16(head[1]))%31 == 0 {
			zr, err := zlib.NewReader(d.src)
			if err != nil {
				return errReader{err}
			}
			d.closeFn = func() { zr.Close() }
			return zr
		}
		fr := flate.NewReader(d.src)
		d.closeFn = func() { fr.Close() }
		return fr
	case "br":
		return brotli.NewReader(d.src)
	case "zstd":
		zr, err := zstd.NewReader(d.src, zstd.WithDecoderConcurrency(1))
		if err != nil {
			return errReader{err}
		}
		d.closeFn = zr.Close
		return zr
	}
	return d.src
}

func (d *contentDecoder) Read(p []byte) (int, error) {
	if d.r == nil {
		d.r = d.open()
	}
	return d.r.Read(p)
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

// decodedBody reads a response body with its content codings removed.
type decodedBody struct {
	io.Reader
	decoders []*contentDecoder
}

// Close releases the decoders. It does not close the underlying body.
func (b *decodedBody) Close() error {
	for _, d := range b.decoders {
		if d.closeFn != nil {
			d.closeFn()
		}
	}
	return nil
}

// newDecodedBody wraps body so that reads return the payload with the codings
// named in the Content-Encoding header values removed. It handles gzip,
// deflate, br and zstd, in any letter case, and several codings in one header
// ("gzip, br"), which it undoes in reverse order.
//
// ok is false, and body is returned untouched, when there is nothing to decode
// or the header names a coding we don't know.
func newDecodedBody(body io.Reader, contentEncoding []string) (decoded io.ReadCloser, ok bool) {
	codings, known := parseContentEncoding(contentEncoding)
	if !known || len(codings) == 0 {
		return io.NopCloser(body), false
	}

	out := &decodedBody{Reader: body}
	for i := len(codings) - 1; i >= 0; i-- {
		d := &contentDecoder{src: bufio.NewReader(out.Reader), coding: codings[i]}
		out.decoders = append(out.decoders, d)
		out.Reader = d
	}
	return out, true
}
