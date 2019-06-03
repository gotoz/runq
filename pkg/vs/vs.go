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

// control bytes
const (
	TypeControlConn byte = iota // connection to submit a new job
	TypeExecuteConn             // connection to start a job and handle IO
	Done                        // indicates that the job has finished
)

// JobID is used to connect a control connection to an execute connection.
type JobID [4]byte

// JobRequest defines a command to run inside a runq vm.
type JobRequest struct {
	Args      []string
	Env       []string
	WithStdin bool
	WithTTY   bool
}

// Encode encodes a job request into gob binary format.
func (jr JobRequest) Encode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(jr); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodeJobRequest decodes a byte buffer into a job request object.
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
