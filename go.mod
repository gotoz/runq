module github.com/gotoz/runq

go 1.21.10

replace github.com/gotoz/runq/pkg/vm => ./pkg/vm

require (
	github.com/creack/pty v1.1.21
	github.com/gotoz/runq/pkg/vm v0.0.0-00010101000000-000000000000
	github.com/mdlayher/vsock v1.2.1
	github.com/opencontainers/runtime-spec v1.2.0
	github.com/pmorjan/kmod v1.1.1
	github.com/seccomp/libseccomp-golang v0.10.0
	github.com/spf13/pflag v1.0.5
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/vishvananda/netlink v1.2.1-beta.2.0.20240425164735-856e190dd707
	golang.org/x/sys v0.20.0
	golang.org/x/term v0.20.0
)

require (
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/vishvananda/netns v0.0.5-0.20240501230406-261288576cd7 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
)
