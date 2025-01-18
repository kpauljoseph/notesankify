#!/bin/bash
set -x

# Temporary mechanishm till fyne-cross package works on github runner.
# Script for uploading locally built artifacts to GitHub release
# Usage: ./upload-release.sh <version>
# Example: ./upload-release.sh v0.1.0

VERSION=$1

if [ -z "${VERSION}" ]; then
    echo "Please provide version tag (e.g., v0.1.0)"
    exit 1
fi

if [ -z "${GITHUB_TOKEN}" ]; then
    echo "GITHUB_TOKEN environment variable is not set"
    exit 1
fi

echo "Uploading artifacts for version ${VERSION}..."

# Upload macOS builds
#gh release upload "$VERSION" \
#    "fyne-cross/dist/darwin-amd64/NotesAnkify" \
#    "fyne-cross/dist/darwin-arm64/NotesAnkify" \
#    --clobber

# Upload macOS universal build
gh release upload "${VERSION}" \
    "fyne-cross/dist/darwin-dmg/NotesAnkify-macos-universal-${VERSION}.dmg" \
    --clobber

# Upload Windows builds
gh release upload "${VERSION}" \
    "fyne-cross/dist/windows-amd64/NotesAnkify-windows-amd64-${VERSION}.zip" \
    "fyne-cross/dist/windows-arm64/NotesAnkify-windows-arm64-${VERSION}.zip" \
    --clobber

# Upload Linux builds
echo "Uploading Linux builds..."
gh release upload "${VERSION}" \
    "fyne-cross/dist/linux-amd64/NotesAnkify-linux-amd64-${VERSION}.tar.xz" \
    --clobber

echo "Upload complete!"
