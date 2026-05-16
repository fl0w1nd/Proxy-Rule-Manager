# syntax=docker/dockerfile:1.7

############################
# Stage 1: build frontend  #
############################
FROM node:26-alpine AS frontend
WORKDIR /app
RUN corepack enable
COPY package.json pnpm-lock.yaml* ./
RUN --mount=type=cache,target=/pnpm/store \
    pnpm install --frozen-lockfile --prefer-offline || pnpm install --prefer-offline
COPY . .
RUN pnpm run build

############################
# Stage 2: build backend   #
############################
FROM golang:1.26-alpine AS backend
WORKDIR /src
RUN apk add --no-cache git
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG TARGETOS=linux
ARG TARGETARCH=amd64
# VERSION is injected by CI from package.json (see .github/workflows/*.yml).
# It is stamped into the Go binary via -ldflags -X and surfaced through
# /api/status. Defaults to "dev" so plain `docker build` still works.
ARG VERSION=dev
ENV CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH}
RUN go build -trimpath \
        -ldflags="-s -w -X github.com/fl0w1nd/proxy-rule-manager/backend/internal/api.Version=${VERSION}" \
        -o /out/proxy-rule-manager ./cmd/server

############################
# Stage 3: runtime         #
############################
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.source="https://github.com/fl0w1nd/proxy-rule-manager"
LABEL org.opencontainers.image.description="代理规则集编排管理 WebUI (Go backend)"
LABEL org.opencontainers.image.licenses="MIT"

WORKDIR /app
COPY --from=frontend /app/out ./out
COPY --from=backend  /out/proxy-rule-manager ./proxy-rule-manager

ENV PORT=3000
ENV DATA_DIR=/app/data
ENV OUT_DIR=/app/out
EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/app/proxy-rule-manager", "--healthcheck"]

USER nonroot:nonroot
ENTRYPOINT ["/app/proxy-rule-manager"]
