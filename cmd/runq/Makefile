include ../../make.rules

RUNC_COMMIT := 9f9c96235cc97674e935002fc3d78361b696a69e
RUNC_PATH := $(GOPATH)/src/github.com/opencontainers/runc
EXTRA_LDFLAGS := "-X main.runqCommit=$(GIT_COMMIT)"

runq: $(RUNC_PATH)/.git $(shell find . -type f ! -name runq)
	@echo check GOPATH; test -n "$(GOPATH)"
	cd $(RUNC_PATH) && git reset --hard -q
	cd $(RUNC_PATH) && git clean -xfdq
	cd $(RUNC_PATH) && git reset --hard -q $(RUNC_COMMIT) || { git pull; git reset --hard -q $(RUNC_COMMIT); }
	cd $(RUNC_PATH) && git apply $(CURDIR)/runc-to-runq.patch
	cp $(CURDIR)/runq.go $(RUNC_PATH)/
	CC=gcc EXTRA_LDFLAGS=$(EXTRA_LDFLAGS) $(MAKE) -C $(RUNC_PATH) BUILDTAGS="" runc
	cp -f $(RUNC_PATH)/runc $(CURDIR)/runq

$(RUNC_PATH)/.git:
	@echo check GOPATH; test -n "$(GOPATH)"
	@echo Error: RunC source not found
	@echo To download RunC source: '"go get -d github.com/opencontainers/runc"'; false

install: runq
	install -m 0755 -D $(CURDIR)/runq $(RUNQ_ROOT)/runq

clean:
	$(MAKE) -C $(RUNC_PATH) clean
	rm -f runq
