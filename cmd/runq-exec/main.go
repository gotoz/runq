package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
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
	flag "github.com/spf13/pflag"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	certDefault = "/var/lib/runq/cert.pem"
	keyDefault  = "/var/lib/runq/key.pem"
)

var (
	certFile = flag.StringP("cert", "c", certDefault, "TLS certificate file")
	keyFile  = flag.StringP("key", "k", keyDefault, "TLS private key file")
	help     = flag.BoolP("help", "h", false, "print this help")
	stdin    = flag.BoolP("interactive", "i", false, "keep STDIN open even if not attached")
	tty      = flag.BoolP("tty", "t", false, "allocate a pseudo-TTY")
	version  = flag.BoolP("version", "v", false, "print version")

	exitCode      = 1
	gitCommit     string
	terminalState *terminal.State
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
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
}

func main() {
	mainMain()
	if terminalState != nil {
		terminal.Restore(0, terminalState)
	}
	os.Exit(exitCode)
}

func mainMain() {
	flag.CommandLine.SortFlags = false
	flag.SetInterspersed(false)
	flag.Usage = usage

	flag.Parse()
	if *help {
		flag.Usage()
		exitCode = 0
		return
	}
	if *version {
		fmt.Printf("%s (%s)\n", gitCommit, runtime.Version())
		exitCode = 0
		return
	}
	var cmdConf byte
	if *stdin {
		cmdConf |= vs.ConfStdin
	}
	if *tty {
		cmdConf |= vs.ConfTTY
		terminalState, _ = terminal.MakeRaw(0)
	}
	if flag.NArg() < 2 {
		flag.Usage()
		return
	}

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Print(err)
		return
	}
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	var cmdline [][]byte
	for _, v := range flag.Args()[1:] {
		cmdline = append(cmdline, []byte(v))
	}
	cmdBuf := bytes.Join(cmdline, []byte{0})

	containerID, err := realContainerID(flag.Arg(0))
	if err != nil {
		log.Print(err)
		return
	}

	// cid = vsock context ID
	// unique uint32 taken from first 4 bytes of the real container ID
	i, err := strconv.ParseUint(containerID[:8], 16, 32)
	if err != nil {
		log.Printf("invalid container id: %s", containerID)
		return
	}
	cid := uint32(i)

	// create job request
	conn, err := vsock.Dial(cid, vs.Port)
	if err != nil {
		log.Printf("failed to dial: %v", err)
		return
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		log.Print(err)
		return
	}

	buf := append([]byte{vs.ConnControl, cmdConf}, cmdBuf...)
	if _, err := tlsConn.Write(buf); err != nil {
		log.Printf("failed to send inital request: %v", err)
		return
	}

	buf = make([]byte, 10)
	_, err = tlsConn.Read(buf)
	if err != nil {
		log.Printf("failed to read job id: %v", err)
		return
	}
	jobid := string(buf)

	done := make(chan int)
	go execute(done, tlsConfig, cid, jobid)
	go wait(done, tlsConn)
	exitCode = <-done
}

// wait waits for early execution errors or the final exit code
func wait(done chan<- int, c *tls.Conn) {
	buf := make([]byte, 3)
	n, err := c.Read(buf)
	if err != nil {
		log.Printf("failed to read return code: %v", err)
		done <- 1
		return
	}

	if _, err := c.Write([]byte{vs.Done}); err != nil {
		log.Printf("failed to send ack message: %v", err)
	}

	exitCode, err := strconv.Atoi(string(buf[:n]))
	if err != nil {
		log.Printf("failed to parse exit code: ", err)
		done <- 1
		return
	}
	done <- exitCode
}

// execute executes the requested job witch stdin and stdout attached
func execute(done chan<- int, tlsConfig *tls.Config, cid uint32, jobid string) {
	conn, err := vsock.Dial(cid, vs.Port)
	if err != nil {
		log.Printf("failed to dial: %v", err)
		done <- 1
		return
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("tls handshake failed: %v", err)
		done <- 1
		return
	}

	buf := append([]byte{vs.ConnExecute}, jobid...)
	if _, err := tlsConn.Write(buf); err != nil {
		log.Printf("failed to write: %v", err)
		done <- 1
		return
	}

	if *stdin {
		go io.Copy(tlsConn, os.Stdin)
	}
	io.Copy(os.Stdout, tlsConn)
}

// realContainerID tries to find the real container id (64 hex chars) for a given identifier
// by a REST API call to the Docker engine.
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
