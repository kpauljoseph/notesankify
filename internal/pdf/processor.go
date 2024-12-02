package pdf

import (
	"context"
	"fmt"
	"github.com/gen2brain/go-fitz"
	"github.com/kpauljoseph/notesankify/pkg/models"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

type Processor struct {
	tempDir       string
	flashcardSize models.PageDimensions
}

func NewProcessor(tempDir string, flashcardSize models.PageDimensions) (*Processor, error) {
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	return &Processor{
		tempDir:       tempDir,
		flashcardSize: flashcardSize,
	}, nil
}

func (p *Processor) ProcessPDF(ctx context.Context, pdfPath string) ([]models.FlashcardPage, error) {
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer doc.Close()

	var flashcards []models.FlashcardPage

	for pageNum := 0; pageNum < doc.NumPage(); pageNum++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			width, height := doc.PageSize(pageNum)

			if MatchesFlashcardSize(width, height, p.flashcardSize) {
				img, err := doc.Image(pageNum)
				if err != nil {
					return nil, fmt.Errorf("failed to extract page %d: %w", pageNum, err)
				}

				filename := fmt.Sprintf("flashcard_%d_%s.png", pageNum, filepath.Base(pdfPath))
				imagePath := filepath.Join(p.tempDir, filename)

				if err := saveImage(img, imagePath); err != nil {
					return nil, fmt.Errorf("failed to save image for page %d: %w", pageNum, err)
				}

				flashcards = append(flashcards, models.FlashcardPage{
					PDFPath:   pdfPath,
					PageNum:   pageNum,
					ImagePath: imagePath,
				})
			}
		}
	}

	return flashcards, nil
}

func MatchesFlashcardSize(width, height float64, expected models.PageDimensions) bool {
	// Allow for small variations in dimensions (e.g., due to rounding)
	const tolerance = 0.1

	widthMatch := abs(width-expected.Width) <= tolerance
	heightMatch := abs(height-expected.Height) <= tolerance

	return widthMatch && heightMatch
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func saveImage(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}

func (p *Processor) Cleanup() error {
	return os.RemoveAll(p.tempDir)
}
