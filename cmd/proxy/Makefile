include ../../make.rules

LD_FLAGS := -ldflags="-s -w -X main.gitCommit=$(GIT_COMMIT)"

proxy: $(shell find . -name '*.go')
	CGO_ENABLED=0 $(GO) build -tags netgo $(LD_FLAGS)

install: proxy
	install -m 0755 -D proxy $(QEMU_ROOT)/proxy

clean:
	rm -f proxy
