GO ?= go
export GOWORK := off

.DEFAULT_GOAL := check
.PHONY: check build test lint fmt replay coverage

check: lint build test

build:
	$(GO) build ./...

test:
	$(GO) test -race ./...

lint:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

replay:
	$(GO) test -race -coverpkg=./pkg/tuya -cover ./tests/replay

coverage:
	$(GO) test -coverpkg=./pkg/... -coverprofile=coverage.out ./pkg/...
	$(GO) run ./tools/coverage -profile coverage.out -min 80
