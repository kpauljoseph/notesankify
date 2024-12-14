.PHONY: build test clean lint check test-int test-all run deps

BINARY_NAME=notesankify
BUILD_DIR=bin
COVERAGE_FILE=coverage.out
GINKGO = go run github.com/onsi/ginkgo/v2/ginkgo

# Add Go build flags for better optimization and debugging
GOBUILD=go build -v -ldflags="-s -w"

build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) cmd/notesankify/main.go

test:
	$(GINKGO) -r -v --trace --show-node-events --cover -coverprofile=$(COVERAGE_FILE) ./...

coverage-html: test
	go tool cover -html=$(COVERAGE_FILE)

lint:
	golangci-lint run

check: lint test

clean:
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_FILE)
	go clean -testcache
	find . -type f -name '*.test' -delete

run:
	go run cmd/notesankify/main.go

deps:
	go mod download
	go mod tidy
	go mod verify