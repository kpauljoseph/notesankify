.PHONY: build test clean lint check test-int test-all run deps package-all darwin-app windows-app linux-app generate-version darwin-dmg

VERSION := $(shell git describe --tags --always --dirty)
COMMIT  := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H%M%S') # ISO 8601 format for filename

# ldflags not working in fyne-cross. Wait for issue to get fixed in repo.
#LDFLAGS := -ldflags="\
#    -X 'github.com/kpauljoseph/notesankify/pkg/version.Version=$(VERSION)' \
#    -X 'github.com/kpauljoseph/notesankify/pkg/version.CommitSHA=$(COMMIT)' \
#    -X 'github.com/kpauljoseph/notesankify/pkg/version.BuildTime=$(BUILD_TIME)'"

#darwin-app: export GOFLAGS='-ldflags=-X=github.com/kpauljoseph/notesankify/pkg/version.Version=123456789 \
#                             -X=github.com/kpauljoseph/notesankify/pkg/version.CommitSHA=commit123456 \
#                             -X=github.com/kpauljoseph/notesankify/pkg/version.BuildTime=build123456'
#windows-app: export GOFLAGS=--ldflags=-X=github.com/kpauljoseph/notesankify/pkg/version.Version=123456789,-X=github.com/kpauljoseph/notesankify/pkg/version.CommitSHA=commit123456,-X=github.com/kpauljoseph/notesankify/pkg/version.BuildTime=build123456

CLI_BINARY_NAME=notesankify
GUI_BINARY_NAME=notesankify-gui
BUILD_DIR=bin
DIST_DIR=fyne-cross/dist
COVERAGE_FILE=coverage.out
GINKGO = go run github.com/onsi/ginkgo/v2/ginkgo
APP_NAME = NotesAnkify
BUNDLE_ID = com.notesankify.app

GUI_SRC_DIR=cmd/notesankify-gui
ASSETS_ICONS = assets/icons
ICON_SOURCE = $(ASSETS_ICONS)/NotesAnkify-icon.svg
ICON_SET = $(ASSETS_ICONS)/icon.iconset
ICONS_NEEDED = 16 32 64 128 256 512 1024
ASSETS_BUNDLE_DIR = assets/bundle

DARWIN_UNIVERSAL_DIR := $(DIST_DIR)/darwin-universal
DMG_DIR := $(DIST_DIR)/darwin-dmg

GOBUILD=go build -v -ldflags="-s -w"

generate-version:
	@echo "Generating version information..."
	@echo "package version" > pkg/version/generated.go
	@echo "" >> pkg/version/generated.go
	@echo "func init() {" >> pkg/version/generated.go
	@echo "    Version = \"$(VERSION)\"" >> pkg/version/generated.go
	@echo "    CommitSHA = \"$(COMMIT)\"" >> pkg/version/generated.go
	@echo "    BuildTime = \"$(BUILD_TIME)\"" >> pkg/version/generated.go
	@echo "}" >> pkg/version/generated.go
	@cat pkg/version/generated.go

install-tools:
	@echo "Installing fyne-cross..."
	go install github.com/fyne-io/fyne-cross@latest

build: generate-version icons bundle-assets
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(CLI_BINARY_NAME) cmd/notesankify/main.go
	$(GOBUILD) -o $(BUILD_DIR)/$(GUI_BINARY_NAME) cmd/gui/main.go


darwin-app: generate-version
	@echo "Building MacOS app..."
	fyne-cross darwin \
		-arch=amd64,arm64 \
		-icon ./$(ASSETS_ICONS)/icon.icns \
		-name "$(APP_NAME)" \
		--app-id "$(BUNDLE_ID)" \
		-output "$(APP_NAME)" \
		$(GUI_SRC_DIR)

darwin-universal: darwin-app
	@echo "Creating universal macOS binary..."
	mkdir -p $(DARWIN_UNIVERSAL_DIR)/$(APP_NAME).app
	cp -r fyne-cross/dist/darwin-amd64/$(APP_NAME).app/* $(DARWIN_UNIVERSAL_DIR)/$(APP_NAME).app/
	mkdir -p $(DARWIN_UNIVERSAL_DIR)/$(APP_NAME).app/Contents/MacOS
	lipo -create \
		fyne-cross/dist/darwin-amd64/$(APP_NAME).app/Contents/MacOS/$(GUI_BINARY_NAME) \
		fyne-cross/dist/darwin-arm64/$(APP_NAME).app/Contents/MacOS/$(GUI_BINARY_NAME) \
		-output $(DARWIN_UNIVERSAL_DIR)/$(APP_NAME).app/Contents/MacOS/$(GUI_BINARY_NAME)


darwin-dmg: darwin-universal
	@echo "Creating DMG..."
	mkdir -p $(DMG_DIR)
	hdiutil create -volname "$(APP_NAME)" -srcfolder $(DARWIN_UNIVERSAL_DIR)/$(APP_NAME).app -ov -format UDZO $(DMG_DIR)/$(APP_NAME)-macos-universal-$(VERSION).dmg

windows-app: generate-version
	@echo "Building Windows app..."
	fyne-cross windows \
		-arch=amd64,arm64 \
		-icon ./$(ASSETS_ICONS)/icon.ico \
		-name "$(APP_NAME)" \
		-app-id "$(BUNDLE_ID)" \
		-output "$(APP_NAME)" \
		$(GUI_SRC_DIR)
	mv $(DIST_DIR)/windows-amd64/$(APP_NAME).zip $(DIST_DIR)/windows-amd64/$(APP_NAME)-windows-amd64-$(VERSION).zip
	mv $(DIST_DIR)/windows-arm64/$(APP_NAME).zip $(DIST_DIR)/windows-arm64/$(APP_NAME)-windows-arm64-$(VERSION).zip

# linux-arm64 does not work yet.
linux-app: generate-version
	@echo "Building Linux app..."
	fyne-cross linux \
		-arch=amd64 \
		-icon ./$(ASSETS_ICONS)/png/icon-512.png \
		-name "$(APP_NAME)" \
		--app-id "$(BUNDLE_ID)" \
		-output "$(APP_NAME)" \
		$(GUI_SRC_DIR)
	mv $(DIST_DIR)/linux-amd64/$(APP_NAME).tar.xz $(DIST_DIR)/linux-amd64/$(APP_NAME)-linux-amd64-$(VERSION).tar.xz

package-all: clean bundle-assets generate-version darwin-dmg windows-app linux-app

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
	rm -f pkg/version/version.go.tmp
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
		-resize $${size}x$${size} $(ASSETS_ICONS)/png/icon-$${size}.png; \
		cp $(ASSETS_ICONS)/png/icon-$${size}.png $(ICON_SET)/icon_$${size}x$${size}.png; \
	done
	# Create icns for macOS
	iconutil -c icns -o $(ASSETS_ICONS)/icon.icns $(ICON_SET)
	# Create ico for Windows (using sizes up to 256 as per ICO spec)
	magick $(ASSETS_ICONS)/png/icon-16.png $(ASSETS_ICONS)/png/icon-32.png \
		$(ASSETS_ICONS)/png/icon-64.png \
		$(ASSETS_ICONS)/png/icon-128.png $(ASSETS_ICONS)/png/icon-256.png \
		$(ASSETS_ICONS)/icon.ico

clean-icons:
	rm -rf $(ASSETS_ICONS)/png
	rm -rf $(ASSETS_ICONS)/icon.iconset
	rm -rf $(ASSETS_ICONS)/icon.ico
	rm -rf $(ASSETS_ICONS)/icon.icns
	rm -rf $(ASSETS_BUNDLE_DIR)