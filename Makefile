.PHONY: build clean install test lint fmt run snapshot release

BINARY := taux
BUILD_DIR := bin
MODULE := github.com/glory0216/taux
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS := -ldflags "-s -w -X $(MODULE)/internal/cli.Version=$(VERSION) -X $(MODULE)/internal/cli.Commit=$(COMMIT)"

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/taux/

install: build
	mkdir -p $(HOME)/.local/bin
	cp $(BUILD_DIR)/$(BINARY) $(HOME)/.local/bin/$(BINARY)

clean:
	rm -rf $(BUILD_DIR) dist/

test:
	go test -race ./...

lint:
	go vet ./...

fmt:
	gofmt -s -w .

run: build
	./$(BUILD_DIR)/$(BINARY)

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean

.DEFAULT_GOAL := build
