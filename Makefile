# Project variables
BINARY_NAME=lea
VERSION=0.2.1
BUILD_DIR=bin
MAIN_PATH=./cmd/lea/main.go

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

.PHONY: all build clean test lint run help install

all: build test

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

clean: ## Remove build artifacts and local graph data
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf .lea/

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

run: build ## Build and run the binary
	./$(BUILD_DIR)/$(BINARY_NAME)

install: build ## Install the binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	cp $(BUILD_DIR)/$(BINARY_NAME) $(shell go env GOPATH)/bin/

index: build ## Index the current directory
	./$(BUILD_DIR)/$(BINARY_NAME) index .

tui: build ## Start the interactive TUI
	./$(BUILD_DIR)/$(BINARY_NAME) tui

mcp: build ## Start the MCP server
	./$(BUILD_DIR)/$(BINARY_NAME) mcp

watch: build ## Start the file watcher
	./$(BUILD_DIR)/$(BINARY_NAME) watch .

tidy: ## Tidy up go.mod and go.sum
	$(GOMOD) tidy
