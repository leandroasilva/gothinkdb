# Multi-stage Dockerfile for GoThinkDB
# Stage 1: Build the Go binary
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /build

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
# CGO_ENABLED=0 for static binary
# -ldflags="-s -w" to strip debug info and reduce size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o /gothinkdb \
    ./cmd/gothinkdb

# Stage 2: Create minimal runtime image
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && addgroup -S gothinkdb \
    && adduser -S gothinkdb -G gothinkdb

# Create data directory
RUN mkdir -p /data/gothinkdb && chown gothinkdb:gothinkdb /data/gothinkdb

WORKDIR /app

# Copy binary from builder
COPY --from=builder /gothinkdb /app/gothinkdb

# Expose ports
# 28015 - Driver protocol (ReQL)
# 8080  - HTTP admin interface
# 29015 - Cluster communication
EXPOSE 28015 8080 29015

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/api/health || exit 1

# Switch to non-root user
USER gothinkdb

# Data volume
VOLUME ["/data/gothinkdb"]

# Entry point
ENTRYPOINT ["/app/gothinkdb"]

# Default arguments
CMD ["-data", "/data/gothinkdb"]
