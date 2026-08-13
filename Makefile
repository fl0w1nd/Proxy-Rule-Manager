.PHONY: build test test-race lint ci run serve update validate clean distclean help proto docker-build docker-run

BINARY := bin/prm
VERSION_FILE := version/VERSION
VERSION ?= $(shell tr -d '[:space:]' < $(VERSION_FILE))
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
BUILD_DATE ?= $(shell git show -s --format=%cI HEAD 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PACKAGE := github.com/fl0w1nd/proxy-rule-manager/version
LDFLAGS := -s -w -X $(VERSION_PACKAGE).Version=$(VERSION) -X $(VERSION_PACKAGE).Commit=$(COMMIT) -X $(VERSION_PACKAGE).Date=$(BUILD_DATE)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

proto: ## Regenerate protobuf Go code from .proto (requires protoc + protoc-gen-go)
	protoc --go_out=. --go_opt=paths=source_relative internal/geosite/geosite.proto

build: ## Build the prm binary
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/prm

test: ## Run all tests
	go test -shuffle=on -coverprofile=coverage.out ./...

test-race: ## Run all tests with the race detector
	go test -race -shuffle=on ./...

lint: ## Run formatting, vet, and golangci-lint checks
	@echo "==> gofmt"
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	@echo "==> go vet"
	go vet ./...
	@echo "==> golangci-lint"
	golangci-lint run ./...

ci: lint test-race ## Run the local CI quality gate

run: build ## Build and run update
	./$(BINARY) update

serve: build ## Build and run serve
	./$(BINARY) serve

update: build ## Build and run update
	./$(BINARY) update

validate: build ## Build and validate config
	./$(BINARY) validate

clean: ## Remove build artifacts
	rm -rf bin/

distclean: clean ## Remove build artifacts AND all generated data (state, caches, artifacts)
	rm -rf data/

docker-build: ## Build Docker image
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t fl0w1nd/prm:$(VERSION) .

docker-run: docker-build ## Build and run Docker container
	docker run --rm -e PRM_ADMIN_TOKEN -v "$$(pwd)/data:/data" -p 3001:3001 fl0w1nd/prm:$(VERSION)
