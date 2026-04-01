# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
BINARY_NAME=gitm
BINARY_DIR=bin
BINARY_PATH=$(BINARY_DIR)/$(BINARY_NAME)

# Version info injected at build time
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w \
	-X github.com/alexDouze/gitm/cmd.Version=$(VERSION) \
	-X github.com/alexDouze/gitm/cmd.Commit=$(COMMIT) \
	-X github.com/alexDouze/gitm/cmd.Date=$(DATE)

all: test build

build:
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_PATH)

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)

run:
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_PATH)
	./$(BINARY_PATH)

deps:
	$(GOCMD) mod download

# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(BINARY_NAME)_linux

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(BINARY_NAME).exe

build-macos:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(BINARY_NAME)_macos_amd64

build-macos-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(BINARY_NAME)_macos_arm64

# Docker
docker-build:
	docker build -t $(BINARY_NAME):latest .

# GoReleaser
release-snapshot:
	goreleaser release --snapshot --clean

.PHONY: all build test clean run deps build-linux build-windows build-macos build-macos-arm64 docker-build release-snapshot
