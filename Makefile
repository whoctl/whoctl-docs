# whoctl-docs
#
# The published site: templates, styles, and the builder that turns every
# provider's documentation bundle into one static site.
#
# scripts/ holds only what is a program in its own right — serving the site needs a
# container and a handful of decisions. Everything else is a recipe, so there is
# one place that knows how to do each thing.

DOCS_PORT        ?= 8080
CONTAINER_ENGINE ?= podman
NGINX_IMAGE      ?= nginx:alpine
OUTPUT           ?= site
# A local catalogue wins when it is there, so a workspace builds against the
# providers checked out beside it instead of against published releases.
CATALOGUE        ?= $(if $(wildcard providers.local.yaml),providers.local.yaml,providers.yaml)
VERSION          ?= dev

export DOCS_PORT CONTAINER_ENGINE NGINX_IMAGE OUTPUT VERSION CATALOGUE

.DEFAULT_GOAL := help

## build: render the site from the bundles named in the catalogue
.PHONY: build
build:
	@go run ./cmd/whoctl-docs -providers $(CATALOGUE) -o $(OUTPUT) -version $(VERSION)

## local: point the catalogue at the provider repositories checked out beside this one
.PHONY: local
local:
	@echo "# Written by \`make local\`. Not committed: it describes this checkout." > providers.local.yaml
	@echo "providers:" >> providers.local.yaml
	@for repo in ../whoctl-provider-*; do \
		name=$${repo#../whoctl-provider-}; \
		[ -d "$$repo" ] || continue; \
		( cd "$$repo" && make --no-print-directory docs >/dev/null ); \
		printf '  - name: %s\n    version: dev\n    bundle: %s/bundle.json\n' "$$name" "$$repo" >> providers.local.yaml; \
	done
	@echo "wrote providers.local.yaml"

## keygen: generate the registry signing key pair (run once, by a person)
.PHONY: keygen
keygen:
	@go run ./cmd/keygen

## serve: build it, then serve it with nginx on localhost (see DOCS_PORT)
.PHONY: serve
serve:
	@scripts/serve.sh

## test: the whole suite
.PHONY: test
test:
	@go test ./...

## fmt: format and vet
.PHONY: fmt
fmt:
	@gofmt -w .
	@go vet ./...

## clean: remove the rendered site
.PHONY: clean
clean:
	@rm -rf $(OUTPUT)

## help: list the available targets
.PHONY: help
help:
	@echo "whoctl-docs targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | awk -F': ' '{printf "  %-7s %s\n", $$1, $$2}'
	@echo
	@echo "Variables: DOCS_PORT=$(DOCS_PORT) OUTPUT=$(OUTPUT) CONTAINER_ENGINE=$(CONTAINER_ENGINE)"
	@echo
	@echo "A provider's bundle comes from its own repository:"
	@echo "  cd ../whoctl-provider-linux && make docs"
