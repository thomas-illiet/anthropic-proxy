SHELL := /bin/bash

BINARY := anthropic-proxy
CMD := .
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/thomas-illiet/anthropic-proxy/internal/cli.version=$(VERSION)
DIST_DIR := dist/release
PACKAGE_DIR := dist/package

.PHONY: build
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) $(CMD)

.PHONY: run
run: build
	./$(BINARY) serve

.PHONY: test
test:
	go test ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	gofmt -s -w .

.PHONY: fmt-check
fmt-check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

.PHONY: clean
clean:
	rm -rf $(BINARY) dist
