.PHONY: build test clean lint check test-int test-all

BINARY_NAME=notesankify
BUILD_DIR=build

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) cmd/notesankify/main.go

test:
	go test ./... -v -short

test-int:
	go test ./... -v -run 'Integration'

test-all:
	go test ./... -v -coverage

lint:
	golangci-lint run

check: lint test

clean:
	rm -rf $(BUILD_DIR)
	go clean -testcache

run:
	go run cmd/notesankify/main.go

deps:
	go mod download
	go mod tidy