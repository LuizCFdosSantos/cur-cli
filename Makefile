# Binary name
BINARY_NAME=cur-cli

# Go files
PKG=./...

# Output
COVERAGE_FILE=coverage.out

# Default target
.PHONY: all
all: fmt vet lint test build

# ----------------------
# Formatting
# ----------------------
.PHONY: fmt
fmt:
	go fmt $(PKG)

# ----------------------
# Vet (static analysis)
# ----------------------
.PHONY: vet
vet:
	go vet $(PKG)

# ----------------------
# Lint (requires golangci-lint)
# ----------------------
.PHONY: lint
lint:
	golangci-lint run

# ----------------------
# Build
# ----------------------
.PHONY: build
build:
	go build -o bin/$(BINARY_NAME) .

# ----------------------
# Run (optional helper)
# ----------------------
.PHONY: run
run:
	go run .

# ----------------------
# Test
# ----------------------
.PHONY: test
test:
	go test $(PKG) -coverprofile=$(COVERAGE_FILE) -coverpkg=$(PKG)

# ----------------------
# Coverage (CLI output)
# ----------------------
.PHONY: coverage
coverage: test
	go tool cover -func=$(COVERAGE_FILE)

# ----------------------
# Coverage (HTML)
# ----------------------
.PHONY: coverage-html
coverage-html: test
	go tool cover -html=$(COVERAGE_FILE)

# ----------------------
# Clean
# ----------------------
.PHONY: clean
clean:
	rm -rf bin $(COVERAGE_FILE)

# ----------------------
# Tidy dependencies
# ----------------------
.PHONY: tidy
tidy:
	go mod tidy

# ----------------------
# Download dependencies
# ----------------------
.PHONY: deps
deps:
	go mod download
	

