.PHONY: lint test doc doc-check logo website website-serve help

GO = go

lint: ## Check every recipe (offline, sub-second)
	$(GO) run ./cmd/registry-lint

test: ## Run the linter's own tests
	$(GO) test ./...

doc: ## Regenerate the catalogue table in README.md from the recipes
	$(GO) run ./cmd/registry-doc

doc-check: ## Fail if the README catalogue is stale (offline; run by CI)
	$(GO) run ./cmd/registry-doc -check

logo: ## Redraw doc/img from scripts/gen-logo.py (requires Python and Pillow)
	python3 ./scripts/gen-logo.py

website: ## Build the catalog website into website/public (requires hugo)
	cd website && hugo --gc --minify --cleanDestinationDir

website-serve: ## Serve the catalog website locally with live reload
	cd website && hugo server

.DEFAULT_GOAL := help
help:
	@grep -E '^[0-9a-zA-Z_-]+[[:blank:]]*:.*?## .*$$' $(MAKEFILE_LIST) | sort \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[1;32m%-15s\033[0m %s\n", $$1, $$2}'
