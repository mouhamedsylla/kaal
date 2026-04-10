## pilot — Makefile
## Usage: make <target>
##
##   make build        Build the binary for the current platform (./pilot)
##   make install      Build + install to $GOPATH/bin (makes 'pilot' available globally)
##   make dev          Install without version stamp — fast iteration loop
##   make release      Tag + push a new version (usage: make release VERSION=v0.2.0)
##   make test         Run all tests
##   make lint         Run golangci-lint
##   make clean        Remove built binary
##   make version      Show current version info

# ──────────────────────────── variables ────────────────────────────────────

MODULE     := github.com/mouhamedsylla/pilot
BINARY     := pilot
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X $(MODULE)/internal/version.Version=$(VERSION) \
           -X $(MODULE)/internal/version.Commit=$(COMMIT) \
           -X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

# Installation directory — picks the first writable directory in this order:
#   $GOBIN → $GOPATH/bin → ~/go/bin → ~/.local/bin → /usr/local/bin (sudo)
GOBIN_DIR   := $(shell go env GOBIN 2>/dev/null)
GOPATH_DIR  := $(shell go env GOPATH 2>/dev/null)
INSTALL_DIR := $(shell \
  if [ -n "$(GOBIN_DIR)" ]; then echo "$(GOBIN_DIR)"; \
  elif [ -n "$(GOPATH_DIR)" ]; then echo "$(GOPATH_DIR)/bin"; \
  elif [ -d "$(HOME)/go/bin" ]; then echo "$(HOME)/go/bin"; \
  elif [ -d "$(HOME)/.local/bin" ]; then echo "$(HOME)/.local/bin"; \
  else echo "/usr/local/bin"; fi)

# ──────────────────────────── targets ──────────────────────────────────────

.PHONY: build install dev release test lint clean version help

## build: compile for the current platform → ./pilot
build:
	@echo "→ Building $(BINARY) $(VERSION)"
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .
	@echo "✓ $(BINARY) built ($(shell du -sh $(BINARY) | cut -f1))"

## install: build + copy to $(INSTALL_DIR) (makes 'pilot' available in PATH)
install: build
	@echo "→ Installing to $(INSTALL_DIR)/$(BINARY)"
	@mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "✓ Installed: $(shell which $(BINARY) 2>/dev/null || echo $(INSTALL_DIR)/$(BINARY))"
	@echo "  Version: $(VERSION) ($(COMMIT))"

## dev: fast install without version stamp — for tight iteration loops
dev:
	@echo "→ Installing dev build (no version stamp)"
	go install .
	@echo "✓ Done — 'pilot version' will show 'dev'"

## release: tag a new version and push to GitHub
##   Usage: make release VERSION=v0.2.0
release:
	@if [ -z "$(VERSION)" ] || [ "$(VERSION)" = "dev" ]; then \
		echo "✗ Usage: make release VERSION=v0.2.0"; exit 1; \
	fi
	@if ! echo "$(VERSION)" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+'; then \
		echo "✗ VERSION must follow semver: v0.2.0"; exit 1; \
	fi
	@echo "→ Releasing $(VERSION)"
	@# Ensure clean working tree
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "✗ Working tree is dirty — commit or stash changes first"; exit 1; \
	fi
	@# Run tests before tagging
	$(MAKE) test
	@# Tag and push
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)
	@echo "✓ Tagged and pushed $(VERSION)"
	@echo "  GitHub Actions will build and publish the release binaries."
	@echo "  Users can then run: pilot update"

## test: run all tests
test:
	@echo "→ Running tests"
	go test ./... -v -count=1

## lint: run golangci-lint (install: brew install golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "✗ golangci-lint not found — brew install golangci-lint"; exit 1)
	golangci-lint run ./...

## clean: remove built binary
clean:
	@rm -f $(BINARY)
	@echo "✓ Cleaned"

## version: show version that would be stamped on next build
version:
	@echo "Version   : $(VERSION)"
	@echo "Commit    : $(COMMIT)"
	@echo "BuildDate : $(BUILD_DATE)"
	@echo "InstallDir: $(INSTALL_DIR)"

## help: list available targets
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
