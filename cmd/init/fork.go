package main

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/gotoz/runq/pkg/vm"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func newEntrypoint(entrypoint vm.Entrypoint) (*exec.Cmd, error) {
	runtime.LockOSThread()
	dataReader, dataWriter, err := os.Pipe()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cmd := &exec.Cmd{
		Path:       "/proc/self/exe",
		Args:       append([]string{"entrypoint"}, entrypoint.Args...),
		ExtraFiles: []*os.File{dataReader},
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Stdin:      os.Stdin,
		SysProcAttr: &unix.SysProcAttr{
			Setsid:     true,
			Setctty:    true,
			Cloneflags: unix.CLONE_NEWPID | unix.CLONE_NEWNS | unix.CLONE_NEWIPC,
		},
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	if err := dataReader.Close(); err != nil {
		return nil, errors.WithStack(err)
	}

	gob, err := vm.Encode(entrypoint)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if _, err := dataWriter.Write(gob); err != nil {
		return nil, errors.WithStack(err)
	}
	if err := dataWriter.Close(); err != nil {
		return nil, errors.WithStack(err)
	}
	return cmd, nil
}

func newVsockd(vsockd vm.Vsockd, nspid int) (*exec.Cmd, error) {
	dataReader, dataWriter, err := os.Pipe()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cmd := &exec.Cmd{
		Path:       "/sbin/vsockd",
		Args:       []string{"vsockd", strconv.Itoa(nspid)},
		ExtraFiles: []*os.File{dataReader},
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Stdin:      os.Stdin,
		SysProcAttr: &unix.SysProcAttr{
			Setpgid: true,
		},
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if err := dataReader.Close(); err != nil {
		return nil, errors.WithStack(err)
	}

	gob, err := vm.Encode(vsockd)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if _, err := dataWriter.Write(gob); err != nil {
		return nil, errors.WithStack(err)
	}
	if err := dataWriter.Close(); err != nil {
		return nil, errors.WithStack(err)
	}
	return cmd, nil
}
