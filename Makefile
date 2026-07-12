# appicon — local developer tasks
#
# Usage: make check | make check-fast | make build | make fmt

GO ?= go
PKG ?= ./...
BIN ?= bin/appicon
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS ?= -X github.com/bolens/appicon/internal/version.Version=$(VERSION)

GITLEAKS_VERSION ?= 8.21.2
GOLANGCI_LINT_VERSION ?= 1.64.8
ACTIONLINT_VERSION ?= 1.7.7

.PHONY: help build test vet fmt check check-fast lint \
	check-gitleaks check-actionlint check-markdownlint clean

help:
	@printf '%s\n' \
		'make build              - build $(BIN)' \
		'make test               - go test ./...' \
		'make vet                - go vet ./...' \
		'make fmt                - gofmt -w .' \
		'make check-fast         - test + vet + gofmt clean' \
		'make check              - check-fast + lint + gitleaks + actionlint + markdownlint' \
		'make lint               - golangci-lint run' \
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
	@out=$$(gofmt -l .); \
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

check: check-fast
	@if command -v golangci-lint >/dev/null 2>&1; then $(MAKE) lint; else echo 'skip lint (golangci-lint missing)'; fi
	@$(MAKE) check-gitleaks
	@$(MAKE) check-actionlint
	@$(MAKE) check-markdownlint

clean:
	rm -rf bin/ dist/ coverage.out coverage.html
