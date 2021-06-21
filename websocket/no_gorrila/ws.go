package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"
)

const bufferSize = 4096

type Conn interface {
	Close() error
}

var closeCodes map[int]string = map[int]string{
	1000: "NormalError",
	1001: "GoingAwayError",
	1002: "ProtocolError",
	1003: "UnknownType",
	1007: "TypeError",
	1008: "PolicyError",
	1009: "MessageTooLargeError",
	1010: "ExtensionError",
	1011: "UnexpectedError",
}

func getAcceptHash(key string) string {
	h := sha1.New()
	h.Write([]byte(key))
	h.Write([]byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

type Ws struct {
	conn   Conn
	bufrw  *bufio.ReadWriter
	header http.Header
	status uint16
}

// New hijacks the http request and returns Ws
func New(w http.ResponseWriter, req *http.Request) (*Ws, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("webserver doesn't support http hijacking")
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	return &Ws{conn, bufrw, req.Header, 1000}, nil
}

// Handshake performs the initial websocket handshake
func (ws *Ws) Handshake() error {
	hash := getAcceptHash(ws.header.Get("Sec-WebSocket-Key"))
	lines := []string{
		"HTTP/1.1 101 Web Socket Protocol Handshake",
		"Server: go/echoserver",
		"Upgrade: WebSocket",
		"Connection: Upgrade",
		"Sec-WebSocket-Accept: " + hash,
		"", // required for extra CRLF
		"", // required for extra CRLF
	}
	return ws.write([]byte(strings.Join(lines, "\r\n")))
}

func (ws *Ws) write(data []byte) error {
	if _, err := ws.bufrw.Write(data); err != nil {
		return err
	}
	return ws.bufrw.Flush()
}

func (ws *Ws) read(size int) ([]byte, error) {
	data := make([]byte, 0)
	for {
		if len(data) == size {
			break
		}
		// Temporary slice to read chunk
		sz := bufferSize
		remaining := size - len(data)
		if sz > remaining {
			sz = remaining
		}
		temp := make([]byte, sz)

		n, err := ws.bufrw.Read(temp)
		if err != nil && err != io.EOF {
			return data, err
		}

		data = append(data, temp[:n]...)
	}
	return data, nil
}

func (ws *Ws) validate(fr *Frame) error {
	if !fr.IsMasked {
		ws.status = 1002
		return errors.New("protocol error: unmasked client frame")
	}
	if fr.IsControl() && (fr.Length > 125 || fr.IsFragment) {
		ws.status = 1002
		return errors.New("protocol error: all control frames MUST have a payload length of 125 bytes or less and MUST NOT be fragmented")
	}
	if fr.HasReservedOpcode() {
		ws.status = 1002
		return errors.New("protocol error: opcode " + fmt.Sprintf("%x", fr.Opcode) + " is reserved")
	}
	if fr.Reserved > 0 {
		ws.status = 1002
		return errors.New("protocol error: RSV " + fmt.Sprintf("%x", fr.Reserved) + " is reserved")
	}
	if fr.Opcode == 1 && !fr.IsFragment && !utf8.Valid(fr.Payload) {
		ws.status = 1007
		return errors.New("wrong code: invalid UTF-8 text message ")
	}
	if fr.Opcode == 8 {
		if fr.Length >= 2 {
			code := binary.BigEndian.Uint16(fr.Payload[:2])
			reason := utf8.Valid(fr.Payload[2:])
			if code >= 5000 || (code < 3000 && closeCodes[int(code)] == "") {
				ws.status = 1002
				return errors.New(closeCodes[1002] + " Wrong Code")
			}
			if fr.Length > 2 && !reason {
				ws.status = 1007
				return errors.New(closeCodes[1007] + " invalid UTF-8 reason message")
			}
		} else if fr.Length != 0 {
			ws.status = 1002
			return errors.New(closeCodes[1002] + " Wrong Code")
		}
	}
	return nil
}

// Recv receives data and returns a Frame
func (ws *Ws) Recv() (Frame, error) {
	frame := Frame{}
	head, err := ws.read(2)
	if err != nil {
		return frame, err
	}

	frame.IsFragment = (head[0] & 0x80) == 0x00
	frame.Opcode = head[0] & 0x0F
	frame.Reserved = (head[0] & 0x70)

	frame.IsMasked = (head[1] & 0x80) == 0x80

	var length uint64
	length = uint64(head[1] & 0x7F)

	if length == 126 {
		data, err := ws.read(2)
		if err != nil {
			return frame, err
		}
		length = uint64(binary.BigEndian.Uint16(data))
	} else if length == 127 {
		data, err := ws.read(8)
		if err != nil {
			return frame, err
		}
		length = uint64(binary.BigEndian.Uint64(data))
	}
	mask, err := ws.read(4)
	if err != nil {
		return frame, err
	}
	frame.Length = length

	payload, err := ws.read(int(length)) // possible data loss
	if err != nil {
		return frame, err
	}

	for i := uint64(0); i < length; i++ {
		payload[i] ^= mask[i%4]
	}
	frame.Payload = payload
	err = ws.validate(&frame)
	return frame, err
}

// Send sends a Frame
func (ws *Ws) Send(fr Frame) error {
	data := make([]byte, 2)
	data[0] = 0x80 | fr.Opcode
	if fr.IsFragment {
		data[0] &= 0x7F
	}

	if fr.Length <= 125 {
		data[1] = byte(fr.Length)
		data = append(data, fr.Payload...)
	} else if fr.Length > 125 && float64(fr.Length) < math.Pow(2, 16) {
		data[1] = byte(126)
		size := make([]byte, 2)
		binary.BigEndian.PutUint16(size, uint16(fr.Length))
		data = append(data, size...)
		data = append(data, fr.Payload...)
	} else if float64(fr.Length) >= math.Pow(2, 16) {
		data[1] = byte(127)
		size := make([]byte, 8)
		binary.BigEndian.PutUint64(size, fr.Length)
		data = append(data, size...)
		data = append(data, fr.Payload...)
	}
	return ws.write(data)
}

// Close sends close frame and closes the TCP connection
func (ws *Ws) Close() error {
	f := Frame{}
	f.Opcode = 8
	f.Length = 2
	f.Payload = make([]byte, 2)
	binary.BigEndian.PutUint16(f.Payload, ws.status)
	if err := ws.Send(f); err != nil {
		return err
	}
	return ws.conn.Close()
}