// Docker Buildx Bake configuration
// Usage: docker buildx bake [target] --push

variable "REGISTRY" {
  default = "ghcr.io"
}

variable "REPO" {
  default = "fl0w1nd/proxy-rule-manager"
}

variable "TAG" {
  default = "latest"
}

// Default target group
group "default" {
  targets = ["app"]
}

// Main application target
target "app" {
  dockerfile = "Dockerfile"
  context    = "."
  tags = [
    "${REGISTRY}/${REPO}:${TAG}",
    "${REGISTRY}/${REPO}:latest"
  ]
  platforms = ["linux/amd64", "linux/arm64"]
  cache-from = ["type=gha"]
  cache-to   = ["type=gha,mode=max"]
}

// Local build target (single platform, faster)
target "local" {
  inherits = ["app"]
  platforms = []  // Use current platform only
  cache-from = ["type=local,src=/tmp/.buildx-cache"]
  cache-to   = ["type=local,dest=/tmp/.buildx-cache-new,mode=max"]
}

// Development target (no push, load to local docker)
target "dev" {
  inherits = ["local"]
  tags     = ["proxy-rule-manager:dev"]
  output   = ["type=docker"]
}
