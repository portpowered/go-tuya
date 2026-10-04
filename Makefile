GO ?= go
GOLANGCI_LINT_VERSION ?= v2.14.0
GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
export GOWORK := off

.DEFAULT_GOAL := check
.PHONY: check build test lint fmt replay coverage wire-routes generate-wire \
	build-cli test-cli vet-cli lint-cli module-cli check-cli

check: lint wire-routes build test check-cli check-example

wire-routes:
	$(GO) run ./tools/wiremodels -check
	$(GO) run ./tools/publicmodels -check
	$(GO) run ./tools/wireroutes -check

generate-wire:
# Public numeric bindings are checked by the wire inventory, so generate them first.
	$(GO) run ./tools/publicmodels -generate
	$(GO) run ./tools/wiremodels -generate
	$(GO) run ./tools/wireroutes

build:
	$(GO) build ./...

test:
	$(GO) test -race ./...

lint:
	$(GO) vet ./...
	$(GOLANGCI_LINT) run --allow-parallel-runners --config .golangci.yml --timeout=5m ./...
	$(MAKE) lint-cli
	$(MAKE) lint-example

build-cli:
	$(GO) -C cmd/go-tuya build ./...

test-cli:
	$(GO) -C cmd/go-tuya test -race ./...

vet-cli:
	$(GO) -C cmd/go-tuya vet ./...

lint-cli:
	$(GO) -C cmd/go-tuya run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --allow-parallel-runners --config ../../.golangci.yml --timeout=5m ./...

module-cli:
	$(GO) -C cmd/go-tuya mod tidy -diff
	$(GO) run ./tools/modulepath -dir cmd/go-tuya -want github.com/portpowered/go-tuya/cmd/go-tuya

check-cli: module-cli build-cli test-cli vet-cli

fmt:
	$(GO) fmt ./...

replay:
	$(GO) test -race -coverpkg=./pkg/tuya -cover ./tests/replay
	$(GO) test -race -run TestSyntheticMQTTPairedTranscript ./pkg/tuya

coverage:
	$(GO) test -coverpkg=./pkg/... -coverprofile=coverage.out ./pkg/...
	$(GO) run ./tools/coverage -profile coverage.out -min 80

.PHONY: lint-example check-example
lint-example:
	$(GO) -C examples/auth run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --allow-parallel-runners --config ../../.golangci.yml --timeout=5m ./...

check-example:
	$(GO) -C examples/auth mod tidy -diff
	$(GO) -C examples/auth build ./...
	$(GO) -C examples/auth test -race ./...
	$(GO) -C examples/auth vet ./...
