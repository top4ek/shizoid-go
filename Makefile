SHELL := /bin/bash

GOLANGCI_LINT_VERSION ?= v2.13.2
GO := go
GOLANGCI_LINT := $(if $(shell command -v golangci-lint 2>/dev/null),$(shell command -v golangci-lint 2>/dev/null),$(shell $(GO) env GOPATH)/bin/golangci-lint)

.PHONY: help all build test test-short vet fmt lint lint-install dev dev-down vuln ci

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

all: build vet test ## Build, vet and run the full test suite

build: ## Compile all packages
	cd apps/shizoid && $(GO) build ./...

test: ## Run the full test suite (integration tests need docker)
	cd apps/shizoid && $(GO) test -race ./...

test-short: ## Run unit tests only (skips dockerized integration tests)
	cd apps/shizoid && $(GO) test -short ./...

vet: ## Run go vet
	cd apps/shizoid && $(GO) vet ./...

fmt: ## Check gofmt formatting
	@test -z "$$(cd apps/shizoid && gofmt -l cmd internal)"

lint: ## Run golangci-lint
	cd apps/shizoid && $(GOLANGCI_LINT) run

lint-install: ## Install pinned golangci-lint
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$($(GO) env GOPATH)/bin $(GOLANGCI_LINT_VERSION)

dev: ## Run the dev stack in Docker (hot reload + Delve; postgres + llama)
	test -f build/dev/.env || cp build/dev/.env-example build/dev/.env
	docker compose up --build

dev-down: ## Stop the dev stack
	docker compose down

vuln: ## Scan dependencies for known vulnerabilities
	cd apps/shizoid && $(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

ci: fmt vet lint test vuln ## Everything CI runs
