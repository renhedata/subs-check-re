FROM mcr.microsoft.com/playwright:v1.51.0-noble

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

EXPOSE 3000

ENV PORT=3000
ENV MAX_CONCURRENT=5
ENV DEFAULT_TIMEOUT=30000

CMD ["npx", "tsx", "src/index.ts"]
