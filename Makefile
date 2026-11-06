.DEFAULT_GOAL := build

GO ?= go
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
DOCKER ?= docker
INTEGRATION_IMAGE ?= pockit-integration

.PHONY: build install deps test test-race test-integration vet fmt fmt-check check clean help

build: ## Build ./pockit with reproducible build flags
	$(GO) build -trimpath -buildvcs=false -ldflags=-buildid= -o ./pockit ./cmd/pockit

install: ## Build for this machine and install to ~/.local/bin (override BINDIR or PREFIX)
	@mkdir -p "$(BINDIR)"
	GOOS="$$($(GO) env GOHOSTOS)" GOARCH="$$($(GO) env GOHOSTARCH)" $(GO) build -trimpath -buildvcs=false -ldflags=-buildid= -o "$(BINDIR)/pockit" ./cmd/pockit
	@printf 'Installed %s\n' "$(BINDIR)/pockit"
	@if ! printf '%s\n' ":$$PATH:" | grep -Fq -- ":$(BINDIR):"; then \
		printf 'Add the install directory to your shell PATH:\n  export PATH="%s:$$PATH"\n' "$(BINDIR)"; \
	fi

deps: ## Download Go dependencies
	$(GO) mod download

test: ## Run tests
	$(GO) test ./...

test-race: ## Run tests with the race detector (requires CGO)
	$(GO) test -race ./...

test-integration: ## Install all supported hosts in Docker and test package installation/discovery (amd64)
	$(DOCKER) build --platform linux/amd64 -f integration/Dockerfile -t "$(INTEGRATION_IMAGE)" .
	$(DOCKER) run --rm --network none --cap-drop ALL --security-opt no-new-privileges "$(INTEGRATION_IMAGE)"

vet: ## Run go vet
	$(GO) vet ./...

fmt: ## Format Go source files
	gofmt -w cmd data internal integration

fmt-check: ## Check formatting without modifying files
	gofmt -l cmd data internal integration | diff -u /dev/null -

check: fmt-check vet test test-race ## Run formatting, vet, and test checks

clean: ## Remove the built binary
	rm -f ./pockit

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
