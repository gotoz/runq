include ../make.rules

.PHONY: initrd

initrd: ../cmd/init/init /sbin/docker-init
	QEMU_ROOT=$(QEMU_ROOT) $(CURDIR)/mkinitrd.sh

install: initrd
	install -m 0644 -D initrd $(QEMU_ROOT)/initrd

/sbin/docker-init:
	@echo "$@ is required for 'docker run --init ...'"
	@echo "run 'touch $@' to ignore"; false

clean:
	rm -rf initrd

