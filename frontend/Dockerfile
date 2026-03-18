FROM oven/bun:1.3.10-alpine AS builder

WORKDIR /app

# Copy workspace manifests first for layer caching
COPY package.json bun.lock ./
COPY apps/web/package.json ./apps/web/
COPY packages/ui/package.json ./packages/ui/
COPY packages/env/package.json ./packages/env/
COPY packages/config/package.json ./packages/config/

RUN bun install --frozen-lockfile

# Copy source
COPY . .

# Build
RUN bun run build

# Production image
FROM oven/bun:1.3.10-alpine

WORKDIR /app
COPY --from=builder /app/apps/web/dist ./dist

EXPOSE 3000
CMD ["bunx", "serve", "dist", "-p", "3000"]
