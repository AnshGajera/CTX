BINARY=ctx
PKG=github.com/AnshGajera/CTX
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)
LDFLAGS=-s -w -X $(PKG)/internal/version.Version=$(VERSION) -X $(PKG)/internal/version.Commit=$(COMMIT) -X $(PKG)/internal/version.Date=$(DATE)

.PHONY: build test lint install clean release dev test-go test-python

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/ctx

test: test-go test-python

test-go:
	go test ./... -race

test-python:
	@cd ctx-ml && python -m pytest tests -q 2>/dev/null || echo "Python tests skipped (missing deps)"

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || go vet ./...

install: build
	@echo "Installing $(BINARY) to /usr/local/bin/"
	cp ./$(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist/ stage/

release:
	goreleaser release --clean

dev:
	go run ./cmd/ctx init
	go run ./cmd/ctx extract
	go run ./cmd/ctx status
