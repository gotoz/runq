include ../make.rules

.PHONY: initrd

initrd: ../cmd/init/init
	QEMU_ROOT=$(QEMU_ROOT) $(CURDIR)/mkinitrd.sh

install: initrd
	install -m 0644 -D initrd $(QEMU_ROOT)/initrd

clean:
	rm -rf initrd tmp
