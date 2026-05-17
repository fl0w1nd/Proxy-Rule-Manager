# Proxy Rule Manager - Makefile

REGISTRY ?= ghcr.io
REPO ?= fl0w1nd/proxy-rule-manager
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

# ── Development ────────────────────────────────────────────────

.PHONY: dev
dev: ## Start frontend + backend dev servers
	pnpm run dev

.PHONY: dev-fe
dev-fe: ## Start Next.js dev server only (port 3000)
	pnpm run dev:fe

.PHONY: dev-be
dev-be: ## Start Go backend dev server only (port 3001)
	pnpm run dev:be

# ── Build ──────────────────────────────────────────────────────

.PHONY: build
build: build-fe build-be ## Build frontend + backend

.PHONY: build-fe
build-fe: ## Build Next.js static export to ./out
	pnpm run build

.PHONY: build-be
build-be: ## Build Go binary to ./bin/proxy-rule-manager
	cd backend && go build -trimpath -ldflags='$(GO_LDFLAGS)' -o ../bin/proxy-rule-manager ./cmd/server

# ── Quality ─────────────────────────────────────────────────────

.PHONY: check
check: lint typecheck test ## Run all CI checks (lint + typecheck + test)

.PHONY: lint
lint: lint-fe lint-be ## Lint frontend (ESLint) + backend (gofmt/vet/staticcheck)
	@echo "$(GREEN)lint OK$(NC)"

.PHONY: lint-fe
lint-fe: ## Run frontend ESLint
	pnpm run lint

.PHONY: lint-be
lint-be: ## Run gofmt + go vet + staticcheck
	@cd backend && out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "$(YELLOW)gofmt found unformatted files:$(NC)"; echo "$$out"; exit 1; fi
	cd backend && go vet ./...
	@command -v staticcheck >/dev/null 2>&1 || (echo "$(YELLOW)installing staticcheck...$(NC)" && go install honnef.co/go/tools/cmd/staticcheck@latest)
	cd backend && PATH="$$(go env GOPATH)/bin:$$PATH" staticcheck ./...

.PHONY: typecheck
typecheck: ## Run TypeScript type checking
	pnpm run typecheck

.PHONY: test
test: ## Run Go tests
	cd backend && go test -race ./...

# ── Docker ──────────────────────────────────────────────────────

.PHONY: docker-build
docker-build: ## Build local Docker image (current platform only)
	docker buildx bake dev

.PHONY: docker-run
docker-run: docker-build ## Build and run local Docker container (port 3000, data in ./data)
	docker run --rm -it -p 3000:3000 -v $(PWD)/data:/app/data proxy-rule-manager:dev

.PHONY: up
up: ## Start services (docker compose up -d)
	docker compose up -d

.PHONY: down
down: ## Stop services (docker compose down)
	docker compose down

.PHONY: logs
logs: ## Tail docker compose logs
	docker compose logs -f

# ── Misc ────────────────────────────────────────────────────────

.PHONY: version
version: ## Show version and commit hash
	@echo "$(CYAN)Version:$(NC) v$(VERSION)  $(CYAN)Commit:$(NC) $(GIT_HASH)"

.PHONY: clean
clean: ## Remove build artifacts and Docker build cache
	@echo "$(CYAN)Cleaning...$(NC)"
	rm -rf .next out dist bin
	docker buildx prune -f

.PHONY: repomap
repomap: ## Generate repo map for AI agents
	aider --show-repo-map --exit --no-gitignore --yes-always 1>.agents/repomap.md 2>/dev/null

.DEFAULT_GOAL := help
