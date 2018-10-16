package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vm"
	"github.com/gotoz/runq/pkg/vs"
	"github.com/kr/pty"
	"github.com/mdlayher/vsock"
	"golang.org/x/sys/unix"
)

type jobRequest struct {
	cmd      *exec.Cmd
	conf     byte
	ctrlConn net.Conn
	started  bool
}

type jobDB struct {
	sync.Mutex
	m map[string]jobRequest
}

var jobs jobDB

func mainVsockd() {
	signal.Ignore(unix.SIGTERM, unix.SIGUSR1, unix.SIGUSR2)
	jobs = jobDB{m: make(map[string]jobRequest)}

	rd := os.NewFile(uintptr(3), "rd")
	if rd == nil {
		log.Fatal("failed to open pipe")
	}
	buf, err := ioutil.ReadAll(rd)
	if err != nil {
		log.Fatalf("failed to read from pipe: %v", err)
	}
	rd.Close()

	certs := bytes.Split(buf, []byte{0})
	if len(certs) != 3 {
		log.Fatal("date read from pipe is invalid")
	}

	certpool := x509.NewCertPool()
	if !certpool.AppendCertsFromPEM(certs[0]) {
		log.Fatalf("failed to parse CA certificate")
	}

	cert, err := tls.X509KeyPair(certs[1], certs[2])
	if err != nil {
		log.Fatalf("failed to parse certificate/key pair: %v", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certpool,
	}

	inner, err := vsock.Listen(vm.VsockPort)
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	defer inner.Close()

	l := tls.NewListener(inner, config)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("accept connection failed: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	c, ok := conn.(*tls.Conn)
	if !ok {
		log.Printf("invalid connection type: %T", conn)
		c.Close()
		return
	}

	if err := c.Handshake(); err != nil {
		log.Print(err)
		c.Close()
		return
	}

	addr, ok := c.RemoteAddr().(*vsock.Addr)
	if !ok {
		log.Printf("invalid remote address type: %T", c.RemoteAddr())
		c.Close()
		return
	}

	if addr.ContextID != 2 {
		log.Printf("invalid context ID from %s", addr)
		c.Close()
		return
	}

	// TODO:
	//   c.SetReadDeadline()
	//   requires go1.11+ and latest version of mdlayher/vsock
	buf := make([]byte, 4096)

	n, err := c.Read(buf)
	if err != nil {
		log.Printf("read failed: %v", err)
		c.Close()
		return
	}

	if n < 2 {
		log.Print("message too short")
		c.Close()
		return
	}

	// first byte determines the connection type
	switch buf[0] {
	case vs.ConnControl:
		controlConnection(c, buf[1:n])
	case vs.ConnExecute:
		executeConnection(c, buf[1:n])
	default:
		log.Printf("invalid connection type %#x\n", buf[0])
		c.Close()
	}
}

func controlConnection(c net.Conn, buf []byte) {
	if len(buf) < 2 {
		log.Print("controlConnection: not enough data")
		c.Close()
		return
	}

	// first byte is the config byte
	cmdConf := buf[0]

	// remaining bytes are the command and arguments separated by 0x0
	var args []string
	for _, v := range bytes.Split(buf[1:], []byte{0}) {
		args = append(args, string(v))
	}

	job := jobRequest{
		cmd:      exec.Command(args[0], args[1:]...),
		conf:     cmdConf,
		ctrlConn: c,
	}

	jobid := util.RandStr(8)
	jobs.Lock()
	jobs.m[jobid] = job
	jobs.Unlock()
	c.Write([]byte(jobid))

	// remove job request if it hasn't been started within 1 second
	time.Sleep(time.Second)
	jobs.Lock()
	job, exists := jobs.m[jobid]
	if exists && !job.started {
		delete(jobs.m, jobid)
		c.Close()
		log.Printf("removed unused job request %s", jobid)
	}
	jobs.Unlock()
}

func executeConnection(c net.Conn, buf []byte) {
	id := string(buf)
	jobs.Lock()
	job, exists := jobs.m[id]
	if !exists || job.started {
		jobs.Unlock()
		log.Printf("received invalid jobid: %q", id)
		c.Close()
		return
	}
	job.started = true
	jobs.m[id] = job
	jobs.Unlock()

	var err error
	if job.conf&vs.ConfTTY > 0 {
		err = startCommandWithTTY(c, job)
	} else {
		err = startCommandNoTTY(c, job)
	}

	if err == nil {
		err = job.cmd.Wait()
	}

	// process exit message and return code
	rc, msg := util.ErrorToRc(err)
	if msg != "" {
		if _, err := c.Write([]byte(msg + "\n")); err != nil {
			log.Printf("failed to write exit message: %v", err)
		}
	}

	buf = []byte(strconv.Itoa(int(rc)))
	if _, err := job.ctrlConn.Write(buf); err != nil {
		log.Printf("failed to write exit code: %v", err)
	}

	// wait for acknowledge message
	done := make(chan int, 1)
	go func() {
		buf := make([]byte, 1)
		job.ctrlConn.Read(buf)
		done <- 1
	}()
	select {
	case <-done:
	case <-time.After(time.Second * 3):
	}

	jobs.Lock()
	job, exists = jobs.m[id]
	if exists {
		job.ctrlConn.Close()
		c.Close()
		delete(jobs.m, id)
	}
	jobs.Unlock()
}

func startCommandWithTTY(c net.Conn, job jobRequest) error {
	ptmx, err := pty.Start(job.cmd)
	if err != nil {
		return err
	}
	defer ptmx.Close()

	if job.conf&vs.ConfStdin > 0 {
		go io.Copy(ptmx, c)
	}
	io.Copy(c, ptmx)
	return nil
}

func startCommandNoTTY(c net.Conn, job jobRequest) error {
	var stdin io.WriteCloser
	var stdout, stderr io.ReadCloser
	var err error

	if job.conf&vs.ConfStdin > 0 {
		stdin, err = job.cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("create pipe STDIN failed: %v", err)
		}
	}
	if stdout, err = job.cmd.StdoutPipe(); err != nil {
		return fmt.Errorf("create pipe STDOUT failed: %v", err)
	}
	if stderr, err = job.cmd.StderrPipe(); err != nil {
		return fmt.Errorf("create pipe STDERR failed: %v", err)
	}

	if err := job.cmd.Start(); err != nil {
		return err
	}

	if job.conf&vs.ConfStdin > 0 {
		go io.Copy(stdin, c)
	}

	done := make(chan int)
	go func() {
		io.Copy(c, stdout)
		done <- 1
	}()

	io.Copy(c, stderr)
	<-done
	return nil
}
