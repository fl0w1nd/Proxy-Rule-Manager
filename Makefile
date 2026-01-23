# Proxy Rule Manager - Makefile
# =============================

# Configuration
REGISTRY ?= ghcr.io
REPO ?= fl0w1nd/proxy-rule-manager
GITHUB_USER ?= fl0w1nd
# GITHUB_TOKEN should be set in your shell config (e.g., ~/.zshrc)

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
	@echo "  make release            # Create GitHub Release + optional Docker build"
	@echo "  make push-local         # Build & push Docker image locally"
	@echo "  make push-cloud         # Trigger GitHub Actions Docker build"

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
	@if [ -z "$(GITHUB_TOKEN)" ]; then \
		echo "$(RED)Error: GITHUB_TOKEN environment variable is not set$(NC)"; \
		echo ""; \
		echo "Add to your ~/.zshrc:"; \
		echo "  export GITHUB_TOKEN=\"ghp_your_token_here\""; \
		echo ""; \
		echo "Then run: source ~/.zshrc"; \
		exit 1; \
	fi
	@echo "$(GITHUB_TOKEN)" | docker login $(REGISTRY) -u $(GITHUB_USER) --password-stdin

# Check if logged in to ghcr.io, auto-login if GITHUB_TOKEN is set
define check_login
	@if ! docker login $(REGISTRY) --get-login >/dev/null 2>&1; then \
		if [ -z "$(GITHUB_TOKEN)" ]; then \
			echo "$(RED)Error: Not logged in and GITHUB_TOKEN is not set$(NC)"; \
			echo ""; \
			echo "Add to your ~/.zshrc:"; \
			echo "  export GITHUB_TOKEN=\"ghp_your_token_here\""; \
			echo ""; \
			exit 1; \
		fi; \
		echo "$(YELLOW)Logging into $(REGISTRY)...$(NC)"; \
		echo "$(GITHUB_TOKEN)" | docker login $(REGISTRY) -u $(GITHUB_USER) --password-stdin; \
		echo ""; \
	fi
endef

# Get version from package.json
VERSION := $(shell node -p "require('./package.json').version")
GIT_HASH := $(shell git rev-parse --short HEAD)

