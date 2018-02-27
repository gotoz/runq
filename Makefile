include make.rules

SUBDIRS := cmd/proxy cmd/init cmd/runq
.PHONY: all $(SUBDIRS) install test clean

all: $(SUBDIRS)

$(SUBDIRS):
	$(MAKE) -C $@

install: $(SUBDIRS)
	$(MAKE) -C cmd/proxy install
	$(MAKE) -C cmd/runq install
	$(MAKE) -C initrd install

test:
	$(MAKE) -C test

clean:
	$(foreach d,$(SUBDIRS) initrd,$(MAKE) -C $(d) clean;)
