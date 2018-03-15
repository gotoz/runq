include ../make.rules

.PHONY: all install integration examples

all: integration examples

install:
	$(MAKE) -C testdata install

integration:
	docker run --runtime runq busybox env
	./run.sh ./integration/*.sh

examples:
	./run.sh ./examples/*.sh

