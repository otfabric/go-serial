# Makefile for github.com/otfabric/go-serial

SHELL := /bin/bash

GO ?= go
PKGS := $(shell $(GO) list ./... | grep -v '/example$$')
ALL_PKGS := ./...


.PHONY: help test test-race vet lint fmt bench coverage coverage-html coverage-clean tidy verify check clean

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9_.-]+:.*## / {printf "%-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run unit tests (with shuffle to catch coupling)
	@echo "Running tests..."
	$(GO) test -shuffle=on $(ALL_PKGS)

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	$(GO) test -race -shuffle=on $(ALL_PKGS)

vet: ## Run go vet
	@echo "Running go vet..."
	$(GO) vet $(ALL_PKGS)

lint: ## Run golangci-lint (optional; install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@echo "Running golangci-lint..."
	@golangci-lint run $(PKGS)

fmt: ## Run go fmt
	@echo "Running go fmt..."
	@gofmt -w -s .

bench: ## Run benchmarks (if any)
	$(GO) test -run=^$$ -bench=. -benchmem $(PKGS)

coverage: ## Run tests with coverage profile and text summary
	@echo "Running coverage..."
	$(GO) test -shuffle=on -coverprofile=coverage.out $(PKGS)
	$(GO) tool cover -func=coverage.out | tee coverage.txt

coverage-html: coverage ## Generate HTML coverage report
	@echo "Generating HTML coverage report..."
	$(GO) tool cover -html=coverage.out -o coverage.html

coverage-clean: ## Remove coverage artifacts
	rm -f coverage.out coverage.txt coverage.html

tidy: ## Tidy module files
	@echo "Tidying module files..."
	$(GO) mod tidy

verify: tidy ## Tidy and ensure no diff (like CI)
	@git diff --exit-code || (echo "run 'make tidy' and commit" && exit 1)

check: fmt tidy vet test test-race coverage ## Run core release checks

clean: ## Clean test cache and coverage artifacts
	$(GO) clean -testcache
	$(MAKE) coverage-clean
