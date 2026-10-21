BINARY  := flinkit
PKG     := ./...
CMD     := ./cmd/flinkit
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build install test vet fmt lint run tidy clean help

build: ## Build the flinkit binary into ./flinkit
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

install: ## Install flinkit onto your PATH (GOBIN, else ~/go/bin)
	go install $(LDFLAGS) $(CMD)

test: ## Run all tests
	go test $(PKG)

vet: ## Run go vet
	go vet $(PKG)

fmt: ## Format sources with gofmt
	gofmt -l -w .

lint: vet ## Static checks (alias for vet until a linter is wired in)

run: build ## Build and run the binary
	./$(BINARY)

tidy: ## Tidy and verify module dependencies
	go mod tidy

clean: ## Remove build artifacts
	rm -f $(BINARY)
	rm -rf dist

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'
