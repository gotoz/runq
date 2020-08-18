module github.com/gotoz/runq

go 1.15

replace github.com/gotoz/runq/pkg/vm => ./pkg/vm

require (
	github.com/creack/pty v1.1.11
	github.com/gotoz/runq/pkg/vm v0.0.0-00010101000000-000000000000
	github.com/mdlayher/vsock v0.0.0-20200508120832-7ad3638b3fbc
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/pkg/errors v0.9.1
	github.com/pmorjan/kmod v0.0.0-20200620073327-4889ff2a5685
	github.com/seccomp/libseccomp-golang v0.9.1
	github.com/spf13/pflag v1.0.5
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1
)
