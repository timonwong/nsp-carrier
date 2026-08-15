SHELL := /bin/sh

GO ?= go
BIN_DIR ?= bin
FUZZ_TIME ?= 2s
USB_SPIKE := $(BIN_DIR)/usb-spike

.DEFAULT_GOAL := help

.PHONY: help deps fmt fmt-check test test-race fuzz vet build check usb-spike clean

help: ## Show available targets.
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Download Go module dependencies.
	$(GO) mod download

fmt: ## Format Go source files.
	gofmt -w cmd internal

fmt-check: ## Fail if Go source files need formatting.
	@test -z "$$(gofmt -l cmd internal)" || { \
		echo "Go files need formatting:"; \
		gofmt -l cmd internal; \
		exit 1; \
	}

test: ## Run unit tests without cache.
	$(GO) test -count=1 ./...

test-race: ## Run tests with the race detector.
	$(GO) test -race -count=1 ./...

fuzz: ## Run short DBI codec fuzz smoke tests.
	$(GO) test ./internal/dbi -run='^$$' -fuzz=FuzzDecodeHeader -fuzztime=$(FUZZ_TIME)
	$(GO) test ./internal/dbi -run='^$$' -fuzz=FuzzParseRangeRequest -fuzztime=$(FUZZ_TIME)

vet: ## Run Go static analysis.
	$(GO) vet ./...

build: ## Build the macOS USB diagnostics CLI.
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -o $(USB_SPIKE) ./cmd/usb-spike

check: fmt-check test test-race fuzz vet build ## Run the complete automated Gate 0 prerequisite.

usb-spike: build ## Run the USB spike; pass arguments with ARGS='...'.
	./$(USB_SPIKE) $(ARGS)

clean: ## Remove locally built binaries.
	@rm -f $(USB_SPIKE)
	@rmdir $(BIN_DIR) 2>/dev/null || true
