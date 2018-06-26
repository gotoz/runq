// Package vm defines data types and functions define and
// share data between the runtime runq, proxy and init.
package vm

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/gob"
	"io/ioutil"
	"net"

	"github.com/vishvananda/netlink"
)

// A Msgtype declares the type of a message to be
// exchanged between proxy and init.
type Msgtype uint8

// Msg defines the format of the data exchanged between proxy and init.
type Msg struct {
	Type Msgtype
	Data []byte
}

// Disktype represents a valid disk types.
type Disktype int

// Rlimit details
type Rlimit struct {
	Hard uint64
	Soft uint64
}

// AppCapabilities defines whitelists of Linux capabilities
// for the target application.
type AppCapabilities struct {
	Ambient     []string
	Bounding    []string
	Effective   []string
	Inheritable []string
	Permitted   []string
}

// Network defines a network interface.
type Network struct {
	Name       string
	MvtName    string
	MvtIndex   int
	MacAddress string
	MTU        int
	Addrs      []netlink.Addr
	Gateway    net.IP
}

// Disk defines a disk.
type Disk struct {
	Path   string
	Dir    string
	Cache  string
	Serial string
	Fstype string
	Type   Disktype
}

// Mount defines a mount point.
type Mount struct {
	Source string
	Target string
	Fstype string
	Flags  int
	Data   string
}

// User specifies user information for the VM process.
type User struct {
	UID            uint32
	GID            uint32
	AdditionalGids []uint32
}

// Process contains information to start an application inside the VM.
type Process struct {
	User
	Args            []string
	Capabilities    AppCapabilities
	Cwd             string
	Env             []string
	NoNewPrivileges bool
	Rlimits         map[string]Rlimit
	SeccompGob      []byte
	Terminal        bool
}

// Linux contains the configuration of the VM.
type Linux struct {
	ContainerID string
	CPU         int
	Disks       []Disk
	DNS         []string
	DNSOpts     string
	DNSSearch   string
	GitCommit   string
	Hostname    string
	Mem         int
	Mounts      []Mount
	NestedVM    bool
	Networks    []Network
	Sigusr      bool
	Sysctl      map[string]string
}

// Data contains all data needed by the VM.
type Data struct {
	Process
	Linux
}

// Encode encodes a data struct into binary Gob.
func Encode(e interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(e); err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

// DecodeDataGob decodes a Gob binary buffer into a Data struct.
func DecodeDataGob(buf []byte) (*Data, error) {
	dec := gob.NewDecoder(bytes.NewBuffer(buf))
	v := new(Data)
	if err := dec.Decode(v); err != nil {
		return nil, err
	}
	return v, nil
}

// DecodeProcessGob decodes a Gob binary buffer into a Process struct.
func DecodeProcessGob(buf []byte) (*Process, error) {
	dec := gob.NewDecoder(bytes.NewBuffer(buf))
	v := new(Process)
	if err := dec.Decode(v); err != nil {
		return nil, err
	}
	return v, nil
}

// ZipEncode encodes a data struct into a gzip binary Gob.
func ZipEncode(e interface{}) ([]byte, error) {
	var buf bytes.Buffer
	wr := gzip.NewWriter(&buf)
	enc := gob.NewEncoder(wr)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	if err := wr.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ZipDecodeBase64 decodes a base64 string, into a data struct.
func ZipDecodeBase64(str string) (*Data, error) {
	s, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(s)
	rd, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	res, err := ioutil.ReadAll(rd)
	if err != nil {
		return nil, err
	}

	if err := rd.Close(); err != nil {
		return nil, err
	}
	return DecodeDataGob(res)
}
