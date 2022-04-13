module github.com/gotoz/runq

go 1.17

replace github.com/gotoz/runq/pkg/vm => ./pkg/vm

require (
	github.com/creack/pty v1.1.18
	github.com/gotoz/runq/pkg/vm v0.0.0-00010101000000-000000000000
	github.com/mdlayher/vsock v1.1.1
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/pkg/errors v0.9.1
	github.com/pmorjan/kmod v1.0.0
	github.com/seccomp/libseccomp-golang v0.9.2-0.20220202234545-e214ef109e10
	github.com/spf13/pflag v1.0.5
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad
	golang.org/x/term v0.0.0-20220411215600-e5f449aeb171
)

require (
	github.com/mdlayher/socket v0.2.0 // indirect
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df // indirect
	golang.org/x/net v0.0.0-20190503192946-f4e77d36d62c // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
)
