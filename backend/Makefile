.PHONY: build dev test lint vet fmt

# Version is normally injected from the repo-root Makefile / CI workflows
# (which read it from ../package.json). When running `make -C backend build`
# directly, fall back to the same lookup so the resulting binary still
# advertises a meaningful version through /api/status.
VERSION ?= $(shell node -p "require('../package.json').version" 2>/dev/null || echo "dev")
GO_LDFLAGS ?= -s -w -X github.com/fl0w1nd/proxy-rule-manager/backend/internal/api.Version=$(VERSION)

build:
	go build -trimpath -ldflags='$(GO_LDFLAGS)' -o ../bin/server ./cmd/server

dev:
	go run ./cmd/server

test:
	go test ./...

lint:
	go vet ./...

vet: lint

fmt:
	gofmt -s -w .
