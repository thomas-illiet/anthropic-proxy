SHELL := /bin/bash

BINARY := anthropic-proxy
CMD := .
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/thomas-illiet/anthropic-proxy/internal/cli.version=$(VERSION)
DIST_DIR := dist/release
PACKAGE_DIR := dist/package
COVERAGE_THRESHOLD ?= 75.0
STATICCHECK := honnef.co/go/tools/cmd/staticcheck@v0.7.0
GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.3.0

.PHONY: check
check: repo-hygiene mod-verify fmt-check vet test race coverage staticcheck vulncheck

.PHONY: repo-hygiene
repo-hygiene:
	bash scripts/check-repo-hygiene.sh

.PHONY: mod-verify
mod-verify:
	go mod verify

.PHONY: build
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) $(CMD)

.PHONY: run
run: build
	./$(BINARY) serve

.PHONY: test
test:
	go test ./...

.PHONY: race
race:
	go test -race ./internal/...

.PHONY: coverage
coverage:
	bash scripts/check-coverage.sh "$(COVERAGE_THRESHOLD)"

.PHONY: staticcheck
staticcheck:
	go run $(STATICCHECK) ./...

.PHONY: vulncheck
vulncheck:
	go run $(GOVULNCHECK) ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	gofmt -s -w .

.PHONY: fmt-check
fmt-check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

.PHONY: dist
dist:
	bash scripts/build-dist.sh "$(VERSION)" "$(LDFLAGS)" "$(CMD)" "$(BINARY)"

.PHONY: docker-check
docker-check:
	docker compose config --quiet
	docker build -t anthropic-proxy:ci .

.PHONY: release-check
release-check: check docker-check dist
	bash scripts/verify-dist.sh "$(VERSION)" "$(DIST_DIR)" "$(BINARY)"

.PHONY: clean
clean:
	rm -rf $(BINARY) dist coverage coverage.out
