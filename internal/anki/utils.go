package anki

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
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

func ImageHash(imagePath string) (string, error) {
	// Read the image file
	file, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	// Decode the PNG image
	img, err := png.Decode(file)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	// Create a hash of the image content
	hasher := sha256.New()
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			fmt.Fprintf(hasher, "%d%d%d%d", r, g, b, a)
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func FlashcardHash(questionPath, answerPath string) (string, error) {
	questionHash, err := ImageHash(questionPath)
	if err != nil {
		return "", fmt.Errorf("failed to hash question image: %w", err)
	}

	answerHash, err := ImageHash(answerPath)
	if err != nil {
		return "", fmt.Errorf("failed to hash answer image: %w", err)
	}

	combinedHasher := sha256.New()
	fmt.Fprintf(combinedHasher, "%s%s", questionHash, answerHash)
	return hex.EncodeToString(combinedHasher.Sum(nil)), nil
}
