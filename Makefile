# Proxy Rule Manager - Makefile
# =============================

# Configuration
REGISTRY ?= ghcr.io
REPO ?= fl0w1nd/proxy-rule-manager

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
CYAN := \033[0;36m
RED := \033[0;31m
NC := \033[0m # No Color

.PHONY: help
help: ## Show this help message
	@echo "$(CYAN)Proxy Rule Manager - Build Commands$(NC)"
	@echo ""
	@echo "$(YELLOW)Usage:$(NC)"
	@echo "  make <target>"
	@echo ""
	@echo "$(YELLOW)Targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-18s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(YELLOW)Examples:$(NC)"
	@echo "  make dev                # Run development server"
	@echo "  make build-dev          # Build local dev Docker image"
	@echo "  make push-local         # Build & push from local (will prompt for tag)"
	@echo "  make push-cloud         # Trigger GitHub Actions build (will prompt for tag)"

# ==================
# Development
# ==================

.PHONY: dev
dev: ## Run development server (frontend + backend)
	npm run start:dev

.PHONY: dev-frontend
dev-frontend: ## Run Next.js development server only
	npm run dev

.PHONY: dev-backend
dev-backend: ## Run backend development server only
	npm run dev:server

.PHONY: build
build: ## Build Next.js application
	npm run build

.PHONY: test
test: ## Run tests
	npm run test

.PHONY: lint
lint: ## Run linter
	npm run lint

# ==================
# Docker - Development
# ==================

.PHONY: build-dev
build-dev: ## Build Docker image for local development (no push)
	@echo "$(CYAN)Building development image...$(NC)"
	docker buildx bake dev

.PHONY: run-dev
run-dev: build-dev ## Build and run development container
	@echo "$(CYAN)Running development container...$(NC)"
	docker run --rm -it -p 3000:3000 -v $(PWD)/data:/app/data proxy-rule-manager:dev

# ==================
# Docker - Local Build & Push
# ==================

.PHONY: login
login: ## Login to GitHub Container Registry
	@echo "$(CYAN)Logging into $(REGISTRY)...$(NC)"
	@if command -v gh >/dev/null 2>&1; then \
		echo "Using GitHub CLI for authentication..."; \
		gh auth token | docker login $(REGISTRY) -u $$(gh api user -q .login) --password-stdin; \
	else \
		echo "Please enter your GitHub Personal Access Token:"; \
		docker login $(REGISTRY); \
	fi

.PHONY: push-local
push-local: ## Build multi-arch image locally and push to ghcr.io
	@echo "$(CYAN)========================================$(NC)"
	@echo "$(CYAN)  Local Docker Build & Push$(NC)"
	@echo "$(CYAN)========================================$(NC)"
	@echo ""
	@echo "$(YELLOW)Enter version tag:$(NC)"
	@echo "  - Press Enter     → dev build"
	@echo "  - v1.0.0          → release (updates latest)"
	@echo "  - other           → isolated tag (no latest)"
	@echo ""
	@read -p "> " input_tag; \
	if [ -z "$$input_tag" ]; then \
		echo ""; \
		echo "$(CYAN)Building DEV image...$(NC)"; \
		echo "  Tag: $(REGISTRY)/$(REPO):dev"; \
		echo ""; \
		docker buildx bake app --push \
			--set app.tags=$(REGISTRY)/$(REPO):dev; \
		echo ""; \
		echo "$(GREEN)✓ Dev build pushed!$(NC)"; \
		echo "  Image: $(REGISTRY)/$(REPO):dev"; \
	elif echo "$$input_tag" | grep -q "^v"; then \
		echo ""; \
		echo "$(CYAN)Building RELEASE image...$(NC)"; \
		echo "  Tags: $(REGISTRY)/$(REPO):$$input_tag"; \
		echo "        $(REGISTRY)/$(REPO):latest"; \
		echo ""; \
		docker buildx bake app --push \
			--set app.tags=$(REGISTRY)/$(REPO):$$input_tag \
			--set app.tags=$(REGISTRY)/$(REPO):latest; \
		echo ""; \
		echo "$(GREEN)✓ Release build pushed!$(NC)"; \
		echo "  Image: $(REGISTRY)/$(REPO):$$input_tag"; \
		echo "  Image: $(REGISTRY)/$(REPO):latest"; \
	else \
		echo ""; \
		echo "$(CYAN)Building ISOLATED image...$(NC)"; \
		echo "  Tag: $(REGISTRY)/$(REPO):$$input_tag"; \
		echo ""; \
		docker buildx bake app --push \
			--set app.tags=$(REGISTRY)/$(REPO):$$input_tag; \
		echo ""; \
		echo "$(GREEN)✓ Isolated build pushed!$(NC)"; \
		echo "  Image: $(REGISTRY)/$(REPO):$$input_tag"; \
	fi

# ==================
# Docker - Cloud Build (GitHub Actions)
# ==================

.PHONY: push-cloud
push-cloud: ## Trigger GitHub Actions to build and push
	@if ! command -v gh >/dev/null 2>&1; then \
		echo "$(RED)Error: GitHub CLI (gh) is required$(NC)"; \
		echo "Install: brew install gh"; \
		exit 1; \
	fi
	@echo "$(CYAN)========================================$(NC)"
	@echo "$(CYAN)  GitHub Actions Docker Build$(NC)"
	@echo "$(CYAN)========================================$(NC)"
	@echo ""
	@echo "$(YELLOW)Enter version tag:$(NC)"
	@echo "  - Press Enter     → dev build"
	@echo "  - v1.0.0          → release (updates latest)"
	@echo "  - other           → isolated tag (no latest)"
	@echo ""
	@read -p "> " input_tag; \
	if [ -z "$$input_tag" ]; then \
		echo ""; \
		echo "$(CYAN)Triggering DEV build...$(NC)"; \
		echo "  Tag: dev"; \
		gh workflow run docker-build.yml -f tag=dev -f tag_type=dev; \
	elif echo "$$input_tag" | grep -q "^v"; then \
		echo ""; \
		echo "$(CYAN)Triggering RELEASE build...$(NC)"; \
		echo "  Tags: $$input_tag, latest"; \
		gh workflow run docker-build.yml -f tag=$$input_tag -f tag_type=release; \
	else \
		echo ""; \
		echo "$(CYAN)Triggering ISOLATED build...$(NC)"; \
		echo "  Tag: $$input_tag"; \
		gh workflow run docker-build.yml -f tag=$$input_tag -f tag_type=isolated; \
	fi; \
	echo ""; \
	echo "$(GREEN)✓ Workflow triggered!$(NC)"; \
	echo "  Check status: make push-status"; \
	echo "  Watch logs:   make push-logs"

.PHONY: push-status
push-status: ## Check GitHub Actions build status
	@echo "$(CYAN)Recent workflow runs:$(NC)"
	gh run list --workflow=docker-build.yml --limit=5

.PHONY: push-logs
push-logs: ## Watch the latest GitHub Actions build logs
	gh run watch

# ==================
# Docker Compose
# ==================

.PHONY: up
up: ## Start services with docker-compose
	docker compose up -d

.PHONY: down
down: ## Stop services
	docker compose down

.PHONY: logs
logs: ## View container logs
	docker compose logs -f

.PHONY: restart
restart: down up ## Restart services

# ==================
# Utilities
# ==================

.PHONY: clean
clean: ## Clean build artifacts and caches
	@echo "$(CYAN)Cleaning build artifacts...$(NC)"
	rm -rf .next out dist node_modules/.cache
	docker buildx prune -f

.PHONY: info
info: ## Show current configuration
	@echo "$(CYAN)Current Configuration:$(NC)"
	@echo "  Registry:   $(REGISTRY)"
	@echo "  Repository: $(REPO)"
	@echo ""
	@echo "$(CYAN)Docker Buildx Info:$(NC)"
	@docker buildx ls

# Default target
.DEFAULT_GOAL := help
