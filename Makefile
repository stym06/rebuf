# Makefile

# Define environment variable
export TEST_LOG_DIR := "something"

# Define the output binary directory and name
BINARY_DIR := bin
BINARY_NAME := rebuf

# Define Go build command
BUILD_CMD := go build -o $(BINARY_DIR)/$(BINARY_NAME)

# Define Go test command
TEST_CMD := go test -cover -coverprofile=coverage.txt ./...

.PHONY: all build test

# Default target
all: build test

# Build target
build:
	@echo "Building the binary..."
	$(BUILD_CMD)
	@echo "Build complete. Binary is located at $(BINARY_DIR)/$(BINARY_NAME)"

# Test target
test:
	@echo "Running tests..."
	$(TEST_CMD)
	@echo "Tests complete. Coverage report is available in coverage.txt"
