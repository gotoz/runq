include ../../make.rules

RUNC_REV := v1.0.0-rc92
EXTRA_LDFLAGS := "-X main.runqCommit=$(GIT_COMMIT)"

all: $(RUNC_ROOT)/.git
	git -C $(RUNC_ROOT) clean -fd
	git -C $(RUNC_ROOT) checkout -q -f $(RUNC_REV)
	git -C $(RUNC_ROOT) apply $(CURDIR)/runc.patch
	CC=gcc EXTRA_LDFLAGS=$(EXTRA_LDFLAGS) $(MAKE) -C $(RUNC_ROOT) BUILDTAGS="" runc
	cp -f $(RUNC_ROOT)/runc $(CURDIR)/runq

$(RUNC_ROOT)/.git:
	@echo Error: RunC source not found
	@echo To download RunC source: '"git clone https://github.com/opencontainers/runc"'; false

install: runq
	install -m 0755 -D $(CURDIR)/runq $(RUNQ_ROOT)/runq

clean:
	cd $(RUNC_ROOT) && git clean -xfd && git reset --hard && git checkout master
	rm -f runq
