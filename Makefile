SHELL := /bin/sh

GO ?= go
PNPM ?= pnpm
BIN_DIR ?= bin
FUZZ_TIME ?= 2s
USB_SPIKE := $(BIN_DIR)/usb-spike
WAILS ?= $(shell command -v wails3 2>/dev/null || { d="$$(go env GOBIN)"; [ -n "$$d" ] || d="$$(go env GOPATH)/bin"; printf '%s/wails3' "$$d"; })
WAILS_VERSION ?= v3.0.0-beta.28

.DEFAULT_GOAL := help

.PHONY: help deps fmt fmt-check test test-race fuzz vet build check gate0-probe usb-spike wails-install wails-doctor wails-bindings wails-bindings-check ui-install ui-format ui-format-check ui-test ui-check ui-build app-build app-dev clean

help: ## Show available targets.
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Download Go module dependencies.
	$(GO) mod download

fmt: ## Format Go source files.
	gofmt -w *.go cmd internal

fmt-check: ## Fail if Go source files need formatting.
	@test -z "$$(gofmt -l *.go cmd internal)" || { \
		echo "Go files need formatting:"; \
		gofmt -l *.go cmd internal; \
		exit 1; \
	}

test: ## Run unit tests without cache.
	$(GO) test -count=1 ./...

test-race: ## Run tests with the race detector.
	$(GO) test -race -count=1 ./...

fuzz: ## Run short protocol codec fuzz smoke tests.
	$(GO) test ./internal/dbi -run='^$$' -fuzz=FuzzDecodeHeader -fuzztime=$(FUZZ_TIME)
	$(GO) test ./internal/dbi -run='^$$' -fuzz=FuzzParseRangeRequest -fuzztime=$(FUZZ_TIME)
	$(GO) test ./internal/awoo -run='^$$' -fuzz=FuzzDecodeCommandHeader -fuzztime=$(FUZZ_TIME)
	$(GO) test ./internal/awoo -run='^$$' -fuzz=FuzzParseRangeRequest -fuzztime=$(FUZZ_TIME)
	$(GO) test ./internal/goldleaf -run='^$$' -fuzz=FuzzDecodeRequest -fuzztime=$(FUZZ_TIME)
	$(GO) test ./internal/sphaira -run='^$$' -fuzz=FuzzDecodePacket -fuzztime=$(FUZZ_TIME)

vet: ## Run Go static analysis.
	$(GO) vet ./...

build: ## Build the macOS USB diagnostics CLI.
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -o $(USB_SPIKE) ./cmd/usb-spike

check: fmt-check test test-race fuzz vet build ui-check ui-build ## Run all Go and frontend validation.

gate0-probe: ## Build first, then wait for and claim DBI USB endpoints.
	./scripts/gate0-probe.sh

usb-spike: build ## Run the USB spike; pass arguments with ARGS='...'.
	./$(USB_SPIKE) $(ARGS)

wails-install: ## Install the pinned Wails v3 CLI.
	$(GO) install github.com/wailsapp/wails/v3/cmd/wails3@$(WAILS_VERSION)

wails-doctor: ## Check local Wails build dependencies.
	$(WAILS) doctor

wails-bindings: ## Generate canonical Wails bindings.
	WAILS_BIN="$(WAILS)" $(PNPM) --dir frontend run bindings:generate

wails-bindings-check: ## Regenerate canonical bindings and fail when tracked output is stale.
	WAILS_BIN="$(WAILS)" $(PNPM) --dir frontend run bindings:check

ui-install: ## Install locked frontend dependencies.
	$(PNPM) --dir frontend install --frozen-lockfile

ui-format: ## Format frontend sources and tracked generated files.
	$(PNPM) --dir frontend run format

ui-format-check: ## Fail if frontend files need canonical formatting.
	$(PNPM) --dir frontend run format:check

ui-test: ## Run frontend behavior tests.
	$(PNPM) --dir frontend run test

ui-check: ui-test ui-format-check ## Run frontend tests, formatting, Svelte, and TypeScript diagnostics.
	$(PNPM) --dir frontend run check

ui-build: ## Build static frontend assets.
	$(PNPM) --dir frontend run build

app-build: wails-bindings ## Build the macOS arm64 Wails application bundle.
	PATH="$(dir $(WAILS)):$$PATH" $(WAILS) task package ARCH=arm64

app-dev: wails-bindings ## Run the Wails application with frontend hot reload.
	PATH="$(dir $(WAILS)):$$PATH" $(WAILS) task dev

clean: ## Remove locally built binaries.
	@rm -f $(USB_SPIKE)
	@rmdir $(BIN_DIR) 2>/dev/null || true
