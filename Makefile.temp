.PHONY: build test clean lint check test-int test-all run deps package-all darwin-app windows-app linux-app

VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT  ?= $(shell git rev-parse --short HEAD)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d %H:%M:%S')

LDFLAGS := -ldflags="\
    -X 'github.com/kpauljoseph/notesankify/pkg/version.Version=$(VERSION)' \
    -X 'github.com/kpauljoseph/notesankify/pkg/version.CommitSHA=$(COMMIT)' \
    -X 'github.com/kpauljoseph/notesankify/pkg/version.BuildTime=$(BUILD_TIME)'"

CLI_BINARY_NAME=notesankify
GUI_BINARY_NAME=notesankify-gui
BUILD_DIR=bin
DIST_DIR=dist
COVERAGE_FILE=coverage.out
GINKGO = go run github.com/onsi/ginkgo/v2/ginkgo
VERSION = 1.0.0
APP_NAME = NotesAnkify
BUNDLE_ID = com.notesankify.app

GUI_SRC_DIR=cmd/gui
ICON_SOURCE = assets/icons/NotesAnkify-icon.svg
ICON_SET = assets/icons/icon.iconset
ICONS_NEEDED = 16 32 64 128 256 512 1024
ASSETS_BUNDLE_DIR = assets/bundle

DARWIN_DIST_DIR = $(DIST_DIR)/darwin
WINDOWS_DIST_DIR = $(DIST_DIR)/windows
LINUX_DIST_DIR = $(DIST_DIR)/linux

GOBUILD=go build -v -ldflags="-s -w"

install-tools:
	@echo "Installing fyne-cross..."
	go install github.com/fyne-io/fyne-cross@latest

build: icons bundle-assets
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_BINARY_NAME) cmd/notesankify/main.go
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(GUI_BINARY_NAME) cmd/gui/main.go

darwin-app:
	@echo "Building MacOS app..."
	fyne-cross darwin \
		-arch=amd64,arm64 \
		$(LDFLAGS) \
		-icon ./assets/icons/icon.icns \
		-name "$(APP_NAME)" \
		--app-id "$(BUNDLE_ID)" \
		-output "$(APP_NAME)" \
		$(GUI_SRC_DIR)

windows-app:
	@echo "Building Windows app..."
	fyne-cross windows \
		-arch=amd64,arm64 \
		-icon ./assets/icons/icon.ico \
		-name "$(APP_NAME)" \
		-app-id "$(BUNDLE_ID)" \
		-output "$(APP_NAME)" \
		$(GUI_SRC_DIR)

linux-app:
	@echo "Building Linux app..."
	fyne-cross linux \
		-arch=amd64,arm64 \
		-icon ./assets/icons/png/icon-512.png \
		-name "$(APP_NAME)" \
		--app-id "$(BUNDLE_ID)" \
		-output "$(APP_NAME)" \
		$(GUI_SRC_DIR)

package-all: clean bundle-assets darwin-app windows-app linux-app

bundle-assets:
	mkdir -p $(ASSETS_BUNDLE_DIR)
	fyne bundle -o $(ASSETS_BUNDLE_DIR)/bundled.go --package bundle --prefix Resource assets/icons/png/icon-256.png

test:
	$(GINKGO) -r -v --trace --show-node-events --cover -coverprofile=$(COVERAGE_FILE) ./...

coverage-html: test
	go tool cover -html=$(COVERAGE_FILE)

lint:
	golangci-lint run

check: lint test

clean:
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -f $(COVERAGE_FILE)
	go clean -testcache
	find . -type f -name '*.test' -delete
	rm -rf ./fyne-cross

run:
	go run cmd/notesankify/main.go

run-gui:
	go run cmd/gui/main.go

deps:
	go mod download
	go mod tidy
	go mod verify

# Generate locally and push to main before release.
icons: clean-icons
	@echo "Generating icons..."
	mkdir -p $(ICON_SET)
	mkdir -p assets/icons/png
	# Generate PNGs
	for size in $(ICONS_NEEDED); do \
		magick -background none -density $${size}x$${size} $(ICON_SOURCE) \
		-resize $${size}x$${size} assets/icons/png/icon-$${size}.png; \
		cp assets/icons/png/icon-$${size}.png $(ICON_SET)/icon_$${size}x$${size}.png; \
	done
	# Create icns for macOS
	iconutil -c icns -o assets/icons/icon.icns $(ICON_SET)
	# Create ico for Windows (using sizes up to 256 as per ICO spec)
	magick assets/icons/png/icon-16.png assets/icons/png/icon-32.png \
		assets/icons/png/icon-64.png \
		assets/icons/png/icon-128.png assets/icons/png/icon-256.png \
		assets/icons/icon.ico

clean-icons:
	rm -rf assets/icons/png
	rm -rf assets/icons/icon.iconset
	rm -rf assets/icons/icon.ico
	rm -rf assets/icons/icon.icns
	rm -rf $(ASSETS_BUNDLE_DIR)