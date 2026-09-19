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

.PHONY: all build run check toolchain templates vet staticcheck lint sec vuln test cover integration e2e e2e-lab e2e-deps tf terraform-deps fmt clean version generate

all: check test

build:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/marketplace

run: build
	./$(BINARY)

version:
	@echo $(VERSION)

check: toolchain templates vet staticcheck lint sec vuln

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

# The generated template code is committed, so that a build needs no code
# generator and a reviewer sees what changed. This is what keeps it honest: a
# template edited without regenerating fails here rather than serving the old
# markup.
templates:
	@$(GO) tool templ generate > /dev/null
	@if ! git diff --quiet -- '*_templ.go'; then \
	   echo "the generated template code is out of date; run 'make generate' and commit the result"; \
	   git --no-pager diff --stat -- '*_templ.go'; \
	   exit 1; \
	 fi
	@echo "the generated template code is current"

generate:
	$(GO) tool templ generate

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

# The tests that need a real PostgreSQL. They are behind the `integration` build
# tag so that `make test` needs nothing installed and stays fast, which is what
# lets it run on every save. Set TEST_DATABASE_URL to use a database that is
# already running, as CI does with its service container; without it the suite
# starts a cluster of its own and stops it afterwards
# (internal/platform/dbtest).
integration:
	$(GO) test -tags=integration ./... -race -count=1

# The suite runs under python3, which is the interpreter the environment gave
# Playwright; the pytest on the PATH is a uv tool with its own interpreter and
# cannot import it. Installing here, from the pinned file, is what keeps the
# versions in one place: the environment setup script calls this target instead
# of repeating the numbers (see docs/claude-code-environment.md).
e2e-deps:
	python3 -m pip install --retries 5 --timeout 60 -r e2e/requirements.txt \
	  || python3 -m pip install --break-system-packages --retries 5 --timeout 60 -r e2e/requirements.txt

e2e: build
	@python3 e2e/check_browser.py
	MARKETPLACE_BINARY=$(PWD)/$(BINARY) python3 -m pytest e2e/ -v

# The same suite against a service that is already running, which is how a
# deployment is proved to answer rather than only the build that produced it
# (docs/roadmap.md, F3). The two checks that read the process's own stdout and
# exit status cannot describe a service reached over the internet; they are
# skipped by name, and the run prints which. Nothing is built here: the service
# under test was built by the pipeline.
e2e-lab:
	@test -n "$(MARKETPLACE_BASE_URL)" || { \
	  echo "set MARKETPLACE_BASE_URL to the service to test, for example:"; \
	  echo "  make e2e-lab MARKETPLACE_BASE_URL=https://marketplace.lab.aleogr.dev"; \
	  exit 1; \
	}
	@python3 e2e/check_browser.py
	MARKETPLACE_BASE_URL=$(MARKETPLACE_BASE_URL) python3 -m pytest e2e/ -v

# The Terraform half of the pipeline, in the order the Terraform workflow runs
# it. No backend and no credentials: a session answers whether the files are
# well-formed, and CI answers what they would change.
tf:
	@command -v terraform > /dev/null 2>&1 || { \
	  echo "terraform is not on the path; run 'make terraform-deps'"; \
	  echo "(the environment setup script normally does, see docs/claude-code-environment.md)"; \
	  exit 1; \
	}
	terraform -chdir=infra/terraform fmt -check -recursive
	terraform -chdir=infra/terraform init -backend=false -input=false
	terraform -chdir=infra/terraform validate

# Terraform is not in the environment image, and the release it should be
# lives in infra/terraform/.terraform-version, which the workflow reads too.
# The setup script calls this target instead of repeating that number, for the
# same reason it calls e2e-deps. The archive is verified against the published
# checksums before anything is installed. linux/amd64 is the environment and
# the GitHub runner alike (docs/roadmap.md, appendix).
terraform-deps:
	@version=$$(cat infra/terraform/.terraform-version); \
	 if [ "$$(terraform version 2>/dev/null | sed -n '1s/^Terraform v//p')" = "$$version" ]; then \
	   echo "Terraform $$version already installed"; \
	   exit 0; \
	 fi; \
	 tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT; \
	 base="https://releases.hashicorp.com/terraform/$$version"; \
	 archive="terraform_$${version}_linux_amd64.zip"; \
	 curl -fsSL --retry 5 -o "$$tmp/$$archive" "$$base/$$archive"; \
	 curl -fsSL --retry 5 -o "$$tmp/sums" "$$base/terraform_$${version}_SHA256SUMS"; \
	 (cd "$$tmp" && grep " $$archive$$" sums | sha256sum -c -); \
	 unzip -oq "$$tmp/$$archive" -d "$$tmp"; \
	 install -m 0755 "$$tmp/terraform" /usr/local/bin/terraform; \
	 terraform version

fmt:
	$(GO) tool golangci-lint fmt

clean:
	rm -rf bin coverage.out e2e/screenshots
	rm -rf infra/terraform/.terraform infra/terraform/tfplan infra/terraform/plan.txt
