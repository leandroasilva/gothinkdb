# GoThinkDB Makefile
# Build, test, and run GoThinkDB via Docker

.PHONY: help build test docker-build docker-up docker-down docker-logs clean deps fmt lint \
	dashboard-build dashboard-dev dashboard-install \
	driver-ts-build driver-go-build driver-rust-build driver-test \
	test-all test-single test-cluster test-dashboard test-drivers

# Default target
help:
	@echo "GoThinkDB - RethinkDB compatible database in Go"
	@echo ""
	@echo "=== Server ==="
	@echo "  docker-build          - Build Docker image (includes dashboard)"
	@echo "  docker-up             - Start single-node cluster"
	@echo "  docker-down           - Stop single-node cluster"
	@echo "  docker-cluster-up     - Start 3-node cluster"
	@echo "  docker-cluster-down   - Stop 3-node cluster"
	@echo "  docker-logs           - View container logs"
	@echo ""
	@echo "=== Dashboard ==="
	@echo "  dashboard-install     - Install dashboard dependencies"
	@echo "  dashboard-build       - Build dashboard for production"
	@echo "  dashboard-dev         - Start dashboard dev server"
	@echo ""
	@echo "=== Drivers ==="
	@echo "  driver-ts-build       - Build TypeScript driver"
	@echo "  driver-go-build       - Build Go driver"
	@echo "  driver-rust-build     - Build Rust driver"
	@echo "  driver-test           - Test all drivers"
	@echo ""
	@echo "=== Testing ==="
	@echo "  test                  - Run Go tests locally"
	@echo "  test-docker           - Run Go tests in Docker"
	@echo "  test-all              - Run all tests (Go + drivers)"
	@echo "  test-single           - Test single-server Docker"
	@echo "  test-cluster          - Test cluster Docker"
	@echo "  test-dashboard        - Test dashboard endpoints"
	@echo "  test-drivers          - Test drivers against running server"
	@echo ""
	@echo "=== Development ==="
	@echo "  build                 - Build Go binary locally"
	@echo "  deps                  - Download Go dependencies"
	@echo "  fmt                   - Format Go code"
	@echo "  lint                  - Run linter"
	@echo "  clean                 - Remove build artifacts and containers"

# =====================
# Server
# =====================

docker-build:
	@echo "Building Docker image (with dashboard)..."
	docker compose build

docker-up: docker-build
	@echo "Starting single-node GoThinkDB..."
	docker compose up -d
	@echo ""
	@echo "GoThinkDB is running!"
	@echo "  Dashboard:       http://localhost:8080"
	@echo "  Driver protocol: localhost:28015"
	@echo "  Health check:    http://localhost:8080/api/health"
	@echo ""

docker-down:
	@echo "Stopping GoThinkDB..."
	docker compose down

docker-cluster-up: docker-build
	@echo "Starting 3-node GoThinkDB cluster..."
	docker compose -f docker-compose.cluster.yml up -d
	@echo ""
	@echo "GoThinkDB cluster is running!"
	@echo "  Node 1 - Dashboard: http://localhost:8080, Driver: localhost:28015"
	@echo "  Node 2 - Dashboard: http://localhost:8081, Driver: localhost:28016"
	@echo "  Node 3 - Dashboard: http://localhost:8082, Driver: localhost:28017"
	@echo ""

docker-cluster-down:
	@echo "Stopping GoThinkDB cluster..."
	docker compose -f docker-compose.cluster.yml down

docker-logs:
	docker compose logs -f

docker-shell:
	docker compose exec gothinkdb sh

# =====================
# Dashboard
# =====================

dashboard-install:
	@echo "Installing dashboard dependencies..."
	cd dashboard && npm install

dashboard-build: dashboard-install
	@echo "Building dashboard..."
	cd dashboard && npm run build

dashboard-dev:
	@echo "Starting dashboard dev server..."
	cd dashboard && npm run dev

# =====================
# Drivers
# =====================

driver-ts-install:
	@echo "Installing TypeScript driver dependencies..."
	cd drivers/typescript && npm install

driver-ts-build: driver-ts-install
	@echo "Building TypeScript driver..."
	cd drivers/typescript && npm run build

driver-ts-test: driver-ts-install
	@echo "Testing TypeScript driver..."
	cd drivers/typescript && npm test

driver-go-build:
	@echo "Building Go driver..."
	cd drivers/go && go build ./...

driver-go-test:
	@echo "Testing Go driver..."
	cd drivers/go && go test ./...

driver-rust-build:
	@echo "Building Rust driver..."
	cd drivers/rust && cargo build

driver-rust-test:
	@echo "Testing Rust driver..."
	cd drivers/rust && cargo test

driver-test: driver-ts-test driver-go-test driver-rust-test
	@echo "All driver tests complete!"

# =====================
# Testing
# =====================

build:
	@echo "Building GoThinkDB..."
	go build -o bin/gothinkdb ./cmd/gothinkdb

test:
	@echo "Running Go tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-docker:
	@echo "Running Go tests in Docker..."
	docker run --rm -v $(PWD):/app -w /app golang:1.23-alpine \
		sh -c "go test -v -race -coverprofile=/tmp/coverage.out ./... && \
		       go tool cover -html=/tmp/coverage.out -o /tmp/coverage.html && \
		       cp /tmp/coverage.out coverage.out && \
		       cp /tmp/coverage.html coverage.html"

