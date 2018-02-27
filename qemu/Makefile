include ../make.rules

all: tar install

tar:
	cd $(ARCH) && \
	docker build -t qemu-runq \
	  --build-arg https_proxy="$(https_proxy)" \
	  --build-arg http_proxy="$(http_proxy)" .
	docker run --rm qemu-runq cat /qemu.tar > qemu.tar

install:
	rm -rf $(QEMU_ROOT)
	mkdir -p $(QEMU_ROOT)
	tar -C $(QEMU_ROOT) -xf qemu.tar

clean:
	rm -f qemu.tar

dist-clean:
	-docker rmi qemu-runq
