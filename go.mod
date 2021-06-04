module github.com/gotoz/runq

go 1.16

replace github.com/gotoz/runq/pkg/vm => ./pkg/vm

require (
	github.com/creack/pty v1.1.13
	github.com/gotoz/runq/pkg/vm v0.0.0-00010101000000-000000000000
	github.com/mdlayher/vsock v0.0.0-20210303205602-10d591861736
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/pkg/errors v0.9.1
	github.com/pmorjan/kmod v0.0.0-20200620073327-4889ff2a5685
	github.com/seccomp/libseccomp-golang v0.9.1
	github.com/spf13/pflag v1.0.5
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/vishvananda/netlink v1.1.0
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	golang.org/x/sys v0.0.0-20210603081109-ebe580a85c40
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56 // indirect
)
