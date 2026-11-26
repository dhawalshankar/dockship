# Multi-stage production Dockerfile for Node.js
# Stage 1: Dependencies & Build
FROM node:20-alpine AS builder
WORKDIR /app

# Layer caching for dependency definitions
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev || npm install

# Copy source code and build
COPY . .
# No build step specified

# Stage 2: Minimal Production Runtime
FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000

# Use non-root user
USER node

COPY --from=builder --chown=node:node /app ./

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:3000/ || exit 1

CMD ["npm", "start"]
