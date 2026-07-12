# appicon — local developer tasks
#
# Usage: make check | make check-fast | make build | make fmt

GO ?= go
PKG ?= ./...
BIN ?= bin/appicon
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS ?= -X github.com/bolens/appicon/internal/version.Version=$(VERSION)

GITLEAKS_VERSION ?= 8.21.2
GOLANGCI_LINT_VERSION ?= 2.12.2
ACTIONLINT_VERSION ?= 1.7.7

.PHONY: help build test vet fmt check check-fast lint \
	check-gitleaks check-actionlint check-markdownlint check-govulncheck \
	check-docs-crosslinks \
	check-ci-path-filters check-nix-packages check-packaging-versions \
	build-packaging clean

help:
	@printf '%s\n' \
		'make build              - build $(BIN)' \
		'make test               - go test ./...' \
		'make vet                - go vet ./...' \
		'make fmt                - gofmt -w .' \
		'make check-fast         - test + vet + gofmt clean' \
		'make check              - full local gate (lint + scripts + packaging versions)' \
		'make build-packaging    - AUR-style smoke builds (set APPICON_BUILD_NIX=1 for nix)' \
		'make lint               - golangci-lint run' \
		'make check-govulncheck  - govulncheck ./...' \
		'make check-docs-crosslinks - docs hub / sibling link contract' \
		'make clean              - remove bin/ and coverage artifacts'

build:
	mkdir -p bin
	$(GO) build -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/appicon

test:
	$(GO) test $(PKG)

vet:
	$(GO) vet $(PKG)

fmt:
	gofmt -w .

check-fast: test vet
	@out=$$(find . \( -path './.gopath' -o -path './.tools' -o -path './vendor' -o -path './.git' \) -prune -o -name '*.go' -print0 | xargs -0 -r gofmt -l); \
	if [ -n "$$out" ]; then \
		printf 'gofmt needed:\n%s\n' "$$out" >&2; \
		exit 1; \
	fi

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo 'golangci-lint not found; install or rely on CI' >&2; \
		exit 1; \
	}
	golangci-lint run

check-gitleaks:
	bash scripts/ci/check-gitleaks.sh

check-actionlint:
	bash scripts/ci/check-actionlint.sh

check-markdownlint:
	bash scripts/ci/check-markdownlint.sh

check-govulncheck:
	bash scripts/ci/check-govulncheck.sh

check-docs-crosslinks:
	bash scripts/ci/check-docs-crosslinks.sh

check-ci-path-filters:
	bash scripts/ci/check-ci-path-filters.sh

check-nix-packages:
	bash scripts/ci/check-nix-packages.sh

check-packaging-versions:
	bash scripts/ci/check-packaging-versions.sh

build-packaging:
	bash scripts/ci/build-packaging.sh

check: check-fast
	@if command -v golangci-lint >/dev/null 2>&1; then $(MAKE) lint; else echo 'skip lint (golangci-lint missing)'; fi
	@$(MAKE) check-govulncheck
	@$(MAKE) check-gitleaks
	@$(MAKE) check-actionlint
	@$(MAKE) check-markdownlint
	@$(MAKE) check-docs-crosslinks
	@$(MAKE) check-ci-path-filters
	@$(MAKE) check-nix-packages
	@$(MAKE) check-packaging-versions

clean:
	rm -rf bin/ dist/ coverage.out coverage.html
