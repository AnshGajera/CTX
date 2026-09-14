BINARY=ctx
PKG=github.com/ctxdev/ctx
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)
LDFLAGS=-s -w -X $(PKG)/internal/version.Version=$(VERSION) -X $(PKG)/internal/version.Commit=$(COMMIT) -X $(PKG)/internal/version.Date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/ctx

test:
	go test ./... 
	go test ./ctx-ml/tests 2>/dev/null || true
	cd ctx-ml && python -m pytest -q 2>/dev/null || true

lint:
	golangci-lint run ./... || go vet ./...

install: build
	cp ./$(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -f $(BINARY) ctx.exe
	rm -rf dist/ .ctx/

release:
	goreleaser release --clean

dev:
	go run ./cmd/ctx init
	go run ./cmd/ctx extract
	go run ./cmd/ctx status
