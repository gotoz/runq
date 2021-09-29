include ../../make.rules

LD_FLAGS := -ldflags="-s -w -extldflags -static -X main.gitCommit=$(GIT_COMMIT)"

init: $(shell find . -name '*.go')
	CGO_ENABLED=1 $(GO) build -tags netgo,osusergo $(LD_FLAGS)

install:

clean:
	rm -f init
