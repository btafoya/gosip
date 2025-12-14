# GoSIP Makefile
# SIP-to-Twilio Bridge PBX

.PHONY: all build run dev test lint clean docker docker-dev help

# Variables
BINARY_NAME=gosip
MAIN_PATH=./cmd/gosip
FRONTEND_DIR=frontend
DOCKER_IMAGE=gosip:latest
GO_FILES=$(shell find . -name '*.go' -not -path './vendor/*')

# Default target
all: build

## Build commands
build: build-frontend build-backend ## Build both frontend and backend

build-backend: ## Build Go backend
	@echo "Building backend..."
	CGO_ENABLED=1 go build -o bin/$(BINARY_NAME) $(MAIN_PATH)

build-frontend: ## Build Vue frontend
	@echo "Building frontend..."
	cd $(FRONTEND_DIR) && pnpm install && pnpm build

build-linux: ## Build for Linux (cross-compile)
	@echo "Building for Linux..."
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)

## Run commands
run: build-backend ## Run the application
	@echo "Running GoSIP..."
	./bin/$(BINARY_NAME)

dev: ## Run in development mode with hot reload
	@echo "Starting development mode..."
	@if command -v air > /dev/null; then \
		air -c .air.toml; \
	else \
		echo "Installing air..."; \
		go install github.com/air-verse/air@latest; \
		air -c .air.toml; \
	fi

dev-frontend: ## Run frontend dev server
	@echo "Starting frontend dev server..."
	cd $(FRONTEND_DIR) && pnpm dev

dev-all: ## Run both backend and frontend in development mode
	@echo "Starting full development environment..."
	@make -j2 dev dev-frontend

## Test commands
test: ## Run all tests
	@echo "Running tests..."
	go test -v -race ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	go test -v -race -tags=integration ./...

## Lint commands
lint: ## Run linters
	@echo "Running linters..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run; \
	fi

lint-frontend: ## Lint frontend code
	@echo "Linting frontend..."
	cd $(FRONTEND_DIR) && pnpm lint

lint-all: lint lint-frontend ## Run all linters

fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w $(GO_FILES)

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

## Docker commands
docker: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

docker-dev: ## Run development Docker environment
	@echo "Starting Docker development environment..."
	docker-compose -f docker-compose.dev.yml up --build

docker-up: ## Start Docker production environment
	@echo "Starting Docker production environment..."
	docker-compose up -d

docker-down: ## Stop Docker environment
	@echo "Stopping Docker environment..."
	docker-compose down

docker-logs: ## View Docker logs
	docker-compose logs -f

docker-clean: ## Clean Docker resources
	@echo "Cleaning Docker resources..."
	docker-compose down -v --rmi local

## Database commands
db-migrate: ## Run database migrations
	@echo "Running migrations..."
	./bin/$(BINARY_NAME) migrate up

db-rollback: ## Rollback last migration
	@echo "Rolling back migration..."
	./bin/$(BINARY_NAME) migrate down

db-reset: ## Reset database (WARNING: destroys data)
	@echo "Resetting database..."
	rm -f data/gosip.db
	./bin/$(BINARY_NAME) migrate up

## Utility commands
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	cd $(FRONTEND_DIR) && pnpm install

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy
	cd $(FRONTEND_DIR) && pnpm update

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf tmp/
	rm -rf coverage.out coverage.html
	rm -rf $(FRONTEND_DIR)/dist
	rm -rf $(FRONTEND_DIR)/node_modules

install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

## Environment setup
setup: deps install-tools ## Set up development environment
	@echo "Setting up development environment..."
	@mkdir -p data/recordings data/voicemails data/backups
	@echo "Development environment ready!"

## Documentation
docs: ## Generate documentation
	@echo "Generating documentation..."
	@if command -v godoc > /dev/null; then \
		echo "Starting godoc server at http://localhost:6060"; \
		godoc -http=:6060; \
	else \
		echo "Installing godoc..."; \
		go install golang.org/x/tools/cmd/godoc@latest; \
		godoc -http=:6060; \
	fi

## Release commands
version: ## Show version
	@cat VERSION 2>/dev/null || echo "0.1.0"

release: test lint build ## Build release artifacts
	@echo "Building release..."
	@mkdir -p dist
	@cp bin/$(BINARY_NAME) dist/
	@tar -czvf dist/$(BINARY_NAME)-$(shell cat VERSION 2>/dev/null || echo "0.1.0").tar.gz -C dist $(BINARY_NAME)
	@echo "Release artifacts in dist/"

## Help
help: ## Show this help
	@echo "GoSIP - SIP-to-Twilio Bridge PBX"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
