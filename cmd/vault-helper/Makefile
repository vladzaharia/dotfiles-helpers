.DEFAULT_GOAL := help
.PHONY: help build test run install clean

HELPER := vault-helper
ROOT := $(shell git rev-parse --show-toplevel)
ARGS ?=

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build this helper into ./bin/
	@cd $(ROOT) && go build -o cmd/$(HELPER)/bin/$(HELPER) ./cmd/$(HELPER)

test: ## Run go test for the whole module (no per-helper isolation today)
	@cd $(ROOT) && go test ./...

run: build ## Build and run; pass args via ARGS="--flag value"
	@./bin/$(HELPER) $(ARGS)

install: ## go install to $GOBIN
	@cd $(ROOT) && go install ./cmd/$(HELPER)

clean: ## Remove ./bin/
	@rm -rf bin/
