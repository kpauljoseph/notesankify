package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
)

func GenerateImageHash(img image.Image) (string, error) {
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
