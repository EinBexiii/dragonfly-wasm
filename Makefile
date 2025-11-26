# Makefile for Dragonfly WASM Plugin System
.PHONY: all build proto test clean fmt lint examples run

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=gofmt
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Protobuf
PROTOC=protoc
PROTO_DIR=api/proto
PROTO_OUT=api/proto

# Binary
BINARY_NAME=dragonfly-plugins
BINARY_DIR=bin

# Default target
all: proto build

# Build the main binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/server

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@mkdir -p $(PROTO_OUT)/eventspb
	@mkdir -p $(PROTO_OUT)/pluginpb
	$(PROTOC) --go_out=$(PROTO_OUT)/eventspb --go_opt=paths=source_relative \
		$(PROTO_DIR)/events.proto
	$(PROTOC) --go_out=$(PROTO_OUT)/pluginpb --go_opt=paths=source_relative \
		$(PROTO_DIR)/plugin.proto

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Lint code
lint:
	@echo "Linting code..."
	$(GOLINT) run ./...

# Build example plugins
examples:
	@echo "Building example plugins..."
	@cd examples/hello-world && ./build.sh

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BINARY_DIR)
	@rm -rf coverage.out coverage.html
	@rm -rf $(PROTO_OUT)/eventspb/*.pb.go
	@rm -rf $(PROTO_OUT)/pluginpb/*.pb.go
	@rm -rf plugin_cache plugin_data

# Run the server
run: build
	@echo "Starting server..."
	./$(BINARY_DIR)/$(BINARY_NAME)

# Run with debug logging
run-debug: build
	@echo "Starting server with debug logging..."
	DEBUG=1 ./$(BINARY_DIR)/$(BINARY_NAME)

# Install development tools
tools:
	@echo "Installing development tools..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/golangci-lint/golangci-lint/cmd/golangci-lint@latest

# Create plugin directory structure
init-plugins:
	@echo "Creating plugin directories..."
	@mkdir -p plugins
	@mkdir -p plugin_data
	@mkdir -p plugin_cache

# Development setup
dev: deps tools init-plugins proto
	@echo "Development environment ready!"

# Help
help:
	@echo "Dragonfly WASM Plugin System"
	@echo ""
	@echo "Usage:"
	@echo "  make              - Build everything"
	@echo "  make build        - Build the server binary"
	@echo "  make proto        - Generate protobuf code"
	@echo "  make test         - Run tests"
	@echo "  make test-coverage - Run tests with coverage"
	@echo "  make fmt          - Format code"
	@echo "  make lint         - Lint code"
	@echo "  make examples     - Build example plugins"
	@echo "  make deps         - Download dependencies"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make run          - Run the server"
	@echo "  make run-debug    - Run with debug logging"
	@echo "  make tools        - Install dev tools"
	@echo "  make dev          - Setup development environment"
	@echo "  make help         - Show this help"
