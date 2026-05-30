# Neocron launcher — convenience wrapper around the Wails + Go +
# Vite toolchain. Run `make help` to see what's available.
#
# Variables you can override:
#   WAILS    path to wails binary  (auto-detected; override if needed)
#   BIN      output binary name    (default: launcher)

# Auto-detect Wails: prefer $PATH, fall back to $GOBIN, then $GOPATH/bin,
# then ~/go/bin. Users who installed via `go install` typically have it
# in ~/go/bin which isn't always on $PATH for non-interactive shells.
GOBIN_FALLBACK := $(or $(shell go env GOBIN 2>/dev/null),$(shell go env GOPATH 2>/dev/null)/bin,$(HOME)/go/bin)
WAILS ?= $(shell command -v wails 2>/dev/null || ls $(GOBIN_FALLBACK)/wails 2>/dev/null || echo wails)

BIN   ?= launcher
BUILD_DIR := build/bin
FRONTEND  := frontend

.DEFAULT_GOAL := help

# ── help ────────────────────────────────────────────────────────────
# Self-documenting: any target of the form `name: ... ## description`
# gets listed. The first `## ` on the line is the separator; anything
# after is the description. Uses sed (not awk FS) for portability —
# different awk variants disagreed about greedy non-`#` matching.
.PHONY: help
help: ## Show this help
	@printf "Usage: make \033[36m<target>\033[0m\n\nTargets:\n"
	@sed -n 's/^\([a-zA-Z0-9_-]\{1,\}\):[^#]*## \(.*\)$$/  \1\t\2/p' \
		$(MAKEFILE_LIST) | \
		awk -F '\t' '{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ── setup ───────────────────────────────────────────────────────────
.PHONY: install
install: install-go install-frontend ## Install all dependencies (Go + npm)

.PHONY: install-go
install-go: ## go mod download + tidy
	go mod download
	go mod tidy

.PHONY: install-frontend
install-frontend: ## npm install in frontend/
	cd $(FRONTEND) && npm install

.PHONY: install-wails
install-wails: ## Install/update the Wails CLI to latest v2
	go install github.com/wailsapp/wails/v2/cmd/wails@latest

# ── build ───────────────────────────────────────────────────────────
# Preflight: bail with a clear instruction if Wails isn't installed
# anywhere we know to look. `command -v` returns 0 if WAILS is on
# PATH (e.g. `wails`), and the ls-fallback already absolute-pathed
# the binary if it's only in ~/go/bin.
.PHONY: wails-check
wails-check:
	@if ! { command -v $(WAILS) >/dev/null 2>&1 || [ -x "$(WAILS)" ]; }; then \
		echo "ERROR: wails binary not found."; \
		echo "Looked at: \$$PATH, $(GOBIN_FALLBACK)/wails"; \
		echo "Install with: make install-wails"; \
		echo "Or ensure your Go bin dir is on \$$PATH:"; \
		echo "  export PATH=\"\$$PATH:\$$(go env GOPATH)/bin\""; \
		exit 1; \
	fi

.PHONY: build
build: wails-check ## Build release binary into build/bin/
	$(WAILS) build

.PHONY: build-debug
build-debug: wails-check ## Build with debug symbols + console
	$(WAILS) build -debug

.PHONY: build-windows
build-windows: wails-check ## Cross-build for Windows amd64
	$(WAILS) build -platform windows/amd64

.PHONY: build-mac
build-mac: wails-check ## Cross-build for macOS (universal)
	$(WAILS) build -platform darwin/universal

.PHONY: build-all
build-all: build build-windows build-mac ## Build for all three desktop platforms

# ── dev / run ───────────────────────────────────────────────────────
.PHONY: dev
dev: wails-check ## Wails dev server with hot reload
	$(WAILS) dev

.PHONY: run
run: ## Run the most recently built binary
	./$(BUILD_DIR)/$(BIN)

.PHONY: frontend-dev
frontend-dev: ## Run frontend vite dev server alone (no Go backend)
	cd $(FRONTEND) && npm run dev

# ── test / verify ───────────────────────────────────────────────────
.PHONY: test
test: ## Run Go tests (all packages, -race)
	go test -race ./...

.PHONY: test-short
test-short: ## Run Go tests, skipping long-running ones
	go test -short ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report -> coverage.html
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: $(PWD)/coverage.html"

.PHONY: vet
vet: ## go vet (static analysis)
	go vet ./...

.PHONY: fmt
fmt: ## gofmt -s -w all .go files
	gofmt -s -w .

.PHONY: fmt-check
fmt-check: ## Fail if any .go files would be reformatted
	@diff=$$(gofmt -s -d .); \
	if [ -n "$$diff" ]; then \
		echo "$$diff"; exit 1; \
	fi

.PHONY: lint
lint: vet fmt-check ## vet + fmt-check (CI-friendly)

# ── screenshots / docs ──────────────────────────────────────────────
.PHONY: screenshots
screenshots: ## Regenerate Playwright UI screenshots (needs `make dev` running)
	cd $(FRONTEND) && npm run screenshots

# ── clean ───────────────────────────────────────────────────────────
.PHONY: clean
clean: ## Remove build artifacts (binaries + frontend dist + coverage)
	rm -rf $(BUILD_DIR)
	rm -rf $(FRONTEND)/dist
	rm -f coverage.out coverage.html

.PHONY: clean-all
clean-all: clean ## clean + nuke node_modules
	rm -rf $(FRONTEND)/node_modules
