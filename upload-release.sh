#!/bin/bash

# Script for uploading locally built artifacts to GitHub release
# Usage: ./upload-release.sh <version>
# Example: ./upload-release.sh v0.1.0

VERSION=$1

if [ -z "$VERSION" ]; then
    echo "Please provide version tag (e.g., v0.1.0)"
    exit 1
fi

if [ -z "$GITHUB_TOKEN" ]; then
    echo "GITHUB_TOKEN environment variable is not set"
    exit 1
fi

echo "Uploading artifacts for version $VERSION..."

# Upload macOS builds
gh release upload "$VERSION" \
    "fyne-cross/dist/darwin-amd64/NotesAnkify.app.zip" \
    "fyne-cross/dist/darwin-arm64/NotesAnkify.app.zip" \
    --clobber

# Upload Windows builds
gh release upload "$VERSION" \
    "fyne-cross/dist/windows-amd64/NotesAnkify.exe" \
    "fyne-cross/dist/windows-arm64/NotesAnkify.exe" \
    --clobber

# Upload Linux builds
echo "Uploading Linux builds..."
gh release upload "$VERSION" \
    "fyne-cross/dist/linux-amd64/NotesAnkify" \
    --clobber

echo "Upload complete!"
