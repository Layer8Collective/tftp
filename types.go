package tftp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	// Entire tftp datagram size (without headers and trailers)
	DatagramSize = 516
	// the amount of data, in bytes, transferred in a single packet(without headers)
	BlockSize = DatagramSize - 4
)

// Specifies the TFTP message type.
type OperationCode uint16

const (
	OpRRQ OperationCode = iota + 1
	OpWRQ
	OpData
	OpAck
	OpErr
)

type ErrCode uint16

const (
	ErrUnknown ErrCode = iota
	ErrNotFound
	ErrAccessViolation
	ErrDiskFull
	ErrIllegalOp
	ErrUnknownID
	ErrFileExists
	ErrNoUser
)

/*
	RRQ / WRQ packet

	2 bytes     string    1 byte     string   1 byte
	------------------------------------------------
	| Opcode |  Filename  |   0  |    Mode    |   0  |
	------------------------------------------------
*/

type WriteReq struct {
	Filename string
	Mode     string
}

func (w *WriteReq) UnmarshalBinary(p []byte) error {
	b := bytes.NewBuffer(p)

	var code OperationCode

	err := binary.Read(b, binary.BigEndian, &code)

	if err != nil {
		return err
	}

	if code != OpWRQ {
		return errors.New("Invalid WRQ: " + fmt.Sprint(code))
	}

	w.Filename, err = b.ReadString(0)

	w.Filename = strings.TrimRight(w.Filename, "\x00")

	w.Mode, err = b.ReadString(0)
	w.Mode = strings.TrimRight(w.Mode, "\x00")

	if w.Mode != "octet" {
		return errors.New("No. We only accept data in octet mode.")
	}

	return nil
}

type ReadReq struct {
	Filename string
	Mode     string
}

func (q ReadReq) MarshalBinary() ([]byte, error) {
	mode := "octet"

	if q.Mode != "" {
		mode = q.Mode
	}

	cap := 2 + len(q.Filename) + 1 + len(q.Mode) + 1

	buffer := new(bytes.Buffer)

	buffer.Grow(cap)

	err := binary.Write(buffer, binary.BigEndian, OpRRQ)

	if err != nil {
		return nil, err
	}

	_, err = buffer.WriteString(q.Filename)
	if err != nil {
		return nil, err
	}

	err = buffer.WriteByte(0)
	if err != nil {
		return nil, err
	}

	_, err = buffer.WriteString(mode)
	if err != nil {
		return nil, err
	}

	err = buffer.WriteByte(0)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil

}

func (q *ReadReq) UnmarshalBinary(p []byte) error {
	r := bytes.NewBuffer(p)

	var code OperationCode

	// reading operation code first
	err := binary.Read(r, binary.BigEndian, &code)

	if err != nil {
		return err
	}

	if code != OpRRQ {
		return errors.New("Invalid RRQ: code is " + fmt.Sprint(code))
	}

	// reading filename
	q.Filename, err = r.ReadString(0)

	if err != nil {
		return fmt.Errorf("invalid RRQ: failed to read filename: %w", err)
	}

	q.Filename = strings.TrimRight(q.Filename, "\x00")

	q.Mode, err = r.ReadString(0)

	if err != nil {
		return fmt.Errorf("invalid RRQ: failed to read Mode: %w", err)
	}

	q.Mode = strings.TrimRight(q.Mode, "\x00")

	if len(q.Mode) == 0 {
		return fmt.Errorf("invalid RRQ: failed to read Mode: %w", err)
	}

	actual := strings.ToLower(q.Mode)

	if actual != "octet" {
		return errors.New("Only binary transfer supported. please use octet mode")
	}

	return nil
}

// TFTP Data Message Format
type Data struct {
	Block   uint16
	Payload io.Reader
}

func (d *Data) MarshalBinary() ([]byte, error) {
	b := new(bytes.Buffer)
	b.Grow(DatagramSize)

	d.Block++ // block numbers increment from 1

	err := binary.Write(b, binary.BigEndian, OpData) // write operation code
	if err != nil {
		return nil, err
	}

	err = binary.Write(b, binary.BigEndian, d.Block) // write block number
	if err != nil {
		return nil, err
	}

	// write up to BlockSize worth of bytes
	_, err = io.CopyN(b, d.Payload, BlockSize)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return b.Bytes(), nil
}

func (d *Data) UnmarshalBinary(p []byte) error {
	if l := len(p); l < 4 || l > DatagramSize {
		return errors.New("invalid DATA")
	}

	var opcode OperationCode

	err := binary.Read(bytes.NewReader(p[:2]), binary.BigEndian, &opcode)
	if err != nil || opcode != OpData {
		return errors.New("invalid DATA")
	}

	err = binary.Read(bytes.NewReader(p[2:4]), binary.BigEndian, &d.Block)
	if err != nil {
		return errors.New("invalid DATA")
	}

	d.Payload = bytes.NewBuffer(p[4:])

	return nil
}

type Ack struct {
	Block uint16
}

func (a Ack) MarshalBinary() ([]byte, error) {
	cap := 2 + 2

	buffer := new(bytes.Buffer)
	buffer.Grow(cap)

	err := binary.Write(buffer, binary.BigEndian, OpAck)

	if err != nil {
		return nil, err
	}

	err = binary.Write(buffer, binary.BigEndian, a.Block)

	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func (a *Ack) UnmarshalBinary(p []byte) error {
	var code OperationCode

	r := bytes.NewReader(p)

	err := binary.Read(r, binary.BigEndian, &code)
	if err != nil {
		return err
	}

	if code != OpAck {
		return errors.New("Invalid Operation")
	}

	return binary.Read(r, binary.BigEndian, &a.Block)
}

type Err struct {
	Error ErrCode
	// intended for human consumption
	Message string
}

func (e Err) MarshalBinary() ([]byte, error) {
	cap := 2 + 2 + len(e.Message) + 1

	buffer := new(bytes.Buffer)
	buffer.Grow(cap)

	err := binary.Write(buffer, binary.BigEndian, OpErr)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buffer, binary.BigEndian, e.Error)
	if err != nil {
		return nil, err
	}

	_, err = buffer.WriteString(e.Message)
	if err != nil {
		return nil, err
	}

	err = buffer.WriteByte(0)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func (e *Err) UnmarshalBinary(p []byte) error {
	r := bytes.NewBuffer(p)

	var opcode OperationCode

	err := binary.Read(r, binary.BigEndian, &opcode)

	if err != nil {
		return err
	}

	if opcode != OpErr {
		return errors.New("Invalid Error")
	}

	err = binary.Read(r, binary.BigEndian, &e.Error)

	if err != nil {
		return err
	}

	e.Message, err = r.ReadString(0)

	e.Message = strings.TrimRight(e.Message, "\x00")

	return err
}
