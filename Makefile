# Makefile for github.com/otfabric/go-serial

SHELL := /bin/bash

GO ?= go
PKGS := ./...
COVER_PKGS := $(shell $(GO) list ./... | grep -v '/example$$')
COVER_MIN := 90
# ci.yml cross-build matrix, plus netbsd and openbsd, which have backends here.
CROSS_PLATFORMS := linux/amd64 linux/arm linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64 freebsd/arm64 netbsd/amd64 openbsd/amd64

.PHONY: help test test-race vet lint vuln fmt bench coverage coverage-check coverage-html coverage-clean cross tidy verify check clean

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9_.-]+:.*## / {printf "%-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Parent otfabric/go.work does not list this module; isolate toolchain commands.
export GOWORK := off

test: ## Run unit tests (with shuffle to catch coupling)
	@echo "Running tests..."
	$(GO) test -shuffle=on $(PKGS)

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	$(GO) test -race -shuffle=on $(PKGS)

vet: ## Run go vet
	@echo "Running go vet..."
	$(GO) vet $(PKGS)

lint: ## Run staticcheck
	@echo "Running staticcheck"
	@staticcheck $(PKGS)

lint-ci: ## Run golangci-lint
	@echo "Running golangci-lint"
	@golangci-lint run $(PKGS)

vuln: ## Run govulncheck
	@echo "Running govulncheck"
	@govulncheck ./...

fmt: ## Run go fmt
	@echo "Running go fmt..."
	@gofmt -w -s .

bench: ## Run benchmarks (if any)
	$(GO) test -run=^$$ -bench=. -benchmem $(PKGS)

coverage: ## Run tests with coverage profile and text summary
	@echo "Running coverage..."
	$(GO) test -shuffle=on -coverprofile=coverage.out \
		-coverpkg=$(shell $(GO) list $(COVER_PKGS) | paste -sd, -) \
		$(COVER_PKGS)
	$(GO) tool cover -func=coverage.out | tee coverage.txt

coverage-check: coverage ## Fail below COVER_MIN (ci.yml coverage-min)
	@total=$$($(GO) tool cover -func=coverage.out | awk '/^total:/ {sub("%","",$$3); print $$3}'); \
	awk -v t="$$total" -v m="$(COVER_MIN)" 'BEGIN { if (t+0 < m+0) { printf "coverage %.1f%% < %s%%\n", t, m; exit 1 } else printf "coverage %.1f%% >= %s%%\n", t, m }'

coverage-html: coverage ## Generate HTML coverage report
	@echo "Generating HTML coverage report..."
	$(GO) tool cover -html=coverage.out -o coverage.html

coverage-clean: ## Remove coverage artifacts
	rm -f coverage.out coverage.txt coverage.html

cross: ## Cross-compile without cgo for CROSS_PLATFORMS
	@for p in $(CROSS_PLATFORMS); do \
		echo "build $$p"; \
		CGO_ENABLED=0 GOOS=$${p%/*} GOARCH=$${p#*/} $(GO) build ./... || exit 1; \
	done

tidy: ## Tidy module files
	@echo "Tidying module files..."
	$(GO) mod tidy

verify: tidy ## Tidy and ensure no diff (like CI)
	@git diff --exit-code || (echo "run 'make tidy' and commit" && exit 1)

check: fmt tidy vet lint lint-ci vuln test test-race coverage ## Run core release checks

clean: ## Clean test cache and coverage artifacts
	$(GO) clean -testcache
	$(MAKE) coverage-clean

FUZZTIME ?= 10s

.PHONY: fuzz
fuzz: ## Run every Fuzz target for FUZZTIME (discovered, never listed)
	@for pkg in $$($(GO) list ./...); do \
		for t in $$($(GO) test -list '^Fuzz' $$pkg | grep '^Fuzz'); do \
			$(GO) test -run '^$$' -fuzz "^$$t$$" -fuzztime $(FUZZTIME) $$pkg || exit 1; \
		done; \
	done
