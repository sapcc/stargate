DATE    = $(shell date +%Y%m%d%H%M)
IMAGE   ?= sapcc/stargate
VERSION = v$(DATE)
GOOS    ?= $(shell go env | grep GOOS | cut -d'"' -f2)
BINARY  := stargate

LDFLAGS := -X github.com/sapcc/stargate/pkg/stargate.VERSION=$(VERSION)
GOFLAGS := -ldflags "$(LDFLAGS)"

SRCDIRS  := cmd pkg
PACKAGES := $(shell find $(SRCDIRS) -type d)
GOFILES  := $(addsuffix /*.go,$(PACKAGES))
GOFILES  := $(wildcard $(GOFILES))

GLIDE := $(shell command -v glide 2> /dev/null)

.PHONY: all clean vendor tests static-check

all: bin/$(GOOS)/$(BINARY)

bin/%/$(BINARY): $(GOFILES) Makefile
	GOOS=$* GOARCH=amd64 go build $(GOFLAGS) -v -i -o bin/$*/$(BINARY) ./cmd

build: tests bin/linux/$(BINARY)
	docker build -t $(IMAGE):$(VERSION) .

static-check:
	@if s="$$(gofmt -s -l *.go pkg 2>/dev/null)"                            && test -n "$$s"; then printf ' => %s\n%s\n' gofmt  "$$s"; false; fi
	@if s="$$(golint . && find pkg -type d -exec golint {} \; 2>/dev/null)" && test -n "$$s"; then printf ' => %s\n%s\n' golint "$$s"; false; fi

tests: all static-check
	go test -v github.com/sapcc/stargate/pkg/...

push: build
	docker push $(IMAGE):$(VERSION)

latest: push
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest
	docker push $(IMAGE):latest

clean:
	rm -rf bin/*

vendor:
	dep ensure
