include make.rules

SUBDIRS := cmd/proxy cmd/init cmd/runq cmd/runq-exec cmd/nsenter cmd/vsockd
TAR := runq-$(GIT_COMMIT).tar.gz

.PHONY: all $(SUBDIRS) install image test tarfile release release-install clean distclean

all: $(SUBDIRS)

$(SUBDIRS):
	$(MAKE) -C $@

install: $(SUBDIRS) $(QEMU_ROOT)
	$(MAKE) -C cmd/proxy install
	$(MAKE) -C cmd/runq install
	$(MAKE) -C cmd/runq-exec install
	$(MAKE) -C initrd install

image:
	$(MAKE) -C qemu image

test:
	$(MAKE) -C test

tarfile:
	tar -C / --numeric-owner --owner=0 --group=0 -czf $(TAR) $(RUNQ_ROOT)

release: image
	docker run --rm -v $(CURDIR):/go/src/github.com/gotoz/runq -v /usr/bin/docker-init:/usr/bin/docker-init:ro runq-build make clean install tarfile clean2

release-install: $(TAR)
	tar -C / -xzf $(TAR)

clean clean2:
	$(foreach d,$(SUBDIRS) qemu initrd,$(MAKE) -C $(d) clean;)

distclean: clean
	$(MAKE) -C qemu distclean
	rm -f runq-*.tar.gz

