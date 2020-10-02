include ../make.rules

.PHONY: all install integration examples

all: integration examples

integration:
	docker run --runtime runq alpine env
	./run.sh ./integration/*.sh

examples:
	./run.sh ./examples/*.sh

