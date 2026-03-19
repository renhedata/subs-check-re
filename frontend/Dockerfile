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

# Production image - nginx serves static files directly
FROM nginx:alpine
COPY --from=builder /app/apps/web/dist /usr/share/nginx/html
RUN printf 'server {\n    listen 3000;\n    root /usr/share/nginx/html;\n    index index.html;\n    location / {\n        try_files $uri $uri/ /index.html;\n    }\n}\n' > /etc/nginx/conf.d/default.conf
EXPOSE 3000
