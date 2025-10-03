# Makefile for Message Dispatcher Service
# Provides convenient commands for development, testing, and deployment

.PHONY: help build run test clean docker-build docker-run migrate deps lint build-all release

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS = -ldflags="-w -s -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Default target
help: ## Show this help message
	@echo "Message Dispatcher Service - Development Commands"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

# Development commands
deps: ## Download Go module dependencies
	go mod download
	go mod tidy

build: ## Build the application binary with version information
	@echo "Building version $(VERSION)..."
	go build $(LDFLAGS) -o bin/server cmd/server/main.go
	go build $(LDFLAGS) -o bin/migrate cmd/migrate/main.go
	go build -o bin/mock-api cmd/mock-api/main.go

run: ## Run the application locally
	go run cmd/server/main.go

run-mock-api: ## Run the Go mock SMS API server
	go run cmd/mock-api/main.go

migrate: ## Run database migrations
	go run cmd/migrate/main.go

# Testing commands
test: ## Run unit tests
	go test -v ./...

test-coverage: ## Run tests with coverage report
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-integration: ## Run integration tests (requires running dependencies)
	go test -tags=integration -v ./...

# Code quality
lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format Go code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

# Docker commands
docker-build: ## Build Docker image
	docker build -t message-dispatcher:latest .

docker-run: ## Run application in Docker with dependencies
	docker-compose up --build

docker-deps: ## Start only dependencies (PostgreSQL, Redis, Mock API)
	docker-compose up -d postgres redis mock-sms-api

docker-stop: ## Stop all Docker containers
	docker-compose down

docker-clean: ## Remove Docker containers and volumes
	docker-compose down -v
	docker system prune -f

# Development workflow
dev-setup: deps docker-deps migrate ## Setup development environment
	@echo "Development environment ready!"
	@echo "Run 'make run' to start the application"

dev-reset: docker-clean dev-setup ## Reset development environment

# Production commands
build-prod: ## Build optimized production binary
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo $(LDFLAGS) -o bin/server-prod cmd/server/main.go

# Cross-platform build targets
build-all: build-windows build-linux build-darwin ## Build binaries for all platforms

build-windows: ## Build Windows binaries
	@echo "Building Windows binaries..."
	@mkdir -p dist/windows
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/windows/message-dispatcher-server.exe cmd/server/main.go
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/windows/message-dispatcher-migrate.exe cmd/migrate/main.go
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/windows/mock-sms-api.exe cmd/mock-api/main.go

build-linux: ## Build Linux binaries
	@echo "Building Linux binaries..."
	@mkdir -p dist/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/linux/message-dispatcher-server cmd/server/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/linux/message-dispatcher-migrate cmd/migrate/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/linux/mock-sms-api cmd/mock-api/main.go

build-darwin: ## Build macOS binaries (Intel and Apple Silicon)
	@echo "Building macOS binaries..."
	@mkdir -p dist/darwin-amd64 dist/darwin-arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/darwin-amd64/message-dispatcher-server cmd/server/main.go
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/darwin-amd64/message-dispatcher-migrate cmd/migrate/main.go
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/darwin-amd64/mock-sms-api cmd/mock-api/main.go
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/darwin-arm64/message-dispatcher-server cmd/server/main.go
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/darwin-arm64/message-dispatcher-migrate cmd/migrate/main.go
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/darwin-arm64/mock-sms-api cmd/mock-api/main.go

# Release management
release: ## Create a new release (requires VERSION parameter)
	@if [ -z "$(VERSION)" ] || [ "$(VERSION)" = "dev" ]; then \
		echo "Error: VERSION parameter is required for releases"; \
		echo "Usage: make release VERSION=v1.0.0"; \
		exit 1; \
	fi
	@echo "Creating release $(VERSION)..."
	@if command -v bash >/dev/null 2>&1; then \
		bash scripts/release.sh $(VERSION); \
	else \
		echo "Bash not found. Please run the release script manually:"; \
		echo "  scripts/release.sh $(VERSION)"; \
	fi

deploy: docker-build ## Deploy using Docker Compose
	docker-compose up -d

# Utility commands
logs: ## Show application logs
	docker-compose logs -f app

db-shell: ## Connect to PostgreSQL database
	docker-compose exec postgres psql -U postgres -d messages_db

redis-shell: ## Connect to Redis
	docker-compose exec redis redis-cli

clean: ## Clean build artifacts
	rm -rf bin/ dist/
	rm -f coverage.out coverage.html
	go clean ./...

# API testing
api-test: ## Test API endpoints (requires running service)
	@echo "Testing API endpoints..."
	@echo "1. Health check:"
	curl -s http://localhost:8080/health | jq .
	@echo "\n2. Start processing:"
	curl -s -X POST http://localhost:8080/api/messaging/start | jq .
	@echo "\n3. Get sent messages:"
	curl -s http://localhost:8080/api/messages/sent | jq .
	@echo "\n4. Stop processing:"
	curl -s -X POST http://localhost:8080/api/messaging/stop | jq .

# Generate sample data
sample-data: ## Insert sample messages into database
	docker-compose exec postgres psql -U postgres -d messages_db -c "INSERT INTO messages (phone_number, content) VALUES ('+1555000001', 'Sample message 1'), ('+1555000002', 'Sample message 2'), ('+1555000003', 'Sample message 3');"

# Monitoring
status: ## Show service status
	@echo "=== Service Status ==="
	@echo "Docker containers:"
	docker-compose ps
	@echo "\n=== Health Check ==="
	@curl -s http://localhost:8080/health 2>/dev/null | jq . || echo "Service not responding"

# Documentation
docs: ## Generate/view API documentation
	@echo "API Documentation available at:"
	@echo "- Swagger YAML: docs/swagger.yaml"
	@echo "- README: README.md"
	@echo ""
	@echo "To view Swagger UI, run the service and visit:"
	@echo "http://localhost:8080/swagger/index.html"