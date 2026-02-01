.PHONY: all build test test-unit test-integration test-e2e test-all clean install

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Default target
all: build

# Build the binary
build:
	go build $(LDFLAGS) -o dnstm ./main.go

# Install the binary
install: build
	sudo cp dnstm /usr/local/bin/dnstm

# Run all tests (mocked systemd)
test: test-unit test-integration

# Unit tests - fast, no external dependencies
test-unit:
	go test -v ./internal/config/...
	go test -v ./internal/keys/...
	go test -v ./internal/certs/...
	go test -v ./internal/router/...
	go test -v ./internal/service/...

# Integration tests - uses mock systemd
test-integration:
	go test -v ./tests/integration/...

# E2E tests - auto-downloads binaries on first run
test-e2e:
	go test -v -timeout 5m ./tests/e2e/...

# All tests with real systemd (requires root)
test-real:
	sudo DNSTM_TEST_REAL_SYSTEMD=1 go test -v ./...

# Full test suite
test-all: test-unit test-integration test-e2e

# Coverage report
test-coverage:
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Download test binaries
test-setup:
	@echo "Downloading test binaries..."
	@mkdir -p tests/.testbin
	@echo "Note: Manual download required - see docs/TESTING.md"

# Clean build artifacts
clean:
	rm -f dnstm
	rm -f coverage.out coverage.html
	rm -rf tests/.testbin tests/.testconfig

# Run linter
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...
	goimports -w .

# Check for issues
check: fmt lint test-unit

# Development helpers
dev-build: build
	@echo "Built dnstm version $(VERSION)"

# Run the application
run: build
	sudo ./dnstm
