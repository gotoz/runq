include ../../make.rules

LD_FLAGS := -ldflags="-X main.gitCommit=$(GIT_COMMIT)"

runq-exec: $(shell find . -name '*.go')
	$(GO) build $(LD_FLAGS)

install: runq-exec
	install -m 0755 -D $(CURDIR)/runq-exec $(RUNQ_ROOT)/runq-exec
	install -m 0755 -D $(CURDIR)/mkcerts.sh $(QEMU_ROOT)/mkcerts.sh

clean:
	rm -f runq-exec
