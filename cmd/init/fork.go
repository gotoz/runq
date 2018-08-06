package main

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/gotoz/runq/pkg/vm"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

type forker struct {
	process vm.Process
}

func newForker(process vm.Process) *forker {
	return &forker{
		process: process,
	}
}

func (f forker) forkEntryPoint() *child {
	return &child{
		process: f.process,
	}
}

func (f forker) forkVsockd(certs vm.Certificates) *child {
	return &child{
		process: vm.Process{
			Args:         []string{"/proc/self/exe", "vsockd"},
			Env:          f.process.Env,
			Certificates: certs,
			Type:         vm.Vsockd,
		},
	}
}

type child struct {
	process vm.Process
	pid     int
}

func (c *child) start() error {
	// pipe to forward vmdata to our child
	dataReader, dataWriter, err := os.Pipe()
	if err != nil {
		return errors.WithStack(err)
	}

	// pipe for the child to send back the PID of grandchild
	pidReader, pidWriter, err := os.Pipe()
	if err != nil {
		return errors.WithStack(err)
	}

	cmd := &exec.Cmd{
		Path:       "/proc/self/exe",
		Args:       append([]string{"child"}, c.process.Args...),
		ExtraFiles: []*os.File{dataReader, pidWriter},
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Stdin:      os.Stdin,
		SysProcAttr: &syscall.SysProcAttr{
			Setpgid: true,
		},
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	if err := dataReader.Close(); err != nil {
		return errors.WithStack(err)
	}
	if err := pidWriter.Close(); err != nil {
		return errors.WithStack(err)
	}

	processGob, err := vm.Encode(c.process)
	if err != nil {
		return errors.WithStack(err)
	}

	ch := make(chan error, 1)
	go func() {
		if _, err := dataWriter.Write(processGob); err != nil {
			ch <- errors.WithStack(err)
			return
		}
		if err := dataWriter.Close(); err != nil {
			ch <- errors.WithStack(err)
			return
		}
		ch <- cmd.Wait()
	}()
	select {
	case err := <-ch:
		if err != nil {
			return err
		}
	case <-time.After(time.Millisecond * 2000):
		cmd.Process.Kill()
		return errors.New("waiting for child timed out")
	}

	// Read PID of grandchild from pipe.
	buf, err := ioutil.ReadAll(pidReader)
	if err != nil {
		return errors.WithStack(err)
	}
	if err = pidReader.Close(); err != nil {
		return errors.WithStack(err)
	}
	if len(buf) != 4 {
		return errors.Errorf("received %d bytes from pipe. Expected: 4", len(buf))
	}
	c.pid = int(binary.BigEndian.Uint32(buf))
	return nil
}

func (c *child) wait() {
	var wstatus unix.WaitStatus
	_, err := unix.Wait4(c.pid, &wstatus, 0, nil)
	if c.process.Type == vm.Entrypoint {
		switch {
		case err != nil:
			shutdown(1, fmt.Sprintf("%+v", errors.WithStack(err)))
		case wstatus.Exited():
			shutdown(uint8(wstatus.ExitStatus()), "")
		case wstatus.Signaled():
			// bash like: 128 + signal number
			shutdown(128+uint8(wstatus.Signal()), "")
		}
		return
	}
	if err != nil {
		log.Print(err)
	}
}

func (c *child) signal(sig syscall.Signal) error {
	p, err := os.FindProcess(c.pid)
	if err != nil {
		return errors.WithStack(err)
	}
	return p.Signal(sig)
}
