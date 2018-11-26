// Package vs is used for the comunication via vsockets.
package vs

import (
	"bytes"
	"encoding/gob"
)

// Port is the listening port number of vsockd process.
const Port = 1

const (
	TypeControlConn byte = iota
	TypeExecuteConn
	Done
)

type JobID [4]byte

type JobRequest struct {
	Args      []string
	Env       []string
	WithStdin bool
	WithTTY   bool
}

func (jr JobRequest) Encode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(jr); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecodeJobRequest(buf []byte) (*JobRequest, error) {
	dec := gob.NewDecoder(bytes.NewBuffer(buf))
	ec := new(JobRequest)
	if err := dec.Decode(ec); err != nil {
		return nil, err
	}
	return ec, nil
}
