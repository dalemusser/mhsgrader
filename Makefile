# MHSGrader Makefile

# Binary name
BINARY=mhsgrader

# Build directory
BUILD_DIR=bin

# Go build flags
LDFLAGS=-ldflags "-s -w"

.PHONY: all build build-linux run clean tidy test help

all: build

## Build targets

build: ## Build the binary for current platform
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/mhsgrader

build-linux: ## Build the binary for Linux (production)
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/mhsgrader

## Run targets

run: ## Run the grader locally
	go run ./cmd/mhsgrader

run-reset: ## Run the grader with --reset to clear all state and grades
	go run ./cmd/mhsgrader --reset

## Dependency management

tidy: ## Tidy go.mod dependencies
	go mod tidy

## Testing

test: ## Run tests
	go test -v ./...

## Deployment

deploy: build-linux ## Build and deploy to EC2 (configure SSH_HOST)
	@if [ -z "$(SSH_HOST)" ]; then \
		echo "Error: SSH_HOST not set. Usage: make deploy SSH_HOST=user@host"; \
		exit 1; \
	fi
	scp $(BUILD_DIR)/$(BINARY)-linux-amd64 $(SSH_HOST):/opt/mhsgrader/$(BINARY)
	ssh $(SSH_HOST) "sudo systemctl restart mhsgrader"

## Utilities

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

fmt: ## Format code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

## Help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
