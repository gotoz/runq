package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/gotoz/runq/pkg/util"
	"github.com/gotoz/runq/pkg/vs"
	"github.com/kr/pty"
	"github.com/mdlayher/vsock"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

type Job struct {
	Cmd      *exec.Cmd
	Config   byte
	CtrlConn net.Conn
	Started  bool
}

var jobs map[string]Job
var mu sync.Mutex

func mainVsockd() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, unix.SIGTERM, unix.SIGUSR1, unix.SIGUSR2)
	go func() { <-ch }()

	jobs = make(map[string]Job)

	l, err := vsock.Listen(vs.Port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatalf("failed to accept: %v", err)
		}

		go func() {
			buf := make([]byte, 4096)
			n, err := c.Read(buf)
			if err != nil {
				log.Printf("%+v", errors.WithStack(err))
				c.Close()
				return
			}
			// 1st byte determines the connection type
			switch buf[0] {
			case vs.ConnControl:
				controlConnection(c, buf[1:n])
			case vs.ConnExecute:
				executeConnection(c, buf[1:n])
			default:
				log.Printf("invald connection type %#x", buf[0])
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
	config := buf[0]

	// remaining bytes are the command and arguments separated by \0
	var args []string
	for _, v := range bytes.Split(buf[1:], []byte{0}) {
		args = append(args, string(v))
	}

	job := Job{
		Cmd:      exec.Command(args[0], args[1:]...),
		Config:   config,
		CtrlConn: c,
	}

	id := util.RandStr(10)
	mu.Lock()
	jobs[id] = job
	mu.Unlock()

	c.Write([]byte(id))

	// cleanup if client does not execute the job within 1 second
	time.Sleep(time.Second)
	mu.Lock()
	job, exists := jobs[id]
	if exists && !job.Started {
		delete(jobs, id)
		c.Close()
	}
	defer mu.Unlock()
}

func executeConnection(c net.Conn, buf []byte) {
	id := string(buf)
	mu.Lock()
	job, exists := jobs[id]
	if !exists || job.Started == true {
		// client has sent invalid job id
		mu.Unlock()
		log.Printf("invalid id:%s", id)
		c.Close()
		return
	}
	job.Started = true
	jobs[id] = job
	mu.Unlock()

	var err error
	if job.Config&vs.ConfTTY > 0 {
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

	// cleanup resources
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

	if job.Config&vs.ConfStdin > 0 {
		go io.Copy(ptmx, c)
	}
	io.Copy(c, ptmx)
	return nil
}

func startCommandNoTTY(c net.Conn, job Job) error {
	var stdin io.WriteCloser
	var stdout, stderr io.ReadCloser
	var err error

	if job.Config&vs.ConfStdin > 0 {
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

	if job.Config&vs.ConfStdin > 0 {
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
