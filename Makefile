BINARY      := symvibe
PKG         := ./cmd/symvibe
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X github.com/danieljustus/symaira-vibecoder/internal/version.Version=$(VERSION)
GOFLAGS_ENV := CGO_ENABLED=0

.DEFAULT_GOAL := build

## seed-copy: keep the embedded seed in sync with the human-facing source
.PHONY: seed-copy
seed-copy:
	cp config/seed-cycle.toml internal/config/seed/seed-cycle.toml

## web: build the React/Vite board into web/dist (optional upgrade; the
## committed web/dist/index.html already works without Node)
.PHONY: web
web:
	@if [ -f web/package.json ]; then cd web && npm ci && npm run build; \
	else echo "web/package.json not present — using the committed dependency-free board"; fi

## build: produce the single binary (embeds web/dist + the seed)
.PHONY: build
build: seed-copy
	$(GOFLAGS_ENV) go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)
	@echo "built ./$(BINARY) ($(VERSION))"

## install: go install into GOBIN
.PHONY: install
install: seed-copy
	$(GOFLAGS_ENV) go install -ldflags "$(LDFLAGS)" $(PKG)

## run: build the board and serve it
.PHONY: run
run: build
	./$(BINARY) serve

## dev: hot Go server (no browser) for development
.PHONY: dev
dev:
	$(GOFLAGS_ENV) go run $(PKG) serve --no-open

## test: unit tests
.PHONY: test
test:
	$(GOFLAGS_ENV) go test ./...

## test-race: race detector (needs CGO)
.PHONY: test-race
test-race:
	CGO_ENABLED=1 go test -race ./internal/...

## lint: gofmt + vet
.PHONY: lint
lint:
	@test -z "$$(gofmt -l ./cmd ./internal ./web/embed.go)" || (echo "gofmt needed:"; gofmt -l ./cmd ./internal; exit 1)
	$(GOFLAGS_ENV) go vet ./...

## clean: remove build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY)
	rm -rf bin

## help: list targets
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
