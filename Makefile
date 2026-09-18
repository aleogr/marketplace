# Build and check targets. The check target runs exactly what CI runs, so a
# green run here means a green run there (see CLAUDE.md).

GO      ?= go
BINARY  ?= bin/marketplace
PKG     := github.com/aleogr/marketplace

# The build identifier: the clean tag on a tagged commit, the tag plus distance
# and commit after it, or the short commit hash and build date before the first
# tag (see docs/requirements.md, section 27).
VERSION ?= $(shell git describe --tags --dirty 2>/dev/null \
	|| echo "$$(git rev-parse --short HEAD 2>/dev/null || echo unknown)-$$(date -u +%Y%m%d)")
LDFLAGS := -X $(PKG)/internal/platform/version.version=$(VERSION)

.PHONY: all build run check vet staticcheck lint sec vuln test cover e2e fmt clean version

all: check test

build:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/marketplace

run: build
	./$(BINARY)

version:
	@echo $(VERSION)

check: vet staticcheck lint sec vuln

vet:
	$(GO) vet ./...

staticcheck:
	$(GO) tool staticcheck ./...

lint:
	$(GO) tool golangci-lint run

sec:
	$(GO) tool gosec -quiet ./...

vuln:
	$(GO) tool govulncheck ./...

test:
	$(GO) test ./... -race -cover

cover:
	$(GO) test ./... -race -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out | tail -1

e2e: build
	MARKETPLACE_BINARY=$(PWD)/$(BINARY) python3 -m pytest e2e/ -v

fmt:
	$(GO) tool golangci-lint fmt

clean:
	rm -rf bin coverage.out e2e/screenshots
