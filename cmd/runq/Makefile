include ../../make.rules

GO_PATH := $(shell go env GOPATH | cut -d: -f1)
RUNC_REV := v1.0.0-rc8
RUNC_PATH := $(GO_PATH)/src/github.com/opencontainers/runc
EXTRA_LDFLAGS := "-X main.runqCommit=$(GIT_COMMIT)"

runq: $(RUNC_PATH)/.git $(shell find . -type f ! -name runq)
	git -C $(RUNC_PATH) clean -fd
	git -C $(RUNC_PATH) checkout -q -f $(RUNC_REV)
	git -C $(RUNC_PATH) apply $(CURDIR)/runc.patch
	CC=gcc EXTRA_LDFLAGS=$(EXTRA_LDFLAGS) $(MAKE) -C $(RUNC_PATH) BUILDTAGS="" runc
	cp -f $(RUNC_PATH)/runc $(CURDIR)/runq

$(RUNC_PATH)/.git:
	@echo Error: RunC source not found
	@echo To download RunC source: '"go get -d github.com/opencontainers/runc"'; false

install: runq
	install -m 0755 -D $(CURDIR)/runq $(RUNQ_ROOT)/runq

clean:
	cd $(RUNC_PATH) && git clean -xfd && git reset --hard && git checkout master
	rm -f runq
