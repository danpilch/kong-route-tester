# Kong Route Tester Makefile

# Variables
MAIN_BINARY := kong-route-tester
TEST_SERVER_BINARY := test-server-bin
GO_FILES := $(shell find . -name '*.go' -not -path './test-server/*')
TEST_SERVER_FILES := $(shell find ./test-server -name '*.go')
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Default target
.PHONY: all
all: build

# Build targets
.PHONY: build
build: $(MAIN_BINARY) $(TEST_SERVER_BINARY)

$(MAIN_BINARY): $(GO_FILES)
	@echo "Building $(MAIN_BINARY)..."
	go build $(LDFLAGS) -o $(MAIN_BINARY) main.go

$(TEST_SERVER_BINARY): $(TEST_SERVER_FILES)
	@echo "Building $(TEST_SERVER_BINARY)..."
	go build -o $(TEST_SERVER_BINARY) ./test-server

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(MAIN_BINARY) $(TEST_SERVER_BINARY)
	go clean

# Test targets
.PHONY: test
test: build
	@echo "Running all tests..."
	go test -v ./...

.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	go test -v -short ./...

.PHONY: test-integration
test-integration: build
	@echo "Running integration tests..."
	go test -v -run TestIntegration ./...

.PHONY: test-coverage
test-coverage: build
	@echo "Running tests with coverage..."
	go test -v -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: test-race
test-race: build
	@echo "Running tests with race detection..."
	go test -v -race ./...

# Lint and format targets
.PHONY: fmt
fmt:
	@echo "Formatting Go code..."
	go fmt ./...

.PHONY: vet
vet:
	@echo "Running go vet..."
	go vet ./...

.PHONY: lint
lint: fmt vet
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping advanced linting"; \
		echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Development targets
.PHONY: dev
dev: lint test
	@echo "Development build complete!"

.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

.PHONY: tidy
tidy:
	@echo "Tidying go modules..."
	go mod tidy

# Demo and example targets
.PHONY: demo
demo: build start-test-server
	@echo "Running demo against local test server..."
	@sleep 2
	./$(MAIN_BINARY) --url=http://127.0.0.1:8080 --max=10 --verbose
	@$(MAKE) stop-test-server

.PHONY: demo-auth
demo-auth: build start-test-server-auth
	@echo "Running auth demo against local test server..."
	@sleep 2
	@echo "Testing without auth token (should see 401s):"
	./$(MAIN_BINARY) --url=http://127.0.0.1:8080 --max=5 --verbose
	@echo "\nTesting with auth token (should succeed):"
	./$(MAIN_BINARY) --url=http://127.0.0.1:8080 --token=test-token-123 --max=5 --verbose
	@$(MAKE) stop-test-server

.PHONY: start-test-server
start-test-server: $(TEST_SERVER_BINARY)
	@echo "Starting test server..."
	./$(TEST_SERVER_BINARY) --port=8080 --verbose &
	@echo $$! > test-server-bin.pid

.PHONY: start-test-server-auth
start-test-server-auth: $(TEST_SERVER_BINARY)
	@echo "Starting test server with authentication..."
	./$(TEST_SERVER_BINARY) --port=8080 --require-auth --auth-token=test-token-123 --verbose &
	@echo $$! > test-server-bin.pid

.PHONY: stop-test-server
stop-test-server:
	@if [ -f test-server-bin.pid ]; then \
		echo "Stopping test server..."; \
		kill `cat test-server-bin.pid` 2>/dev/null || true; \
		rm -f test-server-bin.pid; \
	else \
		pkill -f "test-server-bin" || true; \
	fi

# Docker targets
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker build -t kong-route-tester:$(VERSION) .
	docker tag kong-route-tester:$(VERSION) kong-route-tester:latest

.PHONY: docker-run
docker-run:
	@echo "Running Docker container..."
	docker run --rm kong-route-tester:latest --help

.PHONY: docker-demo
docker-demo:
	@echo "Running Docker demo..."
	docker run --rm -v $(PWD)/kong.yaml:/root/kong.yaml kong-route-tester:latest --file=kong.yaml --dry-run --verbose

# Release targets
.PHONY: release-build
release-build: clean
	@echo "Building release binaries..."
	@mkdir -p dist
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(MAIN_BINARY)-linux-amd64 main.go
	GOOS=linux GOARCH=amd64 go build -o dist/$(TEST_SERVER_BINARY)-linux-amd64 ./test-server
	
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(MAIN_BINARY)-linux-arm64 main.go
	GOOS=linux GOARCH=arm64 go build -o dist/$(TEST_SERVER_BINARY)-linux-arm64 ./test-server
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(MAIN_BINARY)-darwin-amd64 main.go
	GOOS=darwin GOARCH=amd64 go build -o dist/$(TEST_SERVER_BINARY)-darwin-amd64 ./test-server
	
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(MAIN_BINARY)-darwin-arm64 main.go
	GOOS=darwin GOARCH=arm64 go build -o dist/$(TEST_SERVER_BINARY)-darwin-arm64 ./test-server
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(MAIN_BINARY)-windows-amd64.exe main.go
	GOOS=windows GOARCH=amd64 go build -o dist/$(TEST_SERVER_BINARY)-windows-amd64.exe ./test-server
	
	@echo "Release binaries built in dist/"

# Utility targets
.PHONY: help
help:
	@echo "Kong Route Tester - Available Make targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  build          - Build all binaries"
	@echo "  clean          - Clean build artifacts"
	@echo "  release-build  - Build release binaries for multiple platforms"
	@echo ""
	@echo "Test targets:"
	@echo "  test           - Run all tests"
	@echo "  test-unit      - Run unit tests only"
	@echo "  test-integration - Run integration tests only"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-race      - Run tests with race detection"
	@echo ""
	@echo "Code quality targets:"
	@echo "  fmt            - Format Go code"
	@echo "  vet            - Run go vet"
	@echo "  lint           - Run linters (fmt + vet + golangci-lint)"
	@echo ""
	@echo "Development targets:"
	@echo "  dev            - Run lint and test (full dev workflow)"
	@echo "  deps           - Download dependencies"
	@echo "  tidy           - Tidy go modules"
	@echo ""
	@echo "Demo targets:"
	@echo "  demo           - Run demo against local test server"
	@echo "  demo-auth      - Run auth demo (with and without tokens)"
	@echo "  start-test-server - Start test server in background"
	@echo "  start-test-server-auth - Start test server with auth"
	@echo "  stop-test-server - Stop background test server"
	@echo ""
	@echo "Docker targets:"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  docker-demo    - Run Docker demo"
	@echo ""
	@echo "Utility targets:"
	@echo "  help           - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make dev       - Full development workflow"
	@echo "  make demo      - Quick demo of the tool"
	@echo "  make test-coverage - Generate test coverage report"

.PHONY: version
version:
	@echo "Kong Route Tester $(VERSION) (built $(BUILD_TIME))"

# Cleanup on interrupt
.PHONY: interrupt-cleanup
interrupt-cleanup:
	@$(MAKE) stop-test-server

# Ensure test server is stopped on make clean
clean: stop-test-server

# Dependencies for demo targets
demo: stop-test-server
demo-auth: stop-test-server