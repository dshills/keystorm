# Keystorm Makefile
# AI-native programming editor

.PHONY: all build test lint fmt vet clean install run help coverage bench check

# Binary name and paths
BINARY_NAME := keystorm
CMD_PATH := ./cmd/keystorm
BIN_DIR := ./bin
COVERAGE_DIR := ./coverage

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet
GOMOD := $(GOCMD) mod
GORUN := $(GOCMD) run

# Build flags
LDFLAGS := -s -w
BUILD_FLAGS := -ldflags "$(LDFLAGS)"

# Default target
all: check build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(BUILD_FLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)

# Build with debug symbols
build-debug:
	@echo "Building $(BINARY_NAME) with debug symbols..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	$(GOTEST) -race -v ./...

# Run short tests only
test-short:
	@echo "Running short tests..."
	$(GOTEST) -short -v ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report: $(COVERAGE_DIR)/coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

# Run golangci-lint
lint:
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Run all checks (fmt, vet, lint)
check: fmt vet lint

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR)
	@rm -rf $(COVERAGE_DIR)
	$(GOCMD) clean -cache -testcache

# Install binary to GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(CMD_PATH)

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	$(GORUN) $(CMD_PATH)

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

# Verify dependencies
verify:
	@echo "Verifying dependencies..."
	$(GOMOD) verify

# Update dependencies
update:
	@echo "Updating dependencies..."
	$(GOMOD) tidy
	$(GOCMD) get -u ./...
	$(GOMOD) tidy

# Run specific package tests
test-pkg:
	@echo "Running tests for $(PKG)..."
	$(GOTEST) -v $(PKG)

# Generate (if you have go generate directives)
generate:
	@echo "Running go generate..."
	$(GOCMD) generate ./...

# Show help
help:
	@echo "Keystorm Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all          Run checks and build (default)"
	@echo "  build        Build the binary"
	@echo "  build-debug  Build with debug symbols"
	@echo "  test         Run all tests"
	@echo "  test-race    Run tests with race detection"
	@echo "  test-short   Run short tests only"
	@echo "  coverage     Run tests with coverage report"
	@echo "  bench        Run benchmarks"
	@echo "  fmt          Format code"
	@echo "  vet          Run go vet"
	@echo "  lint         Run golangci-lint"
	@echo "  check        Run fmt, vet, and lint"
	@echo "  clean        Remove build artifacts"
	@echo "  install      Install binary to GOPATH/bin"
	@echo "  run          Run the application"
	@echo "  tidy         Tidy dependencies"
	@echo "  deps         Download dependencies"
	@echo "  verify       Verify dependencies"
	@echo "  update       Update dependencies"
	@echo "  test-pkg     Run tests for specific package (PKG=./path/to/pkg)"
	@echo "  generate     Run go generate"
	@echo "  help         Show this help"
