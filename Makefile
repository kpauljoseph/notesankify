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

COVERAGE_FILE=coverage.out
GINKGO = go run github.com/onsi/ginkgo/v2/ginkgo
VERSION = 1.0.0
APP_NAME = NotesAnkify
BUNDLE_ID = com.notesankify.app

ROOT_DIR := $(shell pwd)
BUILD_DIR=$(ROOT_DIR)/bin
DIST_DIR=$(ROOT_DIR)/dist
GUI_SRC_DIR := $(ROOT_DIR)/cmd/gui
ASSETS_ICONS_DIR = $(ROOT_DIR)/assets/icons
ICON_SOURCE = $(ROOT_DIR)/assets/icons/NotesAnkify-icon.svg
ICON_SET = $(ROOT_DIR)/assets/icons/icon.iconset
ICONS_NEEDED = 16 32 64 128 256 512 1024
ASSETS_BUNDLE_DIR = $(ROOT_DIR)/assets/bundle

DARWIN_ARM64_DIR := $(DIST_DIR)/darwin-arm64
DARWIN_AMD64_DIR := $(DIST_DIR)/darwin-amd64
WINDOWS_ARM64_DIR := $(DIST_DIR)/windows-arm64
WINDOWS_AMD64_DIR := $(DIST_DIR)/windows-amd64
LINUX_ARM64_DIR := $(DIST_DIR)/linux-arm64
LINUX_AMD64_DIR := $(DIST_DIR)/linux-amd64

GOBUILD=go build -v -ldflags="-s -w"

install-tools:
	@echo "Installing fyne-cross..."
	go install github.com/fyne-io/fyne-cross@latest

build: icons bundle-assets
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_BINARY_NAME) cmd/notesankify/main.go
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(GUI_BINARY_NAME) cmd/gui/main.go

darwin-app: package-macos-arm64 package-macos-amd64
#	@echo "Building MacOS app..."
#	fyne-cross darwin \
#		-arch=amd64,arm64 \
#		$(LDFLAGS) \
#		-icon ./assets/icons/icon.icns \
#		-name "$(APP_NAME)" \
#		--app-id "$(BUNDLE_ID)" \
#		-output "$(APP_NAME)" \
#		$(GUI_SRC_DIR)

windows-app: package-windows-arm64 package-windows-amd64
#	@echo "Building Windows app..."
#	fyne-cross windows \
#		-arch=amd64,arm64 \
#		$(LDFLAGS) \
#		-icon ./assets/icons/icon.ico \
#		-name "$(APP_NAME)" \
#		--app-id "$(BUNDLE_ID)" \
#		-output "$(APP_NAME)" \
#		$(GUI_SRC_DIR)

linux-app:
	@echo "Building Linux app..."
	fyne-cross linux \
		-arch=amd64 \
		$(LDFLAGS) \
		-icon ./assets/icons/png/icon-512.png \
		-name "$(APP_NAME)" \
		--app-id "$(BUNDLE_ID)" \
		-output "$(APP_NAME)" \
		$(GUI_SRC_DIR)

package-all: clean bundle-assets darwin-app windows-app linux-app

bundle-assets:
	mkdir -p $(ASSETS_BUNDLE_DIR)
	fyne bundle -o $(ASSETS_BUNDLE_DIR)/bundled.go --package bundle --prefix Resource $(ASSETS_ICONS_DIR)/png/icon-256.png

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
	rm -f $(APP_NAME).app
	rm -f $(APP_NAME).exe
	rm -f $(APP_NAME)
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

create-dirs:
	mkdir -p $(BUILD_DIR)
	mkdir -p $(DARWIN_ARM64_DIR)
	mkdir -p $(DARWIN_AMD64_DIR)
	mkdir -p $(WINDOWS_ARM64_DIR)
	mkdir -p $(WINDOWS_AMD64_DIR)
	mkdir -p $(LINUX_AMD64_DIR)

package-macos-arm64: clean create-dirs
	cd $(DARWIN_ARM64_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
	fyne package -os darwin \
	--icon $(ASSETS_ICONS_DIR)/icon.icns \
	-name "$(APP_NAME)" \
	-appID "$(BUNDLE_ID)" \
	--sourceDir $(GUI_SRC_DIR)
	cd $(DARWIN_ARM64_DIR) && zip -r $(APP_NAME)-darwin-arm64.zip $(APP_NAME).app

package-macos-amd64: clean create-dirs
	cd $(DARWIN_AMD64_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
	fyne package -os darwin \
	-icon $(ASSETS_ICONS_DIR)/icon.icns \
	-name "$(APP_NAME)" \
	-appID "$(BUNDLE_ID)" \
	--sourceDir $(GUI_SRC_DIR)
	cd $(DARWIN_AMD64_DIR) && zip -r $(APP_NAME)-darwin-amd64.zip $(APP_NAME).app

package-windows-arm64: clean create-dirs
	cd $(WINDOWS_ARM64_DIR) && CGO_ENABLED=1 GOOS=windows GOARCH=arm64 \
	fyne package -os windows \
	-icon  $(ASSETS_ICONS_DIR)/icon.ico \
	-name "$(APP_NAME)" \
	-appID "$(BUNDLE_ID)" \
	--sourceDir $(GUI_SRC_DIR)
	cd $(WINDOWS_ARM64_DIR) && zip -r $(APP_NAME)-windows-arm64.zip $(APP_NAME).app

package-windows-amd64: clean create-dirs
	cd $(WINDOWS_AMD64_DIR) && CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
	fyne package -os windows \
	-icon  $(ASSETS_ICONS_DIR)/icon.ico \
	-name "$(APP_NAME)" \
	-appID "$(BUNDLE_ID)" \
	--sourceDir $(GUI_SRC_DIR)
	cd $(WINDOWS_AMD64_DIR) && zip -r $(APP_NAME)-windows-amd64.zip $(APP_NAME).app

#package-linux:
#	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
#	fyne package -os linux \
#	-icon assets/icons/png/icon-512.png \
#	-name "$(APP_NAME)" \
#	-appID "$(BUNDLE_ID)" \
#	$(GUI_SRC_DIR)

package-all-test: clean bundle-assets package-macos-arm64 package-macos-amd64 package-windows package-linux