.PHONY: build clean

# 如果找不到 tag 则使用 HEAD commit
VERSION=$(shell git describe --tags `git rev-list --tags --max-count=1` 2>/dev/null || git rev-parse --short HEAD)
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BIN=go-devops
DIR_SRC=./cmd/go-devops

GO_ENV=CGO_ENABLED=0
GO_FLAGS=-ldflags="-X main.version=$(VERSION) -X 'main.buildTime=$(BUILD_TIME)' -extldflags -static -s -w" -trimpath
GO=$(GO_ENV) $(shell which go)

build: $(DIR_SRC)/main.go
	@$(GO) build $(GO_FLAGS) -o $(BIN) $(DIR_SRC)

# clean all build result
clean:
	@$(GO) clean ./...
	@rm -f $(BIN)
	@rm -rf ./dist/*
