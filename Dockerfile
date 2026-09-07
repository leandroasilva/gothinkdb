# Multi-stage Dockerfile for GoThinkDB
# Stage 1: Build the dashboard (React + Vite)
FROM node:20-alpine AS dashboard-builder

WORKDIR /build/dashboard

# Copy dashboard package files
COPY dashboard/package.json dashboard/package-lock.json* ./
RUN npm ci --ignore-scripts 2>/dev/null || npm install --ignore-scripts

# Copy dashboard source
COPY dashboard/ ./

# Build dashboard
RUN npm run build

# Stage 2: Build the Go binary
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /build

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy dashboard build output for embedding
COPY --from=dashboard-builder /build/dashboard/dist ./cmd/gothinkdb/dashboard/

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o /gothinkdb \
    ./cmd/gothinkdb

# Stage 3: Create minimal runtime image
FROM alpine:3.19

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && addgroup -S gothinkdb \
    && adduser -S gothinkdb -G gothinkdb

RUN mkdir -p /data/gothinkdb && chown gothinkdb:gothinkdb /data/gothinkdb

WORKDIR /app

COPY --from=builder /gothinkdb /app/gothinkdb

EXPOSE 28015 8080 29015

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/api/health || exit 1

USER gothinkdb

VOLUME ["/data/gothinkdb"]

ENTRYPOINT ["/app/gothinkdb"]
CMD ["-data", "/data/gothinkdb"]
