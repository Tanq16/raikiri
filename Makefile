APP_NAME := raikiri
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev-build")
LDFLAGS := -s -w -X github.com/tanq16/raikiri/cmd.AppVersion=$(VERSION)

.PHONY: build build-all clean version help assets verify-assets

## build: Build for current platform
build:
	go build -ldflags="$(LDFLAGS)" -o $(APP_NAME) .

## build-all: Cross-compile for 6 platform combos
build-all:
	GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(APP_NAME)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(APP_NAME)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(APP_NAME)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(APP_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(APP_NAME)-windows-amd64.exe .
	GOOS=windows GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(APP_NAME)-windows-arm64.exe .

## clean: Remove built binaries
clean:
	rm -f $(APP_NAME) $(APP_NAME)-*

## version: Print the current version
version:
	@echo $(VERSION)

## assets: Download frontend static assets
assets:
	bash scripts/asset_download.sh

## verify-assets: Check that required static assets exist
verify-assets:
	@test -f internal/server/static/static/js/tailwindcss.js || (echo "Missing tailwindcss.js" && exit 1)
	@test -f internal/server/static/static/js/lucide.min.js  || (echo "Missing lucide.min.js" && exit 1)
	@test -f internal/server/static/static/js/hls.min.js     || (echo "Missing hls.min.js" && exit 1)
	@echo "All required assets present."

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
