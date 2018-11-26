package main

import (
	"bufio"
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
	tlsCertDefault = "/var/lib/runq/cert.pem"
	tlsKeyDefault  = "/var/lib/runq/key.pem"
)

var (
	env     = flag.StringArrayP("env", "e", nil, "Set environment variables for command")
	help    = flag.BoolP("help", "h", false, "Print this help")
	stdin   = flag.BoolP("interactive", "i", false, "Keep STDIN open even if not attached")
	tlsCert = flag.StringP("tlscert", "c", tlsCertDefault, "TLS certificate file")
	tlsKey  = flag.StringP("tlskey", "k", tlsKeyDefault, "TLS private key file")
	tty     = flag.BoolP("tty", "t", false, "Allocate a pseudo-TTY")
	version = flag.BoolP("version", "v", false, "Print version")

	gitCommit     string
	terminalState *terminal.State
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprint(os.Stderr, "  runq-exec [options] <container> command args\n\n")
	fmt.Fprint(os.Stderr, "Run a command in a running runq container\n\n")
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "\nEnvironment Variable:")
	fmt.Fprintln(os.Stderr, "  DOCKER_HOST    specifies the Docker daemon socket.")
	fmt.Fprintln(os.Stderr, "\nExample:")
	fmt.Fprint(os.Stderr, "  runq-exec -ti a6c3b7c bash\n\n")
}

func main() {
	flag.CommandLine.SortFlags = false
	flag.SetInterspersed(false)
	flag.Usage = usage
	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(0)
	}
	if *version {
		fmt.Printf("%s (%s)\n", gitCommit, runtime.Version())
		os.Exit(0)
	}
	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}

	rc := run()
	if terminalState != nil {
		terminal.Restore(0, terminalState)
	}
	os.Exit(rc)
}

func run() int {
	jr := vs.JobRequest{
		Args:      flag.Args()[1:],
		Env:       *env,
		WithTTY:   *tty,
		WithStdin: *stdin,
	}

	cert, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
	if err != nil {
		log.Print(err)
		return 1
	}
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	containerID, err := realContainerID(flag.Arg(0))
	if err != nil {
		log.Print(err)
		return 1
	}

	// cid = vsock context ID
	// unique uint32 taken from first 4 bytes of the real container ID
	i, err := strconv.ParseUint(containerID[:8], 16, 32)
	if err != nil {
		log.Printf("invalid container id: %s", containerID)
		return 1
	}
	cid := uint32(i)

	conn, err := vsock.Dial(cid, vs.Port)
	if err != nil {
		log.Printf("failed to dial: %v", err)
		return 1
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, tlsConfig)

	jrGob, err := jr.Encode()
	if err != nil {
		log.Printf("failed to encode JobRequest: %v", err)
		return 1
	}

	buf := append([]byte{vs.TypeControlConn}, jrGob...)
	if _, err := tlsConn.Write(buf); err != nil {
		log.Printf("failed to send inital request: %v", err)
		return 1
	}

	var jobid vs.JobID
	_, err = tlsConn.Read(jobid[:])
	if err != nil {
		log.Printf("failed to read job id: %v", err)
		return 1
	}

	done := make(chan int, 1)
	go execute(done, tlsConfig, cid, jobid)
	go wait(done, tlsConn)
	return <-done
}

// wait waits for early execution errors of the requested job or the final exit code
func wait(done chan<- int, c *tls.Conn) {
	buf := make([]byte, 3)
	n, err := c.Read(buf)
	if err != nil {
		log.Printf("failed to read exit code: %v", err)
		done <- 1
		return
	}

	if _, err := c.Write([]byte{vs.Done}); err != nil {
		log.Printf("failed to send ack message: %v", err)
	}

	exitCode, err := strconv.Atoi(string(buf[:n]))
	if err != nil {
		log.Printf("failed to parse exit code: %v", err)
		exitCode = 1
	}
	done <- exitCode
}

// execute creates a second vsock connection to execute the requested job inside the VM.
// Incomming data is deliverd to STDOUT. STDIN is deliverd to the job process.
func execute(done chan<- int, tlsConfig *tls.Config, cid uint32, jobid vs.JobID) {
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

	buf := append([]byte{vs.TypeExecuteConn}, jobid[:]...)
	if _, err := tlsConn.Write(buf); err != nil {
		log.Printf("failed to write: %v", err)
		done <- 1
		return
	}

	if *tty {
		terminalState, _ = terminal.MakeRaw(0)
	}
	if *stdin {
		go io.Copy(tlsConn, os.Stdin)
	}
	io.Copy(os.Stdout, tlsConn)
}

// realContainerID tries to find the real container id (64 hex characters) for a
// given identifier by executing a REST API call to the Docker engine.
func realContainerID(id string) (string, error) {
	// re taken from: https://github.com/moby/moby/blob/master/daemon/names/names.go
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
