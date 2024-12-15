package pdf

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gen2brain/go-fitz"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

const (
	GoodnotesPtWidth  = 455.04
	GoodnotesPtHeight = 587.52

	DimensionTolerance = 1.0

	QuestionKeyword = "QUESTION"
	AnswerKeyword   = "ANSWER"
)

type ProcessingStats struct {
	PDFPath        string
	FlashcardCount int
}

type Processor struct {
	tempDir       string
	outputDir     string
	flashcardSize models.PageDimensions
	logger        *log.Logger
}

func NewProcessor(tempDir, outputDir string, flashcardSize models.PageDimensions, logger *log.Logger) (*Processor, error) {
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &Processor{
		tempDir:       tempDir,
		outputDir:     outputDir,
		flashcardSize: flashcardSize,
		logger:        logger,
	}, nil
}

func (p *Processor) ProcessPDF(ctx context.Context, pdfPath string) (ProcessingStats, error) {
	p.logger.Printf("Processing PDF: %s", pdfPath)
	stats := ProcessingStats{PDFPath: pdfPath}

	doc, err := fitz.New(pdfPath)
	if err != nil {
		return stats, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer doc.Close()

	//Page numbers are zero indexed in the fitz package.
	for pageNum := 0; pageNum < doc.NumPage(); pageNum++ {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
			bounds, err := doc.Bound(pageNum)
			if err != nil {
				return stats, fmt.Errorf("failed to get bounds for page %d: %w", pageNum, err)
			}

			width := float64(bounds.Dx())
			height := float64(bounds.Dy())

			p.logger.Printf("Page %d dimensions: %.2f x %.2f", pageNum, width, height)

			isStandardSize := MatchesGoodnotesDimensions(width, height)

			isFlashcard := false
			if isStandardSize {
				text, err := doc.Text(pageNum)
				if err != nil {
					p.logger.Printf("Warning: couldn't extract text from page %d: %v", pageNum, err)
					continue
				}

				isFlashcard = ContainsFlashcardMarkers(text)
			}

			if isFlashcard {
				p.logger.Printf("Found flashcard page: %d", pageNum)

				img, err := doc.Image(pageNum)
				if err != nil {
					return stats, fmt.Errorf("failed to extract image for page %d: %w", pageNum, err)
				}

				pdfBase := filepath.Base(pdfPath)
				pdfName := strings.TrimSuffix(pdfBase, filepath.Ext(pdfBase))
				outName := fmt.Sprintf("%s_page%d.png", pdfName, pageNum)
				outPath := filepath.Join(p.outputDir, outName)

				if err := saveImage(img, outPath); err != nil {
					return stats, fmt.Errorf("failed to save image for page %d: %w", pageNum, err)
				}

				stats.FlashcardCount++
				p.logger.Printf("Saved flashcard: %s", outName)
			}
		}
	}

	return stats, nil
}

func MatchesGoodnotesDimensions(width, height float64) bool {
	widthMatch := abs(width-GoodnotesPtWidth) <= DimensionTolerance
	heightMatch := abs(height-GoodnotesPtHeight) <= DimensionTolerance

	rotatedWidthMatch := abs(width-GoodnotesPtHeight) <= DimensionTolerance
	rotatedHeightMatch := abs(height-GoodnotesPtWidth) <= DimensionTolerance

	return (widthMatch && heightMatch) || (rotatedWidthMatch && rotatedHeightMatch)
}

func ContainsFlashcardMarkers(text string) bool {
	text = strings.ToUpper(text)
	return strings.Contains(text, QuestionKeyword) && strings.Contains(text, AnswerKeyword)
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