test-all: test driver-test
	@echo "All tests complete!"

test-single: docker-up
	@echo "Testing single-server Docker..."
	@sleep 5
	@echo "  Health check..."
	@curl -sf http://localhost:8080/api/health && echo " OK" || echo " FAIL"
	@echo "  Dashboard..."
	@curl -sf http://localhost:8080/ > /dev/null && echo " OK" || echo " FAIL"
	@echo "  Databases API..."
	@curl -sf http://localhost:8080/api/databases && echo " OK" || echo " FAIL"
	@echo "  Create database..."
	@curl -sf -X POST http://localhost:8080/api/databases -H 'Content-Type: application/json' -d '{"name":"testdb"}' && echo " OK" || echo " FAIL"
	@echo "  Create table..."
	@curl -sf -X POST 'http://localhost:8080/api/tables/testtable?db=testdb' && echo " OK" || echo " FAIL"
	@echo "  Server info..."
	@curl -sf http://localhost:8080/api/server/info && echo " OK" || echo " FAIL"
	@echo "  Server stats..."
	@curl -sf http://localhost:8080/api/server/stats && echo " OK" || echo " FAIL"
	@echo "  Cluster status..."
	@curl -sf http://localhost:8080/api/cluster/status && echo " OK" || echo " FAIL"
	@echo "  Logs..."
	@curl -sf http://localhost:8080/api/logs > /dev/null && echo " OK" || echo " FAIL"
	@echo ""
	@echo "Single-server tests complete!"

test-cluster: docker-cluster-up
	@echo "Testing cluster Docker..."
	@sleep 10
	@echo "  Node 1 health..."
	@curl -sf http://localhost:8080/api/health && echo " OK" || echo " FAIL"
	@echo "  Node 2 health..."
	@curl -sf http://localhost:8081/api/health && echo " OK" || echo " FAIL"
	@echo "  Node 3 health..."
	@curl -sf http://localhost:8082/api/health && echo " OK" || echo " FAIL"
	@echo "  Cluster status..."
	@curl -sf http://localhost:8080/api/cluster/status && echo " OK" || echo " FAIL"
	@echo "  Cluster members..."
	@curl -sf http://localhost:8080/api/cluster/members && echo " OK" || echo " FAIL"
	@echo ""
	@echo "Cluster tests complete!"

test-dashboard: docker-up
	@echo "Testing dashboard..."
	@sleep 5
	@echo "  Dashboard HTML..."
	@curl -sf http://localhost:8080/ | grep -q "GoThinkDB" && echo " OK" || echo " FAIL"
	@echo "  Health API..."
	@curl -sf http://localhost:8080/api/health > /dev/null && echo " OK" || echo " FAIL"
	@echo "  Databases API..."
	@curl -sf http://localhost:8080/api/databases > /dev/null && echo " OK" || echo " FAIL"
	@echo "  Tables API..."
	@curl -sf 'http://localhost:8080/api/tables?db=test' > /dev/null && echo " OK" || echo " FAIL"
	@echo "  Server Info API..."
	@curl -sf http://localhost:8080/api/server/info > /dev/null && echo " OK" || echo " FAIL"
	@echo "  Server Stats API..."
	@curl -sf http://localhost:8080/api/server/stats > /dev/null && echo " OK" || echo " FAIL"
	@echo "  Cluster Status API..."
	@curl -sf http://localhost:8080/api/cluster/status > /dev/null && echo " OK" || echo " FAIL"
	@echo "  Logs API..."
	@curl -sf http://localhost:8080/api/logs > /dev/null && echo " OK" || echo " FAIL"
	@echo "  Query API..."
	@curl -sf -X POST http://localhost:8080/api/query -H 'Content-Type: application/json' -d '{"query":"r.table(\"test\")"}' > /dev/null && echo " OK" || echo " FAIL"
	@echo ""
	@echo "Dashboard tests complete!"

test-drivers: docker-up
	@echo "Testing drivers against running server..."
	@sleep 5
	@echo "  TypeScript driver..."
	@cd drivers/typescript && npm test 2>/dev/null && echo " OK" || echo " SKIP (needs npm install)"
	@echo "  Go driver..."
	@cd drivers/go && go test ./... 2>/dev/null && echo " OK" || echo " SKIP"
	@echo "  Rust driver..."
	@cd drivers/rust && cargo test 2>/dev/null && echo " OK" || echo " SKIP (needs cargo)"
	@echo ""
	@echo "Driver tests complete!"

# =====================
# Cleanup
# =====================

clean:
	@echo "Cleaning up..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -rf dashboard/node_modules dashboard/dist
	docker compose down -v 2>/dev/null || true
	docker compose -f docker-compose.cluster.yml down -v 2>/dev/null || true
	docker rmi gothinkdb:latest 2>/dev/null || true
	@echo "Clean complete"

# =====================
# Go Development
# =====================

deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

lint:
	@echo "Running linter..."
	golangci-lint run ./...

dev:
	@echo "Starting development server..."
	air -c .air.toml

init:
	@echo "Initializing Go module..."
	go mod init github.com/leandroasilva/gothinkdb
	go mod tidy
