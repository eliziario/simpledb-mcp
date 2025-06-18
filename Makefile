# SimpleDB MCP Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
BINARY_NAME=simpledb-mcp
CLI_BINARY_NAME=simpledb-cli
PROXY_BINARY_NAME=simpledb-mcp-proxy
BINARY_DIR=bin

# Version
VERSION ?= $(shell cat VERSION | tr -d '\n')
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Platforms
PLATFORMS=darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64

.PHONY: all build build-cli build-proxy clean test deps tidy install-deps help

# Default target
all: clean deps build build-cli build-proxy

# Build main server
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

# Build CLI tool
build-cli:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(CLI_BINARY_NAME) ./cmd/$(CLI_BINARY_NAME)

# Build proxy tool
build-proxy:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(PROXY_BINARY_NAME) ./cmd/$(PROXY_BINARY_NAME)

# Build for current platform
build-local: build build-cli build-proxy

# Cross-compile for all platforms
build-all: clean
	@echo "Building for all platforms..."
	@for platform in $(PLATFORMS); do \
		OS=$$(echo $$platform | cut -d'/' -f1); \
		ARCH=$$(echo $$platform | cut -d'/' -f2); \
		EXT=""; \
		if [ "$$OS" = "windows" ]; then EXT=".exe"; fi; \
		echo "Building for $$OS/$$ARCH..."; \
		GOOS=$$OS GOARCH=$$ARCH $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-$$OS-$$ARCH$$EXT ./cmd/$(BINARY_NAME); \
		GOOS=$$OS GOARCH=$$ARCH $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(CLI_BINARY_NAME)-$$OS-$$ARCH$$EXT ./cmd/$(CLI_BINARY_NAME); \
		GOOS=$$OS GOARCH=$$ARCH $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(PROXY_BINARY_NAME)-$$OS-$$ARCH$$EXT ./cmd/$(PROXY_BINARY_NAME); \
	done

# Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Update dependencies
update-deps:
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run tests for specific package
test-config:
	$(GOTEST) -v ./internal/config

test-credentials:
	$(GOTEST) -v ./internal/credentials

test-database:
	$(GOTEST) -v ./internal/database

test-tools:
	$(GOTEST) -v ./internal/tools

test-integration:
	$(GOTEST) -v ./internal/integration

# Run all unit tests (excluding integration)
test-unit:
	$(GOTEST) -v ./internal/config ./internal/credentials ./internal/database ./internal/tools ./internal/testutil

# Run tests with race detection
test-race:
	$(GOTEST) -v -race ./...

# Run benchmarks
test-bench:
	$(GOTEST) -v -bench=. ./...

# Quick test (no verbose output)
test-quick:
	$(GOTEST) ./...

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

# Prepare binary directory
$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

# Install on macOS
install-macos: build-local
	@echo "Installing SimpleDB MCP on macOS..."
	./scripts/install-macos.sh

# Uninstall on macOS
uninstall-macos:
	@echo "Uninstalling SimpleDB MCP from macOS..."
	./scripts/uninstall-macos.sh

# Install on Windows (run in PowerShell as Administrator)
install-windows: 
	@echo "To install on Windows, run in PowerShell as Administrator:"
	@echo "  .\scripts\install-windows.ps1"

# Development server (runs without installing)
dev-server: build
	./$(BINARY_DIR)/$(BINARY_NAME)

# Development CLI (runs without installing)
dev-cli: build-cli
	./$(BINARY_DIR)/$(CLI_BINARY_NAME)

# Format code
fmt:
	$(GOCMD) fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run ./...

# Security scan (requires gosec)
security:
	gosec ./...

# Create release package
package: build-all
	@echo "Creating release packages..."
	@mkdir -p releases
	@for platform in $(PLATFORMS); do \
		OS=$$(echo $$platform | cut -d'/' -f1); \
		ARCH=$$(echo $$platform | cut -d'/' -f2); \
		EXT=""; \
		if [ "$$OS" = "windows" ]; then EXT=".exe"; fi; \
		PKG_NAME="simpledb-mcp-$(VERSION)-$$OS-$$ARCH"; \
		echo "Packaging $$PKG_NAME..."; \
		mkdir -p releases/$$PKG_NAME; \
		cp $(BINARY_DIR)/$(BINARY_NAME)-$$OS-$$ARCH$$EXT releases/$$PKG_NAME/$(BINARY_NAME)$$EXT; \
		cp $(BINARY_DIR)/$(CLI_BINARY_NAME)-$$OS-$$ARCH$$EXT releases/$$PKG_NAME/$(CLI_BINARY_NAME)$$EXT; \
		cp $(BINARY_DIR)/$(PROXY_BINARY_NAME)-$$OS-$$ARCH$$EXT releases/$$PKG_NAME/$(PROXY_BINARY_NAME)$$EXT; \
		cp README.md releases/$$PKG_NAME/; \
		cp -r configs releases/$$PKG_NAME/; \
		cp -r scripts releases/$$PKG_NAME/; \
		if [ "$$OS" = "windows" ]; then \
			cd releases && zip -r $$PKG_NAME.zip $$PKG_NAME && cd ..; \
		else \
			cd releases && tar -czf $$PKG_NAME.tar.gz $$PKG_NAME && cd ..; \
		fi; \
		rm -rf releases/$$PKG_NAME; \
	done

# Docker build (optional)
docker-build:
	docker build -t simpledb-mcp:$(VERSION) .
	docker build -t simpledb-mcp:latest .

# Help
help:
	@echo "SimpleDB MCP Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  build          Build main server binary"
	@echo "  build-cli      Build CLI tool binary"
	@echo "  build-proxy    Build stdio-to-HTTP proxy for Claude compatibility"
	@echo "  build-local    Build all binaries for current platform"
	@echo "  build-all      Cross-compile for all platforms"
	@echo "  clean          Clean build artifacts"
	@echo "  deps           Install/update dependencies"
	@echo "  test           Run all tests"
	@echo "  test-unit      Run unit tests only"
	@echo "  test-integration Run integration tests"
	@echo "  test-coverage  Run tests with coverage report"
	@echo "  test-race      Run tests with race detection"
	@echo "  test-quick     Run tests without verbose output"
	@echo "  test-config    Run config package tests"
	@echo "  test-credentials Run credentials package tests"
	@echo "  test-database  Run database package tests"
	@echo "  test-tools     Run tools package tests"
	@echo "  fmt            Format code"
	@echo "  lint           Lint code (requires golangci-lint)"
	@echo "  security       Security scan (requires gosec)"
	@echo ""
	@echo "Installation:"
	@echo "  install-macos    Install on macOS using launchd"
	@echo "  uninstall-macos  Uninstall from macOS"
	@echo "  install-windows  Show Windows installation instructions"
	@echo ""
	@echo "Development:"
	@echo "  dev-server     Run server without installing"
	@echo "  dev-cli        Run CLI without installing"
	@echo ""
	@echo "Release:"
	@echo "  package        Create release packages for all platforms"
	@echo "  docker-build   Build Docker images"
	@echo ""
	@echo "Examples:"
	@echo "  make build-local     # Build for current platform"
	@echo "  make test            # Run tests"
	@echo "  make install-macos   # Install on macOS"
	@echo "  make package         # Create release packages"