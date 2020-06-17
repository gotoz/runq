package main

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gotoz/runq/internal/util"
	"github.com/gotoz/runq/internal/vs"
	"github.com/gotoz/runq/pkg/vm"
	"github.com/mdlayher/vsock"
	"github.com/pkg/errors"
)

var (
	gitCommit     string // set via Makefile
	entrypointEnv []string
	entrypointPid string
	jobs          jobDB
)

func init() {
	runtime.LockOSThread()
	log.SetPrefix(fmt.Sprintf("[%s(%d) %s] ", filepath.Base(os.Args[0]), os.Getpid(), gitCommit))
	log.SetFlags(log.Lmicroseconds)
}

type jobExecution struct {
	cmd       *exec.Cmd
	ctrlConn  net.Conn
	started   bool
	withStdin bool
	withTTY   bool
}

type jobDB struct {
	sync.Mutex
	m map[vs.JobID]jobExecution
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("%+v", err)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("Usage: %s <pid of entrypoint>", os.Args[0])
	}
	entrypointPid = os.Args[1]
	if _, err := strconv.Atoi(entrypointPid); err != nil {
		return fmt.Errorf("invalid pid of entrypoint : %v", err)
	}

	rd := os.NewFile(uintptr(3), "rd")
	if rd == nil {
		return errors.New("rd == nil")
	}

	buf, err := ioutil.ReadAll(rd)
	if err != nil {
		return errors.WithStack(err)
	}
	if err := rd.Close(); err != nil {
		return errors.WithStack(err)
	}

	vsockd, err := vm.DecodeVsockdGob(buf)
	if err != nil {
		return errors.WithStack(err)
	}

	entrypointEnv = vsockd.EntrypointEnv

	jobs = jobDB{
		m: make(map[vs.JobID]jobExecution),
	}

	certpool := x509.NewCertPool()
	if !certpool.AppendCertsFromPEM(vsockd.CACert) {
		return errors.New("failed to parse CA certificate")
	}

	cert, err := tls.X509KeyPair(vsockd.Cert, vsockd.Key)
	if err != nil {
		return errors.WithMessage(err, "failed to parse certificate/key pair")
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certpool,
	}

	inner, err := vsock.Listen(vs.Port)
	if err != nil {
		return fmt.Errorf("listen failed: %v", err)
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
	case vs.TypeControlConn:
		controlConnection(c, buf[1:n])
	case vs.TypeExecuteConn:
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

	jr, err := vs.DecodeJobRequest(buf)
	if err != nil {
		log.Printf("can't decode JobRequest: %v", err)
		c.Close()
		return
	}

	env := append([]string{}, entrypointEnv...)
	env = append(env, jr.Env...)

	args := append([]string{"nsenter", entrypointPid}, jr.Args...)

	job := jobExecution{
		cmd: &exec.Cmd{
			Path: "/sbin/nsenter",
			Args: args,
			Env:  env,
		},
		ctrlConn:  c,
		withStdin: jr.WithStdin,
		withTTY:   jr.WithTTY,
	}

	var id vs.JobID
	_, err = rand.Read(id[:])
	if err != nil {
		log.Print(err)
		c.Close()
		return
	}

	jobs.Lock()
	jobs.m[id] = job
	jobs.Unlock()
	c.Write(id[:])

	// remove job request if it hasn't been started within 1 second
	time.Sleep(time.Second)
	jobs.Lock()
	job, exists := jobs.m[id]
	if exists && !job.started {
		delete(jobs.m, id)
		c.Close()
		log.Printf("removed unused job request %v", id)
	}
	jobs.Unlock()
}

func executeConnection(c net.Conn, buf []byte) {
	var id vs.JobID
	copy(id[:], buf)
	jobs.Lock()
	job, exists := jobs.m[id]
	if !exists || job.started {
		jobs.Unlock()
		log.Printf("received invalid jobid: %v", id)
		c.Close()
		return
	}
	job.started = true
	jobs.m[id] = job
	jobs.Unlock()

	var err error
	if job.withTTY {
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

func startCommandWithTTY(c net.Conn, job jobExecution) error {
	ptmx, err := pty.Start(job.cmd)
	if err != nil {
		return err
	}
	defer ptmx.Close()

	if job.withStdin {
		go io.Copy(ptmx, c)
	}
	io.Copy(c, ptmx)
	return nil
}

func startCommandNoTTY(c net.Conn, job jobExecution) error {
	var stdin io.WriteCloser
	var stdout, stderr io.ReadCloser
	var err error

	if job.withStdin {
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

	if job.withStdin {
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
