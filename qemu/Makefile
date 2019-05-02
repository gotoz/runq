include ../make.rules

all: image qemu.tar

image:
	cd $(ARCH) && \
	docker build -t runq-build .

qemu.tar:
	docker run --rm runq-build tar -C $(QEMU_ROOT) -cf - .  > qemu.tar

install: qemu.tar
	rm -rf $(QEMU_ROOT)
	mkdir -p $(QEMU_ROOT)
	tar -C $(QEMU_ROOT) -xf qemu.tar
	chmod 0700 $(QEMU_ROOT)
	chmod 0750 $(RUNQ_ROOT)

clean:
	-rm -f qemu.tar

distclean: clean
	docker rmi -f runq-build; echo
