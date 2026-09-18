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

.PHONY: all build run check toolchain vet staticcheck lint sec vuln test cover e2e fmt clean version

all: check test

build:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/marketplace

run: build
	./$(BINARY)

version:
	@echo $(VERSION)

check: toolchain vet staticcheck lint sec vuln

# The Dockerfile and go.mod must name the same Go release. The toolchain line
# wins at build time, so a Dockerfile ahead of it is decoration: the image would
# say one version while the binary was built with another, and nothing would
# fail to say so. Dependabot only sees the Dockerfile, so this is the check that
# makes it bring go.mod along.
toolchain:
	@image=$$(sed -n 's|^FROM golang:\([^ ]*\) .*|\1|p' Dockerfile); \
	 pinned=$$(sed -n 's|^toolchain go||p' go.mod); \
	 if [ -z "$$image" ] || [ -z "$$pinned" ]; then \
	   echo "cannot read the Go release from Dockerfile ('$$image') or from go.mod ('$$pinned')"; \
	   exit 1; \
	 fi; \
	 if [ "$$image" != "$$pinned" ]; then \
	   echo "Dockerfile builds on golang:$$image but go.mod pins toolchain go$$pinned."; \
	   echo "The toolchain line wins at build time, so both must name the same release."; \
	   exit 1; \
	 fi; \
	 echo "Go $$pinned in both the Dockerfile and go.mod"

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
