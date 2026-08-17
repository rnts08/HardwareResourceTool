SHELL := /bin/sh

GO ?= go
TARGET ?= hardware-resources
LINUX_TARGET ?= hardware-resources-linux-amd64
VERSION ?= 0.10.6
PREFIX ?= /usr/local
DESTDIR ?=

COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_FLAGS := -X hardware-resources-tool/internal/cli.version=$(VERSION) \
	-X hardware-resources-tool/internal/cli.buildCommit=$(COMMIT) \
	-X hardware-resources-tool/internal/cli.buildDate=$(BUILD_DATE)
RELEASE_FLAGS := -trimpath -ldflags "-s -w -buildid= $(VERSION_FLAGS)"

.DEFAULT_GOAL := help

.PHONY: all help build linux test vet fmt fmt-check check coverage install live-test clean

all: linux

help:
	@printf '%s\n' \
		'build       Build the host binary' \
		'linux       Build and strip a minimum Linux amd64 binary' \
		'test        Run all Go tests' \
		'vet         Run go vet' \
		'fmt         Format Go sources' \
		'fmt-check   Verify Go sources are formatted' \
		'check       Run formatting, tests, and vet' \
		'coverage    Write coverage.out and an HTML coverage report' \
		'install     Install the stripped Linux binary under PREFIX' \
		'live-test   Run the root-only live collection capture' \
		'clean       Remove generated binaries and coverage files'

build:
	$(GO) build $(RELEASE_FLAGS) -o $(TARGET) ./cmd/hardware-resources

linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO) build $(RELEASE_FLAGS) -o $(LINUX_TARGET) ./cmd/hardware-resources
	@if command -v strip >/dev/null 2>&1; then strip --strip-all $(LINUX_TARGET); else echo 'warning: strip not found; binary was linker-stripped only' >&2; fi

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*'))" || { echo 'Go sources need formatting; run make fmt' >&2; exit 1; }

check: fmt-check test vet

coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

install: linux
	install -Dm755 $(LINUX_TARGET) $(DESTDIR)$(PREFIX)/bin/hardware-resources

live-test:
	sudo ./scripts/live-collection-test.sh --duration 5s

clean:
	rm -f $(TARGET) $(LINUX_TARGET) coverage.out coverage.html
