include ../../make.rules

LD_FLAGS := -ldflags="-X main.gitCommit=$(GIT_COMMIT)"

.PHONY: runq-exec

runq-exec: $(shell find . -name '*.go')
	@echo check GOPATH; test -n "$(GOPATH)"
	$(GO) build $(LD_FLAGS)

install: runq-exec
	install -m 0755 -D $(CURDIR)/runq-exec $(RUNQ_ROOT)/runq-exec

clean:
	rm -f runq-exec
