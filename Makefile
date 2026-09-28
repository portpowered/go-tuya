GO ?= go
export GOWORK := off

.DEFAULT_GOAL := check
.PHONY: check build test lint fmt replay

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
	$(GO) test -race -coverpkg=./tuya -cover ./tests/replay
