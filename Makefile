.PHONY: build build-web test test-web test-race lint ci run serve update validate clean distclean help proto docker-build docker-run dev

BINARY := bin/prm
VERSION_FILE := version/VERSION
VERSION ?= $(shell tr -d '[:space:]' < $(VERSION_FILE))
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
BUILD_DATE ?= $(shell git show -s --format=%cI HEAD 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PACKAGE := github.com/fl0w1nd/proxy-rule-manager/version
LDFLAGS := -s -w -X $(VERSION_PACKAGE).Version=$(VERSION) -X $(VERSION_PACKAGE).Commit=$(COMMIT) -X $(VERSION_PACKAGE).Date=$(BUILD_DATE)
CONFIG ?= config.dev.yaml
DATA_DIR ?= ./data
AIR := go run github.com/air-verse/air@v1.67.4

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

proto: ## Regenerate protobuf Go code from .proto (requires protoc + protoc-gen-go)
	protoc --go_out=. --go_opt=paths=source_relative internal/geosite/geosite.proto internal/geoip/geoip.proto

build-web: ## Build Svelte 5 admin and public web assets
	cd web && pnpm build

build: build-web ## Build the prm binary with embedded web assets
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/prm

test: ## Run all tests
	go test -shuffle=on -coverprofile=coverage.out ./...

test-web: ## Check and test Svelte web applications
	cd web && pnpm check && pnpm test --run

test-race: ## Run all tests with the race detector
	go test -race -shuffle=on ./...

lint: ## Run formatting, vet, and golangci-lint checks
	@echo "==> gofmt"
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	@echo "==> go vet"
	go vet ./...
	@echo "==> golangci-lint"
	golangci-lint run ./...

ci: lint test-race test-web ## Run the local CI quality gate

run: build ## Build and run update
	./$(BINARY) update

serve: build ## Build and run serve
	./$(BINARY) serve

dev: ## Live-reload Go serve and the admin Vite app
	@if [ ! -f "$(CONFIG)" ]; then \
		if [ "$(CONFIG)" = "config.dev.yaml" ]; then \
			cp config.template.yaml config.dev.yaml; \
			echo "created config.dev.yaml from config.template.yaml"; \
		else \
			echo "error: config file not found: $(CONFIG)" >&2; \
			exit 1; \
		fi; \
	fi
	@if [ ! -d web/node_modules ]; then (cd web && pnpm install); fi
	@if [ ! -f internal/admin/dist/index.html ] || [ ! -f internal/site/dist/public.js ]; then \
		$(MAKE) --no-print-directory build-web; \
	fi
	@echo "==> admin  http://127.0.0.1:5173/admin/"
	@echo "==> server http://127.0.0.1:3001/"
	@set -e; trap 'kill 0' EXIT INT TERM; \
		PRM_DEV=1 $(AIR) -c .air.toml -- serve -c "$(CONFIG)" --data-dir "$(DATA_DIR)" & \
		(cd web && pnpm dev) & \
		wait

update: build ## Build and run update
	./$(BINARY) update

validate: build ## Build and validate config
	./$(BINARY) validate

clean: ## Remove build artifacts
	rm -rf bin/ tmp/

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
