BUF ?= buf
GO ?= go
GOLANGCI-LINT ?= golangci-lint

all: fmt lint

fmt:
	@$(BUF) format -w
	@$(GO) $@ ./...

generate:
	@$(GO) $@ ./...

lint:
	@$(GOLANGCI-LINT) run --fix

proto:
	@$(BUF) generate

gen: generate

SEMVER ?= 0.6.0

release:
	@$(GIT) tag v$(SEMVER)
	@$(GIT) push --tags


.PHONY: all fmt generate lint proto gen release
