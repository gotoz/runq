package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
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
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

type Job struct {
	Cmd      *exec.Cmd
	Conf     byte
	CtrlConn net.Conn
	Started  bool
}

var jobs map[string]Job
var mu sync.Mutex

func mainVsockd() {
	signal.Ignore(unix.SIGTERM, unix.SIGUSR1, unix.SIGUSR2)
	jobs = make(map[string]Job)

	rd := os.NewFile(uintptr(3), "rd")
	if rd == nil {
		log.Fatal("failed to open pipe")
	}
	buf, err := ioutil.ReadAll(rd)
	if err != nil {
		log.Fatalf("failed to read key from pipe: %v", err)
	}
	rd.Close()

	certs := bytes.Split(buf, []byte{0})
	if len(certs) != 3 {
		log.Fatal("failed to read PEMs from pipe")
	}

	certpool := x509.NewCertPool()
	if !certpool.AppendCertsFromPEM(certs[0]) {
		log.Fatalf("can't parse CA pem")
	}

	cert, err := tls.X509KeyPair(certs[1], certs[2])
	if err != nil {
		log.Fatalf("can't parse certificate: %v", err)
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
			log.Printf("accept failed: %v", err)
			continue
		}

		c, ok := conn.(*tls.Conn)
		if !ok {
			log.Printf("invalid connection type: %T", conn)
			c.Close()
			continue
		}

		if err := c.Handshake(); err != nil {
			log.Print(err)
			c.Close()
			continue
		}

		addr, ok := c.RemoteAddr().(*vsock.Addr)
		if !ok {
			log.Printf("invalid remote address type: %T", c.RemoteAddr())
			c.Close()
			continue
		}

		if addr.ContextID != 2 {
			log.Print("invalid connection from %v", addr)
			c.Close()
			continue
		}

		// TODO:
		//   c.SetReadDeadline()
		//   requires go1.11+ and latest version of mdlayher/vsock
		go func() {
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
		}()
	}
}

func controlConnection(c net.Conn, buf []byte) {
	if len(buf) < 2 {
		log.Print("invalid message")
		c.Close()
		return
	}

	// first byte is the config byte
	cmdConf := buf[0]

	// remaining bytes are the command and arguments separated by \0
	var args []string
	for _, v := range bytes.Split(buf[1:], []byte{0}) {
		args = append(args, string(v))
	}

	job := Job{
		Cmd:      exec.Command(args[0], args[1:]...),
		Conf:     cmdConf,
		CtrlConn: c,
	}

	jobid := util.RandStr(10)
	mu.Lock()
	jobs[jobid] = job
	mu.Unlock()

	c.Write([]byte(jobid))

	// cleanup if client does not execute the job within 1 second
	time.Sleep(time.Second)
	mu.Lock()
	job, exists := jobs[jobid]
	if exists && !job.Started {
		delete(jobs, jobid)
		c.Close()
	}
	mu.Unlock()
}

func executeConnection(c net.Conn, buf []byte) {
	id := string(buf)
	mu.Lock()
	job, exists := jobs[id]
	if !exists || job.Started {
		// client has sent invalid job id
		mu.Unlock()
		log.Printf("invalid jobid:%s", id)
		c.Close()
		return
	}
	job.Started = true
	jobs[id] = job
	mu.Unlock()

	var err error
	if job.Conf&vs.ConfTTY > 0 {
		err = startCommandWithTTY(c, job)
	} else {
		err = startCommandNoTTY(c, job)
	}

	if err == nil {
		err = job.Cmd.Wait()
	}

	// process exit message and return code
	rc, msg := util.ErrorToRc(err)
	if msg != "" {
		if _, err := c.Write([]byte(msg + "\n")); err != nil {
			log.Printf("failed to write exit message: %v", err)
		}
	}

	buf = []byte(strconv.Itoa(int(rc)))
	if _, err := job.CtrlConn.Write(buf); err != nil {
		log.Printf("failed to write  rc: %v", err)
	}

	// wait for acknowledge message
	done := make(chan int, 1)
	go func() {
		buf := make([]byte, 1)
		job.CtrlConn.Read(buf)
		done <- 1
	}()
	select {
	case <-done:
	case <-time.After(time.Second * 3):
	}

	mu.Lock()
	job, exists = jobs[id]
	if exists {
		job.CtrlConn.Close()
		c.Close()
		delete(jobs, id)
	}
	mu.Unlock()
}

func startCommandWithTTY(c net.Conn, job Job) error {
	ptmx, err := pty.Start(job.Cmd)
	if err != nil {
		return err
	}
	defer ptmx.Close()

	if job.Conf&vs.ConfStdin > 0 {
		go io.Copy(ptmx, c)
	}
	io.Copy(c, ptmx)
	return nil
}

func startCommandNoTTY(c net.Conn, job Job) error {
	var stdin io.WriteCloser
	var stdout, stderr io.ReadCloser
	var err error

	if job.Conf&vs.ConfStdin > 0 {
		stdin, err = job.Cmd.StdinPipe()
		if err != nil {
			return errors.WithStack(err)
		}
	}
	if stdout, err = job.Cmd.StdoutPipe(); err != nil {
		return errors.WithStack(err)
	}
	if stderr, err = job.Cmd.StderrPipe(); err != nil {
		return errors.WithStack(err)
	}

	if err := job.Cmd.Start(); err != nil {
		return err
	}

	if job.Conf&vs.ConfStdin > 0 {
		go io.Copy(stdin, c)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		io.Copy(c, stdout)
		wg.Done()
	}()

	io.Copy(c, stderr)
	wg.Wait()
	return nil
}
