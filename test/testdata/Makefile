include ../../make.rules

all:

install:
	mkdir -p $(QEMU_ROOT)/.runq
	cp SIGUSR1 SIGUSR2 $(QEMU_ROOT)/.runq/

clean:
	rm -rf $(QEMU_ROOT)/.runq
