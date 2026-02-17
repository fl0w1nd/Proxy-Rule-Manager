# syntax=docker/dockerfile:1.7

# Build stage
FROM node:22-alpine AS builder

WORKDIR /app

RUN corepack enable

# Install dependencies (cache-friendly)
COPY package.json pnpm-lock.yaml ./
RUN --mount=type=cache,target=/pnpm/store \
    pnpm install --frozen-lockfile --prefer-offline

# Install esbuild for bundling (do not modify lock/package)
RUN --mount=type=cache,target=/pnpm/store \
    pnpm add -D esbuild

# Copy source
COPY . .

# Build frontend (Next.js static export)
RUN pnpm run build

# Bundle server
RUN pnpm exec esbuild src/server/index.ts --bundle --platform=node --target=node22 --outfile=dist/server.js

# Production stage
FROM node:22-alpine AS runner

LABEL org.opencontainers.image.source="https://github.com/fl0w1nd/proxy-rule-manager"
LABEL org.opencontainers.image.description="代理规则集编排管理 WebUI"
LABEL org.opencontainers.image.licenses="MIT"

WORKDIR /app

# Copy built frontend static files
COPY --from=builder /app/out ./out

# Copy built backend server bundle
COPY --from=builder /app/dist/server.js ./server.js

# Create data directory
RUN mkdir -p /app/data

# Environment
ENV NODE_ENV=production
ENV PORT=3000
ENV DATA_DIR=/app/data

EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:3000/api/status || exit 1

# Run the bundled server
CMD ["node", "server.js"]
