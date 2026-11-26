# Multi-stage production Dockerfile for Go
# Stage 1: Build binary
FROM golang:1.26.5-alpine AS builder
WORKDIR /app

# Download dependencies first for caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source and compile statically
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/dockship ./cmd/dockship

# Stage 2: Minimal secure runtime (scratch or alpine)
FROM alpine:3.19 AS runner
WORKDIR /app

# Security: unprivileged user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

COPY --from=builder /app/dockship /app/dockship


ENTRYPOINT ["/app/dockship"]
CMD ["--help"]
