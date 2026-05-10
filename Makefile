.DEFAULT_GOAL := help
.PHONY: help test build vet tidy lint clean snapshot \
        release release-minor release-major release-dry

HELPERS := agent-helper vault-helper sops-helper

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ── Dev ───────────────────────────────────────────────────────────────
test: ## Run go test ./...
	@go test ./...

build: ## Build all helper binaries
	@go build ./...

vet: ## Run go vet ./...
	@go vet ./...

tidy: ## Run go mod tidy
	@go mod tidy

lint: ## Run golangci-lint (must be installed)
	@command -v golangci-lint >/dev/null || { echo "error: golangci-lint not installed" >&2; exit 1; }
	@golangci-lint run

clean: ## Remove dist/ and per-helper bin/ artifacts
	@rm -rf dist/
	@for h in $(HELPERS); do rm -rf cmd/$$h/bin; done

# ── Release ───────────────────────────────────────────────────────────
release: ## Patch-bump version, tag, push (CI builds & publishes)
	@./scripts/release.sh patch

release-minor: ## Minor-bump version, tag, push
	@./scripts/release.sh minor

release-major: ## Major-bump version, tag, push
	@./scripts/release.sh major

release-dry: ## Local goreleaser snapshot - no tag, no push
	@./scripts/release.sh --dry-run

snapshot: release-dry ## Alias for release-dry
