BINARY      := simple-slurm-json-exporter
MODULE      := github.com/edingc/simple-slurm-json-exporter

# Inject version info at build time
VERSION     ?= $(shell cat VERSION || echo "dev")
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BRANCH      ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
BUILD_DATE  ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := \
  -X github.com/prometheus/common/version.Version=$(VERSION) \
  -X github.com/prometheus/common/version.Revision=$(COMMIT) \
  -X github.com/prometheus/common/version.Branch=$(BRANCH) \
  -X github.com/prometheus/common/version.BuildDate=$(BUILD_DATE) \
  -w -s

.PHONY: all build build-static test lint fmt vet clean help

all: build

## build: compile the binary
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

## build-static: compile a fully static binary (CGO_ENABLED=0)
build-static:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

## test: run all tests
test:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

## lint: run golangci-lint (install: https://golangci-lint.run/usage/install/)
lint:
	golangci-lint run ./...

## fmt: format all Go source files
fmt:
	gofmt -s -w .

## vet: run go vet
vet:
	go vet ./...

## clean: remove build artifacts
clean:
	rm -f $(BINARY) coverage.txt

## help: show this help
help:
	@grep -E '^## ' Makefile | sed 's/## //'
