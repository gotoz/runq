include ../../make.rules

EXTRA_LDFLAGS := "-X main.runqCommit=$(GIT_COMMIT)"

all: $(RUNC_ROOT)/.git clean
	git -C $(RUNC_ROOT) apply $(CURDIR)/runc.patch
	CC=gcc EXTRA_LDFLAGS=$(EXTRA_LDFLAGS) $(MAKE) -C $(RUNC_ROOT) BUILDTAGS="" runc
	cp -f $(RUNC_ROOT)/runc $(CURDIR)/runq

$(RUNC_ROOT)/.git:
	@echo Error: RunC source not found
	@echo "Run 'git submodule update --init'"; false

install: runq
	install -m 0755 -D $(CURDIR)/runq $(RUNQ_ROOT)/runq

clean:
	git -C $(RUNC_ROOT) clean -fd
	git -C $(RUNC_ROOT) reset --hard
	git submodule update --init
	rm -f runq
