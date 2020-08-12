include make.rules

SUBDIRS := cmd/proxy cmd/init cmd/runq cmd/runq-exec cmd/nsenter cmd/vsockd
TARDIR := runq-$(GIT_COMMIT)
TARFILE := $(TARDIR).tar.gz

.PHONY: all $(SUBDIRS) install image test tarfile release release-install clean distclean version

all: $(SUBDIRS)

$(SUBDIRS):
	$(MAKE) -C $@

install: $(SUBDIRS) $(QEMU_ROOT) version
	$(MAKE) -C cmd/proxy install
	$(MAKE) -C cmd/runq install
	$(MAKE) -C cmd/runq-exec install
	$(MAKE) -C initrd install
	install -m 0444 -D version $(RUNQ_ROOT)/version

version:
	echo "Git Commit: $(GIT_COMMIT)" > $@
	chown `stat --printf "%u:%g" .git` $@

image:
	$(MAKE) -C qemu image

test:
	$(MAKE) -C test

tarfile:
	tar  -C $(RUNQ_ROOT) --transform 's,^./,$(TARDIR)/,' --numeric-owner --owner=0 --group=0 -czf $(TARFILE) .
	chown `stat --printf "%u:%g" .git` $(TARFILE)

release: image
	docker run \
		--rm \
		-v $(CURDIR):/runq \
		-v /usr/bin/docker-init:/usr/bin/docker-init:ro \
		$(BUILD_IMAGE) make clean install tarfile clean2

release-install: $(TARFILE)
	mkdir -p $(RUNQ_ROOT)
	tar -C $(RUNQ_ROOT) --strip-components 1 -xzf $(TARFILE)

clean clean2:
	$(foreach d,$(SUBDIRS) qemu initrd,$(MAKE) -C $(d) clean;)
	git -C $(RUNC_ROOT) clean -xfd
	git -C $(RUNC_ROOT) reset --hard
	rm -f version

distclean: clean
	$(MAKE) -C qemu distclean
	rm -f runq-*.tar.gz

