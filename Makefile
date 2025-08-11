# Portguard Makefile
# AI-aware process management tool

.PHONY: build test clean install run help dev

# Variables
BINARY_NAME=portguard
BUILD_DIR=bin
VERSION?=0.2.5
LDFLAGS=-ldflags "-X github.com/paveg/portguard/internal/cmd.Version=$(VERSION)"

# Default target
all: build

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/portguard
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage (for Codecov integration)
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	@echo "Coverage profile generated: coverage.out"
	@echo "Upload to Codecov for analysis and reporting"

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -race -v ./...

# Run benchmarks
test-bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Generate detailed coverage analysis (optional, for local development)
test-coverage-detailed:
	@echo "Generating detailed coverage analysis..."
	@which gocov > /dev/null || go install github.com/axw/gocov/gocov@latest
	@which gocov-html > /dev/null || go install github.com/matm/gocov-html/cmd/gocov-html@latest
	gocov test ./... | gocov-html > coverage-detailed.html
	@echo "Detailed coverage report: coverage-detailed.html"

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	rm -rf $(BUILD_DIR) build
	rm -f coverage.out coverage-detailed.html
	rm -f example.portguard.yml demo.portguard.yml

# Install to system (requires sudo on Unix)
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin/"
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

# Run in development mode
run:
	go run ./cmd/portguard

# Development tools
dev:
	@echo "Setting up development environment..."
	go mod download
	go mod tidy

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/portguard
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/portguard
	
	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/portguard
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/portguard
	
	@echo "Multi-platform build complete"

# Lint code
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	go mod tidy

# Generate documentation
docs:
	@echo "Generating documentation..."
	@mkdir -p docs
	go run ./cmd/portguard --help > docs/help.txt
	@echo "Documentation generated in docs/"

# Demo commands
demo:
	@echo "Running demo commands..."
	@echo "\n=== Initializing config ==="
	go run ./cmd/portguard config init --file demo.portguard.yml
	
	@echo "\n=== Showing help ==="
	go run ./cmd/portguard --help
	
	@echo "\n=== Checking status (JSON) ==="
	go run ./cmd/portguard check --json
	
	@echo "\n=== Showing config ==="
	go run ./cmd/portguard config show --config demo.portguard.yml
	
	@echo "\n=== Listing processes ==="
	go run ./cmd/portguard list
	
	@echo "\nDemo complete!"

# AI Integration test
ai-test:
	@echo "Testing AI integration..."
	@echo "# Port status check"
	go run ./cmd/portguard check --port 3000 --json
	@echo ""
	@echo "# Available port search"
	go run ./cmd/portguard check --available --start 3000 --json
	@echo ""
	@echo "# Process list"
	go run ./cmd/portguard list --json

# Check dependencies
deps-check:
	@echo "Checking dependencies..."
	go list -m -u all
	go mod verify

# Security scan
security:
	@echo "Running security scan..."
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	gosec ./...

# Help
help:
	@echo "Portguard Development Commands:"
	@echo ""
	@echo "Build Commands:"
	@echo "  build      - Build the application"
	@echo "  build-all  - Build for multiple platforms"
	@echo "  install    - Install to system PATH"
	@echo ""
	@echo "Development Commands:"
	@echo "  dev        - Set up development environment"
	@echo "  run        - Run in development mode"
	@echo "  fmt        - Format code"
	@echo "  lint       - Run linters"
	@echo ""
	@echo "Testing Commands:"
	@echo "  test       - Run tests"
	@echo "  test-coverage - Run tests with coverage for Codecov upload"
	@echo "  test-race  - Run tests with race detection"
	@echo "  test-bench - Run benchmarks"
	@echo "  test-coverage-detailed - Generate detailed coverage analysis"
	@echo "  security   - Run security scan"
	@echo ""
	@echo "Demo Commands:"
	@echo "  demo       - Run demo commands"
	@echo "  ai-test    - Test AI integration features"
	@echo ""
	@echo "Utility Commands:"
	@echo "  clean      - Clean build artifacts"
	@echo "  docs       - Generate documentation"
	@echo "  deps-check - Check dependencies"
	@echo "  help       - Show this help"