package pdf

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/gen2brain/go-fitz"
	"github.com/kpauljoseph/notesankify/pkg/models"
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
	log.Printf("Processing PDF: %s", pdfPath)
	log.Printf("Using flashcard dimensions: %.2f x %.2f", p.flashcardSize.Width, p.flashcardSize.Height)

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
			bounds, err := doc.Bound(pageNum)
			if err != nil {
				return nil, fmt.Errorf("failed to get bounds for page %d: %w", pageNum, err)
			}

			width := float64(bounds.Dx())
			height := float64(bounds.Dy())

			log.Printf("Page %d dimensions: %.2f x %.2f", pageNum, width, height)

			if MatchesFlashcardSize(width, height, p.flashcardSize) {
				log.Printf("Found matching flashcard page: %d", pageNum)

				img, err := doc.ImageDPI(pageNum, 300.0)
				if err != nil {
					return nil, fmt.Errorf("failed to extract image for page %d: %w", pageNum, err)
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
	const tolerance = 5.0 // Increased tolerance for pixel measurements

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

func saveImage(img *image.RGBA, path string) error {
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
