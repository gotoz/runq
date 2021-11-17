module github.com/gotoz/runq

go 1.16

replace github.com/gotoz/runq/pkg/vm => ./pkg/vm

require (
	github.com/creack/pty v1.1.15
	github.com/gotoz/runq/pkg/vm v0.0.0-00010101000000-000000000000
	github.com/mdlayher/vsock v0.0.0-20210303205602-10d591861736
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/pkg/errors v0.9.1
	github.com/pmorjan/kmod v1.0.0
	github.com/seccomp/libseccomp-golang v0.9.2-0.20211028222634-77bddc247e72
	github.com/spf13/pflag v1.0.5
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/crypto v0.0.0-20210915214749-c084706c2272
	golang.org/x/sys v0.0.0-20210915083310-ed5796bab164
)
