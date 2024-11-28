# Default OS/ARCH values
OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

.PHONY: tidy build clean lint
.DEFAULT_GOAL := build

tidy:
	go mod tidy

build-cmd: tidy
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o ./build/jex_$(OS)_$(ARCH) -ldflags "-w"

build: build-cmd

clean:
	go clean
	rm -rf ./build

lint:
	golangci-lint run
