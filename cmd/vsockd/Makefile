include ../../make.rules

LD_FLAGS := -ldflags="-s -w -X main.gitCommit=$(GIT_COMMIT)"

init: $(shell find . -name '*.go')
	CGO_ENABLED=1 $(GO) build -tags osusergo,netgo $(LD_FLAGS)

install:

clean:
	rm -f vsockd
