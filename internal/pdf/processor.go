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

type ProcessingStats struct {
	PDFPath        string
	FlashcardCount int
	ImagePairs     []ImagePair
	PageNumbers    []int
}

type Processor struct {
	tempDir         string
	outputDir       string
	flashcardSize   models.PageDimensions
	skipMarkerCheck bool
	logger          *logger.Logger
	splitter        *Splitter
}

var _ PDFProcessor = (*Processor)(nil)

func NewProcessor(tempDir, outputDir string, flashcardSize models.PageDimensions, skipMarkerCheck bool, logger *logger.Logger) (*Processor, error) {
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
		tempDir:         tempDir,
		outputDir:       outputDir,
		flashcardSize:   flashcardSize,
		logger:          logger,
		splitter:        splitter,
		skipMarkerCheck: skipMarkerCheck,
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

	baseName := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))

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

			isFlashcardSize := p.MatchesFlashcardDimensions(width, height)

			isFlashcard := false
			if isFlashcardSize {
				if p.skipMarkerCheck {
					isFlashcard = true
					p.logger.Debug("Skipping marker check - treating page %d as flashcard based on dimensions", pageNum)
				} else {
					text, err := doc.Text(pageIndex)
					if err != nil {
						p.logger.Debug("Warning: couldn't extract text from page %d: %v", pageNum, err)
						continue
					}
					isFlashcard = ContainsFlashcardMarkers(text)
					if isFlashcard {
						p.logger.Debug("Found QUESTION/ANSWER markers in page %d", pageNum)
					}
				}
			}

			if isFlashcard {
				p.logger.Debug("Found flashcard page: %d", pageNum)

				img, err := doc.Image(pageIndex)
				if err != nil {
					return stats, fmt.Errorf("failed to extract image for page %d: %w", pageNum, err)
				}

				// Generate content hash from the entire image first
				fullHash, err := utils.GenerateImageHash(img)
				if err != nil {
					return stats, fmt.Errorf("failed to generate hash for page %d: %w", pageNum, err)
				}

				tempImagePath := filepath.Join(p.tempDir, fmt.Sprintf("%s_%s.png",
					baseName,
					fullHash[:8]))

				if err := saveImage(img, tempImagePath); err != nil {
					return stats, fmt.Errorf("failed to save temp image for page %d: %w", pageNum, err)
				}

				// Split the image into question and answer
				pair, err := p.splitter.SplitImageWithHash(tempImagePath, baseName, fullHash)
				if err != nil {
					return stats, fmt.Errorf("failed to split image for page %d: %w", pageNum, err)
				}

				stats.ImagePairs = append(stats.ImagePairs, *pair)
				stats.PageNumbers = append(stats.PageNumbers, pageNum) // Store actual page number
				stats.FlashcardCount++
				p.logger.Debug("Split flashcard page %d into question and answer (Hash:%s)", pageNum, fullHash)
			}
		}
	}

	return stats, nil
}

func (p *Processor) MatchesFlashcardDimensions(width, height float64) bool {
	targetWidth := p.flashcardSize.Width
	targetHeight := p.flashcardSize.Height

	p.logger.Debug("Comparing dimensions:")
	p.logger.Debug("  Current Page Dimension (WxH): %.2f x %.2f", width, height)
	p.logger.Debug("  Target Flashcard Page Dimension (WxH): %.2f x %.2f", targetWidth, targetHeight)
	p.logger.Debug("  Tolerance: %.1f", utils.DIMENSION_TOLERANCE)

	widthMatch := abs(width-targetWidth) <= utils.DIMENSION_TOLERANCE
	heightMatch := abs(height-targetHeight) <= utils.DIMENSION_TOLERANCE

	rotatedWidthMatch := abs(width-targetHeight) <= utils.DIMENSION_TOLERANCE
	rotatedHeightMatch := abs(height-targetWidth) <= utils.DIMENSION_TOLERANCE

	matches := (widthMatch && heightMatch) || (rotatedWidthMatch && rotatedHeightMatch)
	p.logger.Debug("  Dimensions match: %v", matches)

	return matches
}

func ContainsFlashcardMarkers(text string) bool {
	return strings.Contains(text, utils.QuestionKeyword) && strings.Contains(text, utils.AnswerKeyword)
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

func (p *Processor) ShouldCheckMarkers() bool {
	return !p.skipMarkerCheck
}

func (p *Processor) Cleanup() error {
	return os.RemoveAll(p.tempDir)
}
