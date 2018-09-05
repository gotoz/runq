include ../../make.rules

LD_FLAGS := -ldflags="-extldflags -static -X main.gitCommit=$(GIT_COMMIT)"

init: $(shell find . -name '*.go')
	CGO_ENABLED=1 $(GO) build -tags "netgo" $(LD_FLAGS)

install:

clean:
	rm -f init
