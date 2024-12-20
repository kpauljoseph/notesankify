package pdf

import (
	"context"
	"fmt"
	"github.com/kpauljoseph/notesankify/pkg/logger"
	"github.com/kpauljoseph/notesankify/pkg/utils"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/gen2brain/go-fitz"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

const (
	DimensionTolerance = 1.0

	QuestionKeyword = "QUESTION"
	AnswerKeyword   = "ANSWER"
)

type ProcessingStats struct {
	PDFPath        string
	FlashcardCount int
	ImagePairs     []ImagePair
}

type Processor struct {
	tempDir       string
	outputDir     string
	flashcardSize models.PageDimensions
	logger        *logger.Logger
	splitter      *Splitter
}

var _ PDFProcessor = (*Processor)(nil)

func NewProcessor(tempDir, outputDir string, flashcardSize models.PageDimensions, logger *logger.Logger) (*Processor, error) {
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	splitter, err := NewSplitter(outputDir, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create splitter: %w", err)
	}

	return &Processor{
		tempDir:       tempDir,
		outputDir:     outputDir,
		flashcardSize: flashcardSize,
		logger:        logger,
		splitter:      splitter,
	}, nil
}

func (p *Processor) ProcessPDF(ctx context.Context, pdfPath string) (ProcessingStats, error) {
	p.logger.Info("Processing PDF: %s", pdfPath)
	stats := ProcessingStats{PDFPath: pdfPath}

	doc, err := fitz.New(pdfPath)
	if err != nil {
		return stats, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer doc.Close()

	// Page numbers are zero indexed in the fitz package.
	// pageIndex -> index, and pageNum -> actual page number in pdf file
	for pageIndex := 0; pageIndex < doc.NumPage(); pageIndex++ {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
			pageNum := pageIndex + 1 // Convert to one-based page number for user-facing content
			bounds, err := doc.Bound(pageIndex)
			if err != nil {
				return stats, fmt.Errorf("failed to get bounds for page %d: %w", pageNum, err)
			}

			width := float64(bounds.Dx())
			height := float64(bounds.Dy())

			p.logger.Debug("Page %d dimensions: %.2f x %.2f", pageNum, width, height)

			isStandardSize := MatchesGoodnotesDimensions(width, height)

			isFlashcard := false
			if isStandardSize {
				text, err := doc.Text(pageIndex)
				if err != nil {
					p.logger.Printf("Warning: couldn't extract text from page %d: %v", pageNum, err)
					continue
				}

				isFlashcard = ContainsFlashcardMarkers(text)
			}

			if isFlashcard {
				p.logger.Printf("Found flashcard page: %d", pageNum)

				img, err := doc.Image(pageIndex)
				if err != nil {
					return stats, fmt.Errorf("failed to extract image for page %d: %w", pageNum, err)
				}

				// Save full image to temp directory first
				tempImagePath := filepath.Join(p.tempDir, fmt.Sprintf("%s_page%d.png",
					strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath)),
					pageNum))

				if err := saveImage(img, tempImagePath); err != nil {
					return stats, fmt.Errorf("failed to save temp image for page %d: %w", pageNum, err)
				}

				// Split the image into question and answer
				pair, err := p.splitter.SplitImage(tempImagePath)
				if err != nil {
					return stats, fmt.Errorf("failed to split image for page %d: %w", pageNum, err)
				}

				stats.ImagePairs = append(stats.ImagePairs, *pair)
				stats.FlashcardCount++
				p.logger.Debug("Split flashcard page %d into question and answer", pageNum)
			}
		}
	}

	return stats, nil
}

func MatchesGoodnotesDimensions(width, height float64) bool {
	widthMatch := abs(width-utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH) <= DimensionTolerance
	heightMatch := abs(height-utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT) <= DimensionTolerance

	rotatedWidthMatch := abs(width-utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT) <= DimensionTolerance
	rotatedHeightMatch := abs(height-utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH) <= DimensionTolerance

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
