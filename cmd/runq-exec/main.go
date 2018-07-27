package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/gotoz/runq/pkg/vs"
	"github.com/mdlayher/vsock"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	gitCommit     string
	terminalState *terminal.State
)

func init() {
	// support POSIX style flags
	for i, v := range os.Args {
		switch v {
		case "-ti", "-it":
			os.Args[i] = "-t"
			os.Args = append(os.Args[:i], append([]string{"-i"}, os.Args[i:]...)...)
		}
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  runq-exec [options] <container> command args\n")
	fmt.Fprintln(os.Stderr, "Run a command in a running runq container\n")
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "\nEnvironment Variable:")
	fmt.Fprintln(os.Stderr, "  DOCKER_HOST    specifies the Docker daemon socket.")
	fmt.Fprintln(os.Stderr, "\nExample:")
	fmt.Fprintln(os.Stderr, "  runq-exec -ti a6c3b7c bash\n")
	os.Exit(1)
}

func exit(rc int, msg string) {
	if terminalState != nil {
		terminal.Restore(0, terminalState)
	}
	if msg != "" {
		fmt.Fprintln(os.Stderr, msg)
	}
	os.Exit(rc)
}

func main() {
	_ = flag.Bool("h", false, "print this help")
	stdin := flag.Bool("i", false, "keep STDIN open even if not attached")
	tty := flag.Bool("t", false, "allocate a pseudo-TTY")
	version := flag.Bool("v", false, "print version")
	flag.Usage = usage
	flag.Parse()
	if *version {
		fmt.Printf("%s (%s)\n", gitCommit, runtime.Version())
		os.Exit(0)
	}
	if flag.NArg() < 2 {
		flag.Usage()
	}

	config := vs.ConfDefault
	if *stdin {
		config |= vs.ConfStdin
	}
	if *tty {
		config |= vs.ConfTTY
		terminalState, _ = terminal.MakeRaw(0)
	}

	var cmdline [][]byte
	for _, v := range flag.Args()[1:] {
		cmdline = append(cmdline, []byte(v))
	}
	cmdlineBuf := bytes.Join(cmdline, []byte{0})

	containerID, err := realContainerID(flag.Arg(0))
	if err != nil {
		exit(1, err.Error())
	}

	// cid: vsock context ID
	// - unique 32 bit number
	// - calculated from first 4 bytes of the real container ID
	cid, err := strconv.ParseUint(containerID[:8], 16, 32)
	if err != nil {
		exit(1, "invalid container id: "+containerID)
	}

	// create ctrl connection
	c, err := vsock.Dial(uint32(cid), uint32(vs.Port))
	if err != nil {
		exit(1, "failed to dial: "+err.Error())
	}
	defer c.Close()

	// send initial request
	buf := append([]byte{vs.ConnControl, config}, cmdlineBuf...)
	if _, err := c.Write(buf); err != nil {
		exit(1, "failed to send inital request: "+err.Error())
	}

	// read job id response
	buf = make([]byte, 10)
	_, err = c.Read(buf)
	if err != nil {
		exit(1, "failed to read job id: "+err.Error())
	}
	jobID := string(buf)

	// create execute connection
	go execute(cid, jobID, *stdin)

	// read job return code
	buf = make([]byte, 3)
	n, err := c.Read(buf)
	if err != nil {
		exit(1, "failed to read return code: "+err.Error())
	}

	// send acknowledge message
	if _, err := c.Write([]byte{vs.Done}); err != nil {
		log.Printf("failed to ack message: " + err.Error())
	}

	rc, err := strconv.Atoi(string(buf[:n]))
	if err != nil {
		exit(1, "failed to parse return code: "+err.Error())
	}
	exit(rc, "")
}

func execute(cid uint64, jobID string, stdin bool) {
	c, err := vsock.Dial(uint32(cid), uint32(vs.Port))
	if err != nil {
		exit(1, "failed to dial: "+err.Error())
	}
	defer c.Close()

	buf := append([]byte{vs.ConnExecute}, jobID...)
	if _, err := c.Write(buf); err != nil {
		exit(1, "failed to write: "+err.Error())
	}

	if stdin {
		go io.Copy(c, os.Stdin)
	}
	io.Copy(os.Stdout, c)
}

// realContainerID tries to find the real container id (64 hex chars) for a given identifier
// by a simple REST API call to the Docker engine.
func realContainerID(id string) (string, error) {
	// taken from: https://github.com/moby/moby/blob/master/daemon/names/names.go
	var re = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	if !re.MatchString(id) {
		return "", fmt.Errorf("invalid container id %s", id)
	}

	network, address := "unix", "/var/run/docker.sock"
	if v := os.Getenv("DOCKER_HOST"); v != "" {
		s := strings.SplitN(v, "://", 2)
		if len(s) != 2 {
			return "", fmt.Errorf("invalid DOCKER_HOST env: %s", v)
		}
		network, address = s[0], s[1]
	}

	c, err := net.Dial(network, address)
	if err != nil {
		return "", err
	}
	defer c.Close()

	data := struct {
		HostConfig struct {
			Runtime string `json:"Runtime"`
		} `json:"HostConfig"`
		ID      string `json:"Id"`
		Message string `json:"message"`
		State   struct {
			Status string `json:"Status"`
		} `json:"State"`
	}{}

	if _, err := fmt.Fprintf(c, "GET /containers/%s/json HTTP/1.0\r\n\r\n", id); err != nil {
		return "", err
	}
	rd := bufio.NewReader(c)
	for {
		line, err := rd.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal(line, &data); err == nil {
			break
		}
	}

	if data.Message != "" {
		return "", fmt.Errorf(data.Message)
	}
	if data.HostConfig.Runtime != "runq" {
		return "", fmt.Errorf("container runtime is not runq")
	}
	if data.State.Status != "running" {
		return "", fmt.Errorf("container is not running")
	}
	if data.ID == "" {
		return "", fmt.Errorf("container not found")
	}

	return data.ID, nil
}