# Check if a tag exists on ghcr.io (returns 0 if exists, 1 if not)
# Usage: $(call tag_exists,tag_name)
define tag_exists
$(shell if curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $(GITHUB_TOKEN)" \
	"https://ghcr.io/v2/$(REPO)/manifests/$(1)" 2>/dev/null | grep -q "200"; then echo "yes"; fi)
endef

.PHONY: push-local
push-local: ## Build multi-arch image locally and push to ghcr.io
	$(call check_login)
	@echo "$(CYAN)========================================$(NC)"
	@echo "$(CYAN)  Local Docker Build & Push$(NC)"
	@echo "$(CYAN)========================================$(NC)"
	@echo ""
	@echo "$(CYAN)Current version:$(NC) v$(VERSION)"
	@echo "$(CYAN)Git commit:$(NC)      $(GIT_HASH)"
	@echo ""
	@echo "$(YELLOW)Select build type:$(NC)"
	@echo "  - Press Enter  → Release v$(VERSION) + latest"
	@echo "  - dev          → Dev build (dev + $(GIT_HASH))"
	@echo "  - <other>      → Isolated tag (no overwrite)"
	@echo ""
	@read -p "> " input_tag; \
	if [ -z "$$input_tag" ]; then \
		echo ""; \
		echo "$(CYAN)Checking if v$(VERSION) already exists...$(NC)"; \
		if curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $(GITHUB_TOKEN)" \
			"https://ghcr.io/v2/$(REPO)/manifests/v$(VERSION)" 2>/dev/null | grep -q "200"; then \
			echo "$(RED)Error: v$(VERSION) already exists on ghcr.io$(NC)"; \
			echo ""; \
			echo "Please bump version in package.json first, then retry."; \
			echo "  npm version patch  # or minor, major"; \
			exit 1; \
		fi; \
		echo "$(GREEN)✓ Version available$(NC)"; \
		echo ""; \
		echo "$(CYAN)Building RELEASE image...$(NC)"; \
		echo "  Tags: $(REGISTRY)/$(REPO):v$(VERSION)"; \
		echo "        $(REGISTRY)/$(REPO):latest"; \
		echo ""; \
		docker buildx bake app --push \
			--set app.tags=$(REGISTRY)/$(REPO):v$(VERSION) \
			--set app.tags=$(REGISTRY)/$(REPO):latest; \
		echo ""; \
		echo "$(GREEN)✓ Release build pushed!$(NC)"; \
		echo "  Image: $(REGISTRY)/$(REPO):v$(VERSION)"; \
		echo "  Image: $(REGISTRY)/$(REPO):latest"; \
	elif [ "$$input_tag" = "dev" ]; then \
		echo ""; \
		echo "$(CYAN)Building DEV image...$(NC)"; \
		echo "  Tags: $(REGISTRY)/$(REPO):dev"; \
		echo "        $(REGISTRY)/$(REPO):$(GIT_HASH)"; \
		echo ""; \
		docker buildx bake app --push \
			--set app.tags=$(REGISTRY)/$(REPO):dev \
			--set app.tags=$(REGISTRY)/$(REPO):$(GIT_HASH); \
		echo ""; \
		echo "$(GREEN)✓ Dev build pushed!$(NC)"; \
		echo "  Image: $(REGISTRY)/$(REPO):dev"; \
		echo "  Image: $(REGISTRY)/$(REPO):$(GIT_HASH)"; \
	else \
		echo ""; \
		if echo "$$input_tag" | grep -qE "^v[0-9]|^latest$$"; then \
			echo "$(RED)Error: Cannot use version tags (v*) or 'latest' as isolated tag$(NC)"; \
			echo "These are reserved for release builds."; \
			exit 1; \
		fi; \
		echo "$(CYAN)Checking if $$input_tag already exists...$(NC)"; \
		if curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $(GITHUB_TOKEN)" \
			"https://ghcr.io/v2/$(REPO)/manifests/$$input_tag" 2>/dev/null | grep -q "200"; then \
			echo "$(RED)Error: Tag '$$input_tag' already exists on ghcr.io$(NC)"; \
			exit 1; \
		fi; \
		echo "$(GREEN)✓ Tag available$(NC)"; \
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
	@echo "$(CYAN)Current version:$(NC) v$(VERSION)"
	@echo "$(CYAN)Git commit:$(NC)      $(GIT_HASH)"
	@echo ""
	@echo "$(YELLOW)Select build type:$(NC)"
	@echo "  - Press Enter  → Release v$(VERSION) + latest"
	@echo "  - dev          → Dev build (dev + git hash)"
	@echo "  - <other>      → Isolated tag (no overwrite)"
	@echo ""
	@read -p "> " input_tag; \
	if [ -z "$$input_tag" ]; then \
		echo ""; \
		echo "$(CYAN)Triggering RELEASE build...$(NC)"; \
		echo "  Tags: v$(VERSION), latest"; \
		gh workflow run docker-build.yml -f tag=v$(VERSION) -f tag_type=release; \
	elif [ "$$input_tag" = "dev" ]; then \
		echo ""; \
		echo "$(CYAN)Triggering DEV build...$(NC)"; \
		echo "  Tags: dev, git-hash"; \
		gh workflow run docker-build.yml -f tag=dev -f tag_type=dev; \
	else \
		if echo "$$input_tag" | grep -qE "^v[0-9]|^latest$$"; then \
			echo "$(RED)Error: Cannot use version tags (v*) or 'latest' as isolated tag$(NC)"; \
			exit 1; \
		fi; \
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
# Release
# ==================

# Get the previous git tag
PREV_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "")

