# Screenshot Server Go - Cross-platform Build Makefile

# Variables
BINARY_NAME=screenshot-server
VERSION?=dev
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD)
LDFLAGS=-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)

# Build directories
BUILD_DIR=build
DIST_DIR=dist

# Default target
.PHONY: all
all: clean test build-all

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@go clean

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Vet code
.PHONY: vet
vet:
	@echo "Vetting code..."
	@go vet ./...

# Run tests
.PHONY: test
test: fmt vet
	@echo "Running tests..."
	@go test -v ./...

# Run tests with race detection
.PHONY: test-race
test-race:
	@echo "Running tests with race detection..."
	@go test -race ./...

# Run benchmarks
.PHONY: bench
bench:
	@echo "Running benchmarks..."
	@go test -bench=. ./...

# Build for current platform (development)
.PHONY: build
build: test
	@echo "Building for current platform..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) .

# Build for all platforms
.PHONY: build-all
build-all: build-linux build-windows build-darwin

# Build for Linux (amd64)
.PHONY: build-linux
build-linux:
	@echo "Building for Linux (amd64)..."
	@mkdir -p $(DIST_DIR)/linux-amd64
	@GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/linux-amd64/$(BINARY_NAME) .

# Build for Linux (arm64)
.PHONY: build-linux-arm64
build-linux-arm64:
	@echo "Building for Linux (arm64)..."
	@mkdir -p $(DIST_DIR)/linux-arm64
	@GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/linux-arm64/$(BINARY_NAME) .

# Build for Windows (amd64)
.PHONY: build-windows
build-windows:
	@echo "Building for Windows (amd64)..."
	@mkdir -p $(DIST_DIR)/windows-amd64
	@GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/windows-amd64/$(BINARY_NAME).exe .

# Build for macOS (amd64 - Intel)
.PHONY: build-darwin
build-darwin:
	@echo "Building for macOS (amd64 - Intel)..."
	@mkdir -p $(DIST_DIR)/darwin-amd64
	@GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/darwin-amd64/$(BINARY_NAME) .

# Build for macOS (arm64 - Apple Silicon)
.PHONY: build-darwin-arm64
build-darwin-arm64:
	@echo "Building for macOS (arm64 - Apple Silicon)..."
	@mkdir -p $(DIST_DIR)/darwin-arm64
	@GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/darwin-arm64/$(BINARY_NAME) .

# Build for all macOS architectures
.PHONY: build-darwin-all
build-darwin-all: build-darwin build-darwin-arm64

# Build universal macOS binary (requires both amd64 and arm64 builds)
.PHONY: build-darwin-universal
build-darwin-universal: build-darwin build-darwin-arm64
	@echo "Creating universal macOS binary..."
	@mkdir -p $(DIST_DIR)/darwin-universal
	@lipo -create -output $(DIST_DIR)/darwin-universal/$(BINARY_NAME) \
		$(DIST_DIR)/darwin-amd64/$(BINARY_NAME) \
		$(DIST_DIR)/darwin-arm64/$(BINARY_NAME)

# Create distribution packages
.PHONY: package
package: build-all
	@echo "Creating distribution packages..."
	@mkdir -p $(DIST_DIR)/packages
	
	# Linux amd64
	@cd $(DIST_DIR)/linux-amd64 && tar -czf ../packages/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)
	
	# Windows amd64
	@cd $(DIST_DIR)/windows-amd64 && zip -q ../packages/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME).exe
	
	# macOS amd64
	@cd $(DIST_DIR)/darwin-amd64 && tar -czf ../packages/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)
	
	@echo "Distribution packages created in $(DIST_DIR)/packages/"

# Run the development server
.PHONY: run
run: build
	@echo "Starting development server..."
	@./$(BUILD_DIR)/$(BINARY_NAME)

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Update dependencies
.PHONY: update-deps
update-deps:
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

# Security audit
.PHONY: audit
audit:
	@echo "Running security audit..."
	@go mod verify
	@which gosec > /dev/null && gosec ./... || echo "gosec not installed, skipping security scan"

# Static analysis
.PHONY: lint
lint:
	@echo "Running static analysis..."
	@which staticcheck > /dev/null && staticcheck ./... || echo "staticcheck not installed, skipping lint"

# Development workflow - run tests and build
.PHONY: dev
dev: clean test build

# Production build with optimizations
.PHONY: prod
prod: clean test lint audit build-all package

# Show build info
.PHONY: info
info:
	@echo "Binary Name: $(BINARY_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Go Version: $(shell go version)"

# Show available targets
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          - Clean, test, and build for all platforms"
	@echo "  build        - Build for current platform"
	@echo "  build-all    - Build for all platforms (Linux, Windows, macOS)"
	@echo "  build-linux  - Build for Linux (amd64)"
	@echo "  build-windows- Build for Windows (amd64)"
	@echo "  build-darwin - Build for macOS (amd64)"
	@echo "  build-darwin-arm64 - Build for macOS (arm64)"
	@echo "  build-darwin-universal - Create universal macOS binary"
	@echo "  package      - Create distribution packages"
	@echo "  test         - Run tests with formatting and vetting"
	@echo "  test-race    - Run tests with race detection"
	@echo "  bench        - Run benchmarks"
	@echo "  run          - Build and run development server"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Install dependencies"
	@echo "  update-deps  - Update dependencies"
	@echo "  audit        - Run security audit"
	@echo "  lint         - Run static analysis"
	@echo "  dev          - Development workflow (clean, test, build)"
	@echo "  prod         - Production workflow (clean, test, lint, audit, build-all, package)"
	@echo "  info         - Show build information"
	@echo "  help         - Show this help message"