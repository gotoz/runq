package cfg

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// MinMem declares the minimum amount of RAM a VM in MiB.
const MinMem = 64

// KernelParameters defines kernel boot parameters.
const KernelParameters = "console=ttyS0 panic=1 module.sig_enforce=1 loglevel=3"

// Envfile contains all entrypoint environment variables.
const Envfile = "/.runqenv"

// ReaperInterval defines the frequency of the process reaper.
var ReaperInterval = time.Second * 60

// SysctlDefault defines default system settings.
var SysctlDefault = map[string]string{
	"fs.file-max":                        "9223372036854775807", // max int64
	"kernel.panic_on_oops":               "1",
	"net.ipv6.conf.all.disable_ipv6":     "1",
	"net.ipv6.conf.default.disable_ipv6": "1",
	"vm.overcommit_memory":               "0",
	"vm.panic_on_oom":                    "0",
}

// SysctlOverride defines system settings that can't be changed.
var SysctlOverride = map[string]string{
	"kernel.kexec_load_disabled": "1",
}

// Rlimits defines process settings.
var Rlimits = map[string]syscall.Rlimit{
	"RLIMIT_NOFILE":     {Max: 65536, Cur: 65536},
	"RLIMIT_NPROC":      {Max: unix.RLIM_INFINITY, Cur: unix.RLIM_INFINITY},
	"RLIMIT_SIGPENDING": {Max: 65536, Cur: 65536},
}

// Signals that proxy catches and forwards to init.
var Signals = []os.Signal{
	syscall.SIGHUP,
	syscall.SIGINT,
	syscall.SIGQUIT,
	syscall.SIGTERM,
	syscall.SIGUSR1,
	syscall.SIGUSR2,
	syscall.SIGCONT,
	syscall.SIGSTOP,
}

// RlimitsMap maps OCI rlimit types to unix flags.
var RlimitsMap = map[string]int{
	"RLIMIT_AS":         unix.RLIMIT_AS,
	"RLIMIT_CORE":       unix.RLIMIT_CORE,
	"RLIMIT_CPU":        unix.RLIMIT_CPU,
	"RLIMIT_DATA":       unix.RLIMIT_DATA,
	"RLIMIT_FSIZE":      unix.RLIMIT_FSIZE,
	"RLIMIT_LOCKS":      unix.RLIMIT_LOCKS,
	"RLIMIT_MEMLOCK":    unix.RLIMIT_MEMLOCK,
	"RLIMIT_MSGQUEUE":   unix.RLIMIT_MSGQUEUE,
	"RLIMIT_NICE":       unix.RLIMIT_NICE,
	"RLIMIT_NOFILE":     unix.RLIMIT_NOFILE,
	"RLIMIT_NPROC":      unix.RLIMIT_NPROC,
	"RLIMIT_RSS":        unix.RLIMIT_RSS,
	"RLIMIT_RTPRIO":     unix.RLIMIT_RTPRIO,
	"RLIMIT_RTTIME":     unix.RLIMIT_RTTIME,
	"RLIMIT_SIGPENDING": unix.RLIMIT_SIGPENDING,
	"RLIMIT_STACK":      unix.RLIMIT_STACK,
}

// ReadonlyPaths sets the provided paths as RO inside the VM.
var ReadonlyPaths = []string{"/proc/bus", "/proc/sysrq-trigger"}

// MaskedPaths masks over the provided paths inside the VM.
var MaskedPaths = []string{
	"/proc/kcore", "/proc/latency_stats", "/proc/timer_list", "/proc/timer_stats",
	"/proc/sched_debug", "/proc/scsi", "/sys/firmware",
}
