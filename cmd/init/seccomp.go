package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
	libseccomp "github.com/seccomp/libseccomp-golang"
	"golang.org/x/sys/unix"
)

func canDoSeccomp() (bool, error) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return false, err
	}
	defer f.Close()

	var res bool
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "Seccomp:") {
			res = true
			break
		}
	}
	if err := sc.Err(); err != nil {
		return false, err
	}
	return res, nil
}

func initSeccomp(seccompGob []byte) error {
	if len(seccompGob) == 0 {
		// Filter can be empty via "--security-opt seccomp=unconfined"
		return nil
	}

	ok, err := canDoSeccomp()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("Kernel does not support seccomp")
	}

	dec := gob.NewDecoder(bytes.NewBuffer(seccompGob))
	sec := new(specs.LinuxSeccomp)
	if err := dec.Decode(sec); err != nil {
		return fmt.Errorf("dec.Decode LinuxSeccomp failed: %w", err)
	}

	defaultAction, err := convertAction(sec.DefaultAction)
	if err != nil {
		return err
	}

	filter, err := libseccomp.NewFilter(defaultAction)
	if err != nil {
		return fmt.Errorf("libseccomp.NewFilter failed: %w", err)
	}

	for _, a := range sec.Architectures {
		arch, err := convertArch(a)
		if err != nil {
			return err
		}
		scmpArch, err := libseccomp.GetArchFromString(arch)
		if err != nil {
			return fmt.Errorf("libseccomp.GetArchFromString failed: %w", err)
		}
		if err := filter.AddArch(scmpArch); err != nil {
			return fmt.Errorf("filter.AddArch failed: %w", err)
		}
	}

	if err := filter.SetNoNewPrivsBit(false); err != nil {
		return fmt.Errorf("set no new privileges failed: %v", err)
	}

	for _, sc := range sec.Syscalls {
		for _, name := range sc.Names {
			if name == "" {
				return errors.New("empty syscall name")
			}

			// Ignore syscall names that can't be resolved.
			// Same as in runc.
			call, err := libseccomp.GetSyscallFromName(name)
			if err != nil {
				continue
			}

			action, err := convertAction(sc.Action)
			if err != nil {
				return err
			}

			// The default errno for SCMP_ACT_ERRNO is EPERM which breaks the new
			// syscall "clone3" used in latest glibc 2.34. ENOSYS is required for
			// "clone3" to trigger the fallback to "clone". Unfortunately we don't get
			// the errno from the OCI spec and therefore we have to set it manually
			// for now. For details see
			// https://github.com/moby/moby/issues/42680 and
			// https://github.com/moby/moby/commit/9f6b562d
			if name == "clone3" && sc.Action == "SCMP_ACT_ERRNO" {
				action = action.SetReturnCode(int16(syscall.ENOSYS))
			}

			if len(sc.Args) == 0 {
				if err := filter.AddRule(call, action); err != nil {
					return fmt.Errorf("seccomp filter.AddRule failed: syscall=%s : %w", name, err)
				}
				continue
			}

			var conditions []libseccomp.ScmpCondition
			for _, arg := range sc.Args {
				op, err := convertOperator(arg.Op)
				if err != nil {
					return err
				}
				condition, err := libseccomp.MakeCondition(arg.Index, op, arg.Value, arg.ValueTwo)
				if err != nil {
					return fmt.Errorf("libseccomp.MakeCondition failed: %w", err)
				}
				conditions = append(conditions, condition)
			}
			if err := filter.AddRuleConditional(call, action, conditions); err != nil {
				return fmt.Errorf("seccomp filter.AddRuleConditional failed: syscall=%s : %w", name, err)
			}
		}
	}

	if err = filter.Load(); err != nil {
		return fmt.Errorf("error loading seccomp filter into kernel: %w", err)
	}
	return nil
}

var actions = map[specs.LinuxSeccompAction]libseccomp.ScmpAction{
	"SCMP_ACT_KILL":  libseccomp.ActKill,
	"SCMP_ACT_ERRNO": libseccomp.ActErrno.SetReturnCode(int16(unix.EPERM)),
	"SCMP_ACT_TRAP":  libseccomp.ActTrap,
	"SCMP_ACT_ALLOW": libseccomp.ActAllow,
	"SCMP_ACT_TRACE": libseccomp.ActTrace.SetReturnCode(int16(unix.EPERM)),
}

func convertAction(a specs.LinuxSeccompAction) (libseccomp.ScmpAction, error) {
	act, ok := actions[a]
	if !ok {
		return 0, fmt.Errorf("seccomp: invalid action %v", a)
	}
	return act, nil
}

var archs = map[specs.Arch]string{
	"SCMP_ARCH_X86":         "x86",
	"SCMP_ARCH_X86_64":      "amd64",
	"SCMP_ARCH_X32":         "x32",
	"SCMP_ARCH_ARM":         "arm",
	"SCMP_ARCH_AARCH64":     "arm64",
	"SCMP_ARCH_MIPS":        "mips",
	"SCMP_ARCH_MIPS64":      "mips64",
	"SCMP_ARCH_MIPS64N32":   "mips64n32",
	"SCMP_ARCH_MIPSEL":      "mipsel",
	"SCMP_ARCH_MIPSEL64":    "mipsel64",
	"SCMP_ARCH_MIPSEL64N32": "mipsel64n32",
	"SCMP_ARCH_PPC":         "ppc",
	"SCMP_ARCH_PPC64":       "ppc64",
	"SCMP_ARCH_PPC64LE":     "ppc64le",
	"SCMP_ARCH_S390":        "s390",
	"SCMP_ARCH_S390X":       "s390x",
}

func convertArch(a specs.Arch) (string, error) {
	arch, ok := archs[a]
	if !ok {
		return "", fmt.Errorf("seccomp: invalid arch %v", a)
	}
	return arch, nil
}

var operators = map[specs.LinuxSeccompOperator]libseccomp.ScmpCompareOp{
	"SCMP_CMP_NE":        libseccomp.CompareNotEqual,
	"SCMP_CMP_LT":        libseccomp.CompareLess,
	"SCMP_CMP_LE":        libseccomp.CompareLessOrEqual,
	"SCMP_CMP_EQ":        libseccomp.CompareEqual,
	"SCMP_CMP_GE":        libseccomp.CompareGreaterEqual,
	"SCMP_CMP_GT":        libseccomp.CompareGreater,
	"SCMP_CMP_MASKED_EQ": libseccomp.CompareMaskedEqual,
}

func convertOperator(in specs.LinuxSeccompOperator) (libseccomp.ScmpCompareOp, error) {
	operator, ok := operators[in]
	if !ok {
		return 0, fmt.Errorf("seccomp: invalid operator %v", in)
	}
	return operator, nil
}
