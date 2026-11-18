package cycletls

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

// buildInit2Packet builds an "init2" packet: the same fields as "init", with a
// uint32 length in front of the options JSON.
func buildInit2Packet(requestID string, windowSize uint32, optionsJSON string) []byte {
	const method = "init2"
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, uint16(len(requestID)))
	b.WriteString(requestID)
	binary.Write(&b, binary.BigEndian, uint16(len(method)))
	b.WriteString(method)
	binary.Write(&b, binary.BigEndian, windowSize)
	binary.Write(&b, binary.BigEndian, uint32(len(optionsJSON)))
	b.WriteString(optionsJSON)
	return b.Bytes()
}

func optionsWithBody(t *testing.T, body string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]interface{}{
		"url":    "http://example.com/upload",
		"method": "POST",
		"body":   body,
	})
	if err != nil {
		t.Fatalf("marshal options: %v", err)
	}
	return string(raw)
}

func TestParseInitMessage_Init2(t *testing.T) {
	sizes := map[string]int{
		"empty body":    0,
		"small body":    100,
		"just over u16": 70000,
		"100 KB":        100000,
		"1 MB":          1 << 20,
		"8 MB":          8 << 20,
	}
	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			body := strings.Repeat("a", size)
			data := buildInit2Packet("req-big", 65536, optionsWithBody(t, body))

			req, window, err := parseInitMessage(data)
			if err != nil {
				t.Fatalf("parseInitMessage(init2) failed: %v", err)
			}
			if req.RequestID != "req-big" {
				t.Fatalf("request id = %q, want req-big", req.RequestID)
			}
			if window != 65536 {
				t.Fatalf("window = %d, want 65536", window)
			}
			if req.Options.Method != "POST" || req.Options.URL != "http://example.com/upload" {
				t.Fatalf("options not decoded: method=%q url=%q", req.Options.Method, req.Options.URL)
			}
			if req.Options.Body != body {
				t.Fatalf("body length = %d, want %d", len(req.Options.Body), len(body))
			}
		})
	}
}

// The original packet must keep parsing: clients older than 3.0.1 send nothing else.
func TestParseInitMessage_LegacyInitStillParses(t *testing.T) {
	body := strings.Repeat("b", 60000)
	data := buildInitPacket("req-old", 65536, optionsWithBody(t, body))

	req, window, err := parseInitMessage(data)
	if err != nil {
		t.Fatalf("parseInitMessage(init) failed: %v", err)
	}
	if req.RequestID != "req-old" || window != 65536 {
		t.Fatalf("got id=%q window=%d", req.RequestID, window)
	}
	if req.Options.Body != body {
		t.Fatalf("body length = %d, want %d", len(req.Options.Body), len(body))
	}
}

// Both layouts must decode the same options to the same request.
func TestParseInitMessage_InitAndInit2Agree(t *testing.T) {
	options := `{"url":"https://example.com/a","method":"PUT","body":"x","headers":{"A":"1","B":["2","3"]},"timeout":-1}`

	oldReq, oldWindow, err := parseInitMessage(buildInitPacket("req-1", 4096, options))
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	newReq, newWindow, err := parseInitMessage(buildInit2Packet("req-1", 4096, options))
	if err != nil {
		t.Fatalf("init2: %v", err)
	}

	if oldWindow != newWindow {
		t.Fatalf("window: init=%d init2=%d", oldWindow, newWindow)
	}
	oldJSON, _ := json.Marshal(oldReq)
	newJSON, _ := json.Marshal(newReq)
	if !bytes.Equal(oldJSON, newJSON) {
		t.Fatalf("requests differ:\ninit:  %s\ninit2: %s", oldJSON, newJSON)
	}
	if newReq.Options.Timeout != -1 {
		t.Fatalf("timeout = %d, want -1", newReq.Options.Timeout)
	}
}

func TestParseInitMessage_Init2OverLimit(t *testing.T) {
	// Only the declared length matters: the check runs before any body is read,
	// so the test does not need to allocate 32 MiB.
	const method = "init2"
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, uint16(5))
	b.WriteString("req-1")
	binary.Write(&b, binary.BigEndian, uint16(len(method)))
	b.WriteString(method)
	binary.Write(&b, binary.BigEndian, uint32(65536))
	binary.Write(&b, binary.BigEndian, uint32(MaxInitOptionsLen+1))
	b.WriteString("{}")

	_, _, err := parseInitMessage(b.Bytes())
	if !errors.Is(err, ErrInitOptionsTooLarge) {
		t.Fatalf("err = %v, want ErrInitOptionsTooLarge", err)
	}
}

func TestParseInitMessage_Init2Malformed(t *testing.T) {
	const options = `{"url":"http://example.com"}`
	valid := buildInit2Packet("req-1", 65536, options)

	t.Run("truncated options", func(t *testing.T) {
		_, _, err := parseInitMessage(valid[:len(valid)-3])
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("err = %v, want io.ErrUnexpectedEOF", err)
		}
	})

	t.Run("missing options length", func(t *testing.T) {
		// requestID + method + window, then two bytes of a four-byte length.
		head := len(valid) - len(options) - 4
		_, _, err := parseInitMessage(valid[:head+2])
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("err = %v, want io.ErrUnexpectedEOF", err)
		}
	})

	t.Run("trailing bytes", func(t *testing.T) {
		_, _, err := parseInitMessage(append(append([]byte{}, valid...), 0xFF))
		if !errors.Is(err, ErrTrailingBytes) {
			t.Fatalf("err = %v, want ErrTrailingBytes", err)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		_, _, err := parseInitMessage(buildInit2Packet("req-1", 65536, `{"url":`))
		if err == nil {
			t.Fatal("expected a JSON error")
		}
	})

	t.Run("window too small", func(t *testing.T) {
		_, _, err := parseInitMessage(buildInit2Packet("req-1", 0, `{}`))
		if !errors.Is(err, ErrWindowTooSmall) {
			t.Fatalf("err = %v, want ErrWindowTooSmall", err)
		}
	})

	t.Run("unknown method", func(t *testing.T) {
		bad := bytes.Replace(valid, []byte("init2"), []byte("init3"), 1)
		_, _, err := parseInitMessage(bad)
		if err == nil || !strings.Contains(err.Error(), "unexpected method") {
			t.Fatalf("err = %v, want unexpected method", err)
		}
	})
}

// An "init" layout sent under the "init2" name (or the reverse) must fail and
// not be read as a different request.
func TestParseInitMessage_LayoutMismatchRejected(t *testing.T) {
	options := `{"url":"http://example.com"}`

	asInit2 := bytes.Replace(buildInitPacket("req-1", 65536, options), []byte("\x00\x04init"), []byte("\x00\x05init2"), 1)
	if _, _, err := parseInitMessage(asInit2); err == nil {
		t.Fatal("u16 options under init2 should not parse")
	}

	asInit := bytes.Replace(buildInit2Packet("req-1", 65536, options), []byte("\x00\x05init2"), []byte("\x00\x04init"), 1)
	if _, _, err := parseInitMessage(asInit); err == nil {
		t.Fatal("u32 options under init should not parse")
	}
}
