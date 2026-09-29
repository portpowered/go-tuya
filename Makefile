GO ?= go
GOLANGCI_LINT_VERSION ?= v2.14.0
GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
export GOWORK := off

.DEFAULT_GOAL := check
.PHONY: check build test lint fmt replay coverage wire-routes generate-wire

check: lint wire-routes build test

wire-routes:
	$(GO) run ./tools/wireroutes -check

generate-wire:
	$(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config pkg/tuya/internal/wire/config.yaml api/openapi.yaml
	$(GO) run ./tools/wireroutes

build:
	$(GO) build ./...

test:
	$(GO) test -race ./...

lint:
	$(GO) vet ./...
	$(GOLANGCI_LINT) run --config .golangci.yml --timeout=5m ./...

fmt:
	$(GO) fmt ./...

replay:
	$(GO) test -race -coverpkg=./pkg/tuya -cover ./tests/replay
	$(GO) test -race -run TestSyntheticMQTTPairedTranscript ./pkg/tuya

coverage:
	$(GO) test -coverpkg=./pkg/... -coverprofile=coverage.out ./pkg/...
	$(GO) run ./tools/coverage -profile coverage.out -min 80
