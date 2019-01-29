// Package vs is used for the comunication via vsockets.
package vs

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"regexp"
	"strconv"
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

// ContextID returns a (uint32) number based on the given input string.
// The input string must consists of at least 8 hexadecimal characters.
func ContextID(id string) (uint32, error) {
	ok, _ := regexp.MatchString("^[0-9a-fA-F]{8,}", id)
	if !ok {
		return 0, fmt.Errorf("ContextID: invalid id: %s", id)
	}
	i, err := strconv.ParseUint(id[:8], 16, 32)
	if i < 3 || i == 1<<32-1 { // reserved
		return 0, fmt.Errorf("ContextID: invalid id: %s", id)
	}
	return uint32(i), err
}
