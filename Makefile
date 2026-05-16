# Proxy Rule Manager - Makefile

REGISTRY ?= ghcr.io
REPO ?= fl0w1nd/proxy-rule-manager
# Single source of truth: package.json. Falls back to "dev" if node is missing.
VERSION := $(shell node -p "require('./package.json').version" 2>/dev/null || echo "dev")
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO_LDFLAGS := -s -w -X github.com/fl0w1nd/proxy-rule-manager/backend/internal/api.Version=$(VERSION)

GREEN := \033[0;32m
YELLOW := \033[0;33m
CYAN := \033[0;36m
NC := \033[0m

.PHONY: help
help: ## Show this help message
	@echo "$(CYAN)Proxy Rule Manager - Commands$(NC)"
	@echo ""
	@echo "$(YELLOW)Usage:$(NC)"
	@echo "  make <target>"
	@echo ""
	@echo "$(YELLOW)Targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-18s$(NC) %s\n", $$1, $$2}'

.PHONY: dev
dev: ## Run frontend and backend in development mode
	pnpm run dev

.PHONY: dev-fe
dev-fe: ## Run Next.js development server only
	pnpm run dev:fe

.PHONY: dev-be
dev-be: ## Run backend development server only
	pnpm run dev:be

.PHONY: build
build: build-fe build-be ## Build frontend (next) and backend (go) binaries

.PHONY: build-fe
build-fe: ## Build the Next.js frontend (static export to ./out)
	pnpm run build

.PHONY: build-be
build-be: ## Build the Go backend binary into ./bin/proxy-rule-manager
	cd backend && go build -trimpath -ldflags='$(GO_LDFLAGS)' -o ../bin/proxy-rule-manager ./cmd/server

.PHONY: test
test: ## Run Go tests
	cd backend && go test ./...

.PHONY: lint
lint: lint-fe lint-be ## Run frontend lint + backend lint
	@echo "$(GREEN)lint OK$(NC)"

.PHONY: lint-fe
lint-fe: ## Run frontend ESLint
	pnpm run lint

.PHONY: lint-be
lint-be: ## Run backend gofmt + go vet + staticcheck
	@cd backend && out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "$(YELLOW)gofmt found unformatted files:$(NC)"; echo "$$out"; echo "$(YELLOW)run 'gofmt -w backend'$(NC)"; exit 1; fi
	cd backend && go vet ./...
	@command -v staticcheck >/dev/null 2>&1 || (echo "$(YELLOW)installing staticcheck...$(NC)" && go install honnef.co/go/tools/cmd/staticcheck@latest)
	cd backend && PATH="$$(go env GOPATH)/bin:$$PATH" staticcheck ./...

.PHONY: typecheck
typecheck: ## Run TypeScript type checking
	pnpm run typecheck

.PHONY: build-dev
build-dev: ## Build the local development image
	@echo "$(CYAN)Building development image...$(NC)"
	docker buildx bake dev

.PHONY: run-dev
run-dev: build-dev ## Build and run the development container
	@echo "$(CYAN)Running development container...$(NC)"
	docker run --rm -it -p 3000:3000 -v $(PWD)/data:/app/data proxy-rule-manager:dev

.PHONY: version
version: ## Show the current application version
	@echo "$(CYAN)Current version:$(NC) v$(VERSION)"
	@echo "$(CYAN)Current commit:$(NC)  $(GIT_HASH)"

.PHONY: up
up: ## Start services with docker compose
	docker compose up -d

.PHONY: down
down: ## Stop services with docker compose
	docker compose down

.PHONY: logs
logs: ## View docker compose logs
	docker compose logs -f

.PHONY: restart
restart: down up ## Restart docker compose services

.PHONY: repomap
repomap: ## Generate repository map using aider
	aider --show-repo-map --exit --no-gitignore --yes-always 1>.agents/repomap.md 2>/dev/null

.PHONY: clean
clean: ## Clean build artifacts and local build cache
	@echo "$(CYAN)Cleaning build artifacts...$(NC)"
	rm -rf .next out dist node_modules/.cache
	docker buildx prune -f

.PHONY: info
info: ## Show build-related configuration
	@echo "$(CYAN)Current Configuration:$(NC)"
	@echo "  Registry:   $(REGISTRY)"
	@echo "  Repository: $(REPO)"
	@echo "  Version:    v$(VERSION)"
	@echo "  Git Hash:   $(GIT_HASH)"

.DEFAULT_GOAL := help