.PHONY: release
release: ## Create GitHub Release with auto-generated notes
	@if ! command -v gh >/dev/null 2>&1; then \
		echo "$(RED)Error: GitHub CLI (gh) is required$(NC)"; \
		echo "Install: brew install gh"; \
		exit 1; \
	fi
	@echo "$(CYAN)========================================$(NC)"
	@echo "$(CYAN)  Create GitHub Release$(NC)"
	@echo "$(CYAN)========================================$(NC)"
	@echo ""
	@echo "$(CYAN)Version:$(NC)      v$(VERSION)"
	@echo "$(CYAN)Previous:$(NC)     $(if $(PREV_TAG),$(PREV_TAG),(first release))"
	@echo "$(CYAN)Git commit:$(NC)   $(GIT_HASH)"
	@echo ""
	@# Check if tag already exists
	@if git tag -l "v$(VERSION)" | grep -q "v$(VERSION)"; then \
		echo "$(RED)Error: Tag v$(VERSION) already exists$(NC)"; \
		echo ""; \
		echo "Bump version first:"; \
		echo "  npm version patch  # 0.1.0 → 0.1.1"; \
		echo "  npm version minor  # 0.1.0 → 0.2.0"; \
		echo "  npm version major  # 0.1.0 → 1.0.0"; \
		exit 1; \
	fi
	@# Check for uncommitted changes
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "$(RED)Error: You have uncommitted changes$(NC)"; \
		echo "Please commit or stash them first."; \
		exit 1; \
	fi
	@# Show changelog only if there's a previous tag
	@if [ -n "$(PREV_TAG)" ]; then \
		echo "$(YELLOW)Changes since $(PREV_TAG):$(NC)"; \
		echo ""; \
		git log $(PREV_TAG)..HEAD --pretty=format:"  %C(yellow)%h%Creset %s %C(dim)(%cr)%Creset" | head -20; \
		echo ""; \
		echo ""; \
	else \
		echo "$(YELLOW)This is the first release.$(NC)"; \
		echo ""; \
	fi
	@printf "$(YELLOW)Create release v$(VERSION)? [y/N] $(NC)"; \
	read confirm; \
	if [ "$$confirm" != "y" ] && [ "$$confirm" != "Y" ]; then \
		echo "Cancelled."; \
		exit 0; \
	fi; \
	echo ""; \
	echo "$(CYAN)Creating release v$(VERSION)...$(NC)"; \
	if [ -n "$(PREV_TAG)" ]; then \
		gh release create "v$(VERSION)" --generate-notes --notes-start-tag "$(PREV_TAG)" --title "v$(VERSION)"; \
	else \
		gh release create "v$(VERSION)" --generate-notes --title "v$(VERSION)"; \
	fi; \
	echo ""; \
	echo "$(GREEN)✓ Release v$(VERSION) created!$(NC)"; \
	echo ""; \
	printf "$(YELLOW)Build and push Docker image? [y/N] $(NC)"; \
	read build_confirm; \
	if [ "$$build_confirm" = "y" ] || [ "$$build_confirm" = "Y" ]; then \
		echo ""; \
		echo "$(YELLOW)Build location:$(NC)"; \
		echo "  1) Local build"; \
		echo "  2) GitHub Actions"; \
		echo ""; \
		printf "> "; \
		read build_choice; \
		case "$$build_choice" in \
			1) \
				$(MAKE) _release-build-local ;; \
			2) \
				$(MAKE) _release-build-cloud ;; \
			*) \
				echo "Invalid choice. Skipping build."; \
				exit 0 ;; \
		esac; \
	fi

# Internal targets for release build (don't show in help)
.PHONY: _release-build-local
_release-build-local:
	$(call check_login)
	@echo ""
	@echo "$(CYAN)Building RELEASE image locally...$(NC)"
	@echo "  Tags: $(REGISTRY)/$(REPO):v$(VERSION)"
	@echo "        $(REGISTRY)/$(REPO):latest"
	@echo ""
	@docker buildx bake app --push \
		--set app.tags=$(REGISTRY)/$(REPO):v$(VERSION) \
		--set app.tags=$(REGISTRY)/$(REPO):latest
	@echo ""
	@echo "$(GREEN)✓ Release build pushed!$(NC)"
	@echo "  Image: $(REGISTRY)/$(REPO):v$(VERSION)"
	@echo "  Image: $(REGISTRY)/$(REPO):latest"

