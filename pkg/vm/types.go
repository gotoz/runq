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

// Msgtype declares the type of a message.
type Msgtype uint8

// Message types
const (
	_       Msgtype = iota
	Command         // command to execute
	Signal          // IPC signal such as SIGTERM
	Vmdata          // VM config data
)

// Msg defines the format of the data exchanged between proxy and init.
type Msg struct {
	Type Msgtype
	Data []byte
}

// Disktype represents a valid disk types.
type Disktype int

// known disk types
const (
	DisktypeUnknown Disktype = iota // disk type is not known
	BlockDevice                     // regular block device
	Qcow2Image                      // Qcow2 image
	RawFile                         // regular file used as block device
)

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
	MacAddress string
	MTU        int
	Addrs      []netlink.Addr
	Gateway    net.IP
	TapDevice  string
}

// Disk defines a disk.
type Disk struct {
	Cache  string
	Dir    string
	Fstype string
	ID     string
	Mount  bool
	Path   string
	Serial string
	Type   Disktype
}

// Mount defines a mount point.
type Mount struct {
	Data   string
	Flags  int
	Fstype string
	ID     string
	Source string
	Target string
}

// User specifies user information for the VM process.
type User struct {
	UID            uint32
	GID            uint32
	AdditionalGids []uint32
}

// Certificates definenes TLS certificates
type Certificates struct {
	CACert []byte
	Cert   []byte
	Key    []byte
}

// Entrypoint contains information of entrypoint.
type Entrypoint struct {
	User
	Args            []string
	Capabilities    AppCapabilities
	Cwd             string
	DockerInit      string
	Env             []string
	NoNewPrivileges bool
	Rlimits         map[string]Rlimit
	SeccompGob      []byte
	Terminal        bool
}

// Vsockd contains config data for the vsockd process.
type Vsockd struct {
	Certificates
	EntrypointPid int
	EntrypointEnv []string
	CID           uint32
}

// DNS contains dns configuration.
type DNS struct {
	Server  []string
	Options string
	Search  string
}

// Data contains all data needed by the VM.
type Data struct {
	APDevice    string
	ContainerID string
	CPU         int
	Disks       []Disk
	DNS         DNS
	GitCommit   string
	Hostname    string
	Mem         int
	Mounts      []Mount
	NestedVM    bool
	Networks    []Network
	NoExec      bool
	Sysctl      map[string]string
	Entrypoint  Entrypoint
	Vsockd      Vsockd
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

// DecodeEntrypointGob decodes a Gob binary buffer into a Entrypoint struct.
func DecodeEntrypointGob(buf []byte) (*Entrypoint, error) {
	dec := gob.NewDecoder(bytes.NewBuffer(buf))
	v := new(Entrypoint)
	if err := dec.Decode(v); err != nil {
		return nil, err
	}
	return v, nil
}

// DecodeVsockdGob decodes a Gob binary buffer into a Vsockd struct.
func DecodeVsockdGob(buf []byte) (*Vsockd, error) {
	dec := gob.NewDecoder(bytes.NewBuffer(buf))
	v := new(Vsockd)
	if err := dec.Decode(v); err != nil {
		return nil, err
	}
	return v, nil
}

// ZipEncodeBase64 encodes arbritray data into a gziped binary Gob
// and returns it encoded as base64.
func ZipEncodeBase64(data interface{}) (string, error) {
	var buf bytes.Buffer
	wr := gzip.NewWriter(&buf)
	enc := gob.NewEncoder(wr)
	if err := enc.Encode(data); err != nil {
		return "", err
	}
	if err := wr.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
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
