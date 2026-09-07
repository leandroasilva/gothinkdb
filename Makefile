# GoThinkDB Makefile
# Build, test, and run GoThinkDB via Docker

.PHONY: help build test docker-build docker-up docker-down docker-logs clean deps fmt lint

# Default target
help:
	@echo "GoThinkDB - RethinkDB compatible database in Go"
	@echo ""
	@echo "Available targets:"
	@echo "  build          - Build the Go binary locally (requires Go)"
	@echo "  test           - Run all tests locally (requires Go)"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-up      - Start single-node cluster via Docker"
	@echo "  docker-down    - Stop single-node cluster"
	@echo "  docker-cluster-up   - Start 3-node cluster via Docker"
	@echo "  docker-cluster-down - Stop 3-node cluster"
	@echo "  docker-logs    - View container logs"
	@echo "  docker-shell   - Open shell in running container"
	@echo "  clean          - Remove build artifacts and containers"
	@echo "  deps           - Download Go dependencies"
	@echo "  fmt            - Format Go code"
	@echo "  lint           - Run linter"
	@echo ""

# Build binary locally (requires Go installed)
build:
	@echo "Building GoThinkDB..."
	go build -o bin/gothinkdb ./cmd/gothinkdb

# Run tests locally (requires Go installed)
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run tests in Docker (no local Go required)
test-docker:
	@echo "Running tests in Docker..."
	docker run --rm -v $(PWD):/app -w /app golang:1.23-alpine \
		sh -c "go test -v -race -coverprofile=/tmp/coverage.out ./... && \
		       go tool cover -html=/tmp/coverage.out -o /tmp/coverage.html && \
		       cp /tmp/coverage.out coverage.out && \
		       cp /tmp/coverage.html coverage.html"

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t gothinkdb:latest .

# Start single-node cluster
docker-up: docker-build
	@echo "Starting single-node GoThinkDB..."
	docker compose up -d
	@echo ""
	@echo "GoThinkDB is running!"
	@echo "  Driver protocol: localhost:28015"
	@echo "  HTTP admin:      http://localhost:8080"
	@echo "  Health check:    http://localhost:8080/api/health"
	@echo ""
	@echo "Use 'make docker-logs' to view logs"
	@echo "Use 'make docker-down' to stop"

# Stop single-node cluster
docker-down:
	@echo "Stopping GoThinkDB..."
	docker compose down

# Start 3-node cluster
docker-cluster-up: docker-build
	@echo "Starting 3-node GoThinkDB cluster..."
	docker compose -f docker-compose.cluster.yml up -d
	@echo ""
	@echo "GoThinkDB cluster is running!"
	@echo "  Node 1 - Driver: localhost:28015, HTTP: http://localhost:8080"
	@echo "  Node 2 - Driver: localhost:28016, HTTP: http://localhost:8081"
	@echo "  Node 3 - Driver: localhost:28017, HTTP: http://localhost:8082"
	@echo ""
	@echo "Use 'make docker-logs' to view logs"
	@echo "Use 'make docker-cluster-down' to stop"

# Stop 3-node cluster
docker-cluster-down:
	@echo "Stopping GoThinkDB cluster..."
	docker compose -f docker-compose.cluster.yml down

# View container logs
docker-logs:
	docker compose logs -f

# Open shell in running container
docker-shell:
	docker compose exec gothinkdb sh

# Clean build artifacts and containers
clean:
	@echo "Cleaning up..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	docker compose down -v 2>/dev/null || true
	docker compose -f docker-compose.cluster.yml down -v 2>/dev/null || true
	docker rmi gothinkdb:latest 2>/dev/null || true
	@echo "Clean complete"

# Download Go dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Format Go code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Run development server with hot-reload (requires air)
dev:
	@echo "Starting development server..."
	air -c .air.toml

# Initialize Go module (run once)
init:
	@echo "Initializing Go module..."
	go mod init github.com/leandroasilva/gothinkdb
	go mod tidy
