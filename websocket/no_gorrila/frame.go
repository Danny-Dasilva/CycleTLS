package main

import (
	"bytes"
	"encoding/binary"
)

type Frame struct {
	IsFragment bool // if the
	Opcode     byte
	Reserved   byte
	IsMasked   bool
	Length     uint64
	Payload    []byte
}

// Get the Pong frame
func (f Frame) Pong() Frame {
	f.Opcode = 10
	return f
}

// Get Text Payload
func (f Frame) Text() string {
	return string(f.Payload)
}

// IsControl checks if the frame is a control frame identified by opcodes where the most significant bit of the opcode is 1
func (f *Frame) IsControl() bool {
	return f.Opcode&0x08 == 0x08
}

func (f *Frame) HasReservedOpcode() bool {
	return f.Opcode > 10 || (f.Opcode >= 3 && f.Opcode <= 7)
}
func (f *Frame) CloseCode() uint16 {
	var code uint16
	binary.Read(bytes.NewReader(f.Payload), binary.BigEndian, &code)
	return code
}