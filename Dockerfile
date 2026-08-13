# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
    -ldflags="-s -w \
      -X github.com/fl0w1nd/proxy-rule-manager/version.Version=$VERSION \
      -X github.com/fl0w1nd/proxy-rule-manager/version.Commit=$COMMIT \
      -X github.com/fl0w1nd/proxy-rule-manager/version.Date=$BUILD_DATE" \
    -o /prm ./cmd/prm

FROM gcr.io/distroless/static-debian12

ARG VERSION
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="Proxy Rule Manager" \
      org.opencontainers.image.description="Compile proxy routing rules into client-specific formats" \
      org.opencontainers.image.source="https://github.com/fl0w1nd/Proxy-Rule-Manager" \
      org.opencontainers.image.version=$VERSION \
      org.opencontainers.image.revision=$COMMIT \
      org.opencontainers.image.created=$BUILD_DATE

COPY --from=builder /prm /usr/local/bin/prm

ENV PRM_DATA_DIR=/data \
    PRM_SERVE_HOST=0.0.0.0 \
    PRM_SERVE_PORT=3001 \
    PRM_TRUSTED_PROXIES=127.0.0.1/32,172.16.0.0/12

VOLUME ["/data"]
WORKDIR /data

EXPOSE 3001

ENTRYPOINT ["prm"]
CMD ["serve"]
