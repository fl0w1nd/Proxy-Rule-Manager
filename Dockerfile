# syntax=docker/dockerfile:1.7

# Build stage
FROM node:22-alpine AS builder

WORKDIR /app

# Install dependencies (cache-friendly)
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --prefer-offline

# Install esbuild for bundling (do not modify lock/package)
RUN --mount=type=cache,target=/root/.npm \
    npm i -D esbuild --no-save --no-audit --prefer-offline

# Copy source
COPY . .

# Build frontend (Next.js static export)
RUN npm run build

# Bundle server
RUN npx esbuild src/server/index.ts --bundle --platform=node --target=node22 --outfile=dist/server.js

# Production stage
FROM node:22-alpine AS runner

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