.PHONY: _release-build-cloud
_release-build-cloud:
	@echo ""
	@echo "$(CYAN)Triggering GitHub Actions build...$(NC)"
	@echo "  Tags: v$(VERSION), latest"
	@gh workflow run docker-build.yml -f tag=v$(VERSION) -f tag_type=release
	@echo ""
	@echo "$(GREEN)✓ Workflow triggered!$(NC)"
	@echo "  Check status: make push-status"

.PHONY: changelog
changelog: ## Preview changelog for upcoming release
	@echo "$(CYAN)========================================$(NC)"
	@echo "$(CYAN)  Changelog Preview$(NC)"
	@echo "$(CYAN)========================================$(NC)"
	@echo ""
	@echo "$(CYAN)Version:$(NC)  v$(VERSION)"
	@echo "$(CYAN)Previous:$(NC) $(if $(PREV_TAG),$(PREV_TAG),(none))"
	@echo ""
	@echo "$(YELLOW)Commits:$(NC)"
	@echo ""
	@if [ -n "$(PREV_TAG)" ]; then \
		git log $(PREV_TAG)..HEAD --pretty=format:"- %s (%h)" ; \
	else \
		git log --pretty=format:"- %s (%h)" ; \
	fi
	@echo ""
	@echo ""
	@echo "$(YELLOW)Grouped by type (if using conventional commits):$(NC)"
	@echo ""
	@echo "$(GREEN)Features:$(NC)"
	@if [ -n "$(PREV_TAG)" ]; then \
		git log $(PREV_TAG)..HEAD --pretty=format:"- %s" | grep -iE "^- feat" || echo "  (none)"; \
	else \
		git log --pretty=format:"- %s" | grep -iE "^- feat" || echo "  (none)"; \
	fi
	@echo ""
	@echo "$(RED)Bug Fixes:$(NC)"
	@if [ -n "$(PREV_TAG)" ]; then \
		git log $(PREV_TAG)..HEAD --pretty=format:"- %s" | grep -iE "^- fix" || echo "  (none)"; \
	else \
		git log --pretty=format:"- %s" | grep -iE "^- fix" || echo "  (none)"; \
	fi
	@echo ""
	@echo "$(CYAN)Other:$(NC)"
	@if [ -n "$(PREV_TAG)" ]; then \
		git log $(PREV_TAG)..HEAD --pretty=format:"- %s" | grep -ivE "^- (feat|fix)" || echo "  (none)"; \
	else \
		git log --pretty=format:"- %s" | grep -ivE "^- (feat|fix)" || echo "  (none)"; \
	fi

.PHONY: version
version: ## Show current version and how to bump
	@echo "$(CYAN)Current version:$(NC) v$(VERSION)"
	@echo ""
	@echo "$(YELLOW)To bump version:$(NC)"
	@echo "  npm version patch  # $(VERSION) → $$(node -p \"'$(VERSION)'.split('.').map((v,i)=>i==2?+v+1:v).join('.')\")"
	@echo "  npm version minor  # $(VERSION) → $$(node -p \"'$(VERSION)'.split('.').map((v,i)=>i==1?+v+1:i==2?0:v).join('.')\")"
	@echo "  npm version major  # $(VERSION) → $$(node -p \"'$(VERSION)'.split('.').map((v,i)=>i==0?+v+1:0).join('.')\")"

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
	@echo "  Version:    v$(VERSION)"
	@echo "  Git Hash:   $(GIT_HASH)"
	@echo ""
	@echo "$(CYAN)Docker Buildx Info:$(NC)"
	@docker buildx ls

# Default target
.DEFAULT_GOAL := help
