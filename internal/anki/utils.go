package anki

import (
	"path/filepath"
	"strings"
)

const (
	ANKI_CONNECT_VERSION = 6
)

func GetDeckNameFromPath(rootPrefix string, relativePath string) string {
	// Get directory path without the file name
	dirPath := filepath.Dir(relativePath)
	if dirPath == "." {
		dirPath = ""
	}

	// Get filename without extension
	fileName := strings.TrimSuffix(filepath.Base(relativePath), filepath.Ext(relativePath))

	var parts []string

	// Add root prefix if provided
	if rootPrefix != "" {
		parts = append(parts, rootPrefix)
	}

	// Add directory structure
	if dirPath != "" {
		dirParts := strings.Split(dirPath, string(filepath.Separator))
		parts = append(parts, dirParts...)
	}

	// Add filename as final part
	parts = append(parts, fileName)

	// Join with Anki's separator
	return strings.Join(parts, "::")
}
