.PHONY: lint test website website-serve help

GO = go

lint: ## Check every recipe (offline, sub-second)
	$(GO) run ./cmd/registry-lint

test: ## Run the linter's own tests
	$(GO) test ./...

website: ## Build the catalog website into website/public (requires hugo)
	cd website && hugo --gc --minify --cleanDestinationDir

website-serve: ## Serve the catalog website locally with live reload
	cd website && hugo server

.DEFAULT_GOAL := help
help:
	@grep -E '^[0-9a-zA-Z_-]+[[:blank:]]*:.*?## .*$$' $(MAKEFILE_LIST) | sort \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[1;32m%-15s\033[0m %s\n", $$1, $$2}'
