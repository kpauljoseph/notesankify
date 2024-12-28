.PHONY: build test clean lint check test-int test-all run deps package-all darwin-app windows-app linux-app

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

DARWIN_DIST_DIR = $(DIST_DIR)/darwin
WINDOWS_DIST_DIR = $(DIST_DIR)/windows
LINUX_DIST_DIR = $(DIST_DIR)/linux

GOBUILD=go build -v -ldflags="-s -w"

build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(CLI_BINARY_NAME) cmd/notesankify/main.go
	$(GOBUILD) -o $(BUILD_DIR)/$(GUI_BINARY_NAME) cmd/gui/main.go

darwin-app: icons
	@echo "Building macOS app..."
	fyne version
	mkdir -p $(DARWIN_DIST_DIR)
	cd $(DARWIN_DIST_DIR) && fyne package \
		-os darwin \
		-icon "../../assets/icons/icon.icns" \
		-name "$(APP_NAME)" \
		-appID "$(BUNDLE_ID)" \
		--sourceDir "../../$(GUI_SRC_DIR)" \
		-release

windows-app: icons
	@echo "Building Windows app..."
	fyne version
	mkdir -p $(WINDOWS_DIST_DIR)
	cd $(WINDOWS_DIST_DIR) && fyne package \
		-os windows \
		-icon "../../assets/icons/icon.ico" \
		-name "$(APP_NAME)" \
		-appID "$(BUNDLE_ID)" \
		--sourceDir "../../$(GUI_SRC_DIR)" \
		-release

linux-app: icons
	@echo "Building Linux app..."
	fyne version
	mkdir -p $(LINUX_DIST_DIR)
	cd $(LINUX_DIST_DIR) && fyne package \
		-os linux \
		-icon "../../assets/icons/icon.png" \
		-name "$(APP_NAME)" \
		-appID "$(BUNDLE_ID)" \
		--sourceDir "../../$(GUI_SRC_DIR)" \
		-release

package-all: clean darwin-app windows-app linux-app
	cd $(DIST_DIR) && \
	zip -r NotesAnkify-$(VERSION)-darwin.zip darwin && \
	zip -r NotesAnkify-$(VERSION)-windows.zip windows && \
	tar czf NotesAnkify-$(VERSION)-linux.tar.gz linux

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
	rm -rf assets/icons/png
	rm -rf $(ICON_SET)
	rm -f assets/icons/icon.icns
	rm -rf $(APP_NAME).app

run:
	go run cmd/notesankify/main.go

run-gui:
	go run cmd/gui/main.go

deps:
	go mod download
	go mod tidy
	go mod verify

icons:
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
	rm -f assets/icons/icon.ico
	rm -f assets/icons/icon.icns
