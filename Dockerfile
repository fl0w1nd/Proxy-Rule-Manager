# Build stage
FROM node:22-alpine AS builder

WORKDIR /app

# Install dependencies
COPY package*.json ./
RUN npm ci

# Copy source
COPY . .

# Build frontend (Next.js static export)
RUN npm run build

# Build backend (Bundle into single file)
# Install esbuild
RUN npm install esbuild --save-dev

# Bundle server
# --bundle: bundle dependencies
# --platform=node: target nodejs
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
