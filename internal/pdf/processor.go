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

type ProcessorConfig struct {
	TempDir    string
	OutputDir  string
	Dimensions models.PageDimensions
	ProcessingOptions
	Logger *logger.Logger
}

type ProcessingOptions struct {
	CheckDimensions bool // if true, only process pages matching dimensions
	CheckMarkers    bool // if true, only process pages with QUESTION/ANSWER markers
}

type Processor struct {
	config   ProcessorConfig
	splitter *Splitter
}

var _ PDFProcessor = (*Processor)(nil)

func NewProcessor(config ProcessorConfig) (*Processor, error) {
	if err := os.MkdirAll(config.TempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	splitter, err := NewSplitter(config.OutputDir, config.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create splitter: %w", err)
	}

	return &Processor{
		config:   config,
		splitter: splitter,
	}, nil
}

func (p *Processor) ProcessPDF(ctx context.Context, pdfPath string) (ProcessingStats, error) {
	p.config.Logger.Info("Processing PDF: %s", pdfPath)
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
			if shouldProcessPage, err := p.shouldProcessPage(doc, pageIndex); err != nil {
				p.config.Logger.Debug("Error checking page %d: %v", pageNum, err)
				continue
			} else if !shouldProcessPage {
				continue
			}

			// Process the page as a flashcard
			p.config.Logger.Debug("Processing page %d as flashcard", pageNum)
			if err := p.processPage(doc, pageIndex, baseName, &stats); err != nil {
				p.config.Logger.Debug("Error processing page %d: %v", pageNum, err)
				continue
			}
		}
	}

	return stats, nil
}

func (p *Processor) shouldProcessPage(doc *fitz.Document, pageIndex int) (bool, error) {
	pageNum := pageIndex + 1

	// Check dimensions if required
	if p.config.CheckDimensions {
		bounds, err := doc.Bound(pageIndex)
		if err != nil {
			return false, fmt.Errorf("failed to get bounds: %w", err)
		}

		width := float64(bounds.Dx())
		height := float64(bounds.Dy())

		p.config.Logger.Debug("Page %d dimensions: %.2f x %.2f", pageNum, width, height)
		if !p.MatchesDimensions(width, height) {
			p.config.Logger.Debug("Page %d does not match required dimensions", pageNum)
			return false, nil
		}
	}

	// Check markers if required
	if p.config.CheckMarkers {
		text, err := doc.Text(pageIndex)
		if err != nil {
			return false, fmt.Errorf("failed to extract text: %w", err)
		}

		if !ContainsFlashcardMarkers(text) {
			p.config.Logger.Debug("Page %d does not contain required markers", pageNum)
			return false, nil
		}
	}

	return true, nil
}

func (p *Processor) processPage(doc *fitz.Document, pageIndex int, baseName string, stats *ProcessingStats) error {
	pageNum := pageIndex + 1

	img, err := doc.Image(pageIndex)
	if err != nil {
		return fmt.Errorf("failed to extract image: %w", err)
	}

	// Generate content hash
	fullHash, err := utils.GenerateImageHash(img)
	if err != nil {
		return fmt.Errorf("failed to generate hash: %w", err)
	}

	// Save temporary image
	tempImagePath := filepath.Join(p.config.TempDir, fmt.Sprintf("%s_%s.png", baseName, fullHash[:8]))
	if err := saveImage(img, tempImagePath); err != nil {
		return fmt.Errorf("failed to save temp image: %w", err)
	}

	// Split into question and answer
	pair, err := p.splitter.SplitImageWithHash(tempImagePath, baseName, fullHash)
	if err != nil {
		return fmt.Errorf("failed to split image: %w", err)
	}

	stats.ImagePairs = append(stats.ImagePairs, *pair)
	stats.PageNumbers = append(stats.PageNumbers, pageNum) // Store actual page number
	stats.FlashcardCount++

	p.config.Logger.Debug("Successfully processed page %d (Hash:%s)", pageNum, fullHash)
	return nil
}

func (p *Processor) MatchesDimensions(width, height float64) bool {
	targetWidth := p.config.Dimensions.Width
	targetHeight := p.config.Dimensions.Height

	p.config.Logger.Debug("Comparing dimensions:")
	p.config.Logger.Debug("  Current: %.2f x %.2f", width, height)
	p.config.Logger.Debug("  Target:  %.2f x %.2f", targetWidth, targetHeight)
	p.config.Logger.Debug("  Tolerance: %.1f", utils.DIMENSION_TOLERANCE)

	// Check both normal and rotated orientations
	return (abs(width-targetWidth) <= utils.DIMENSION_TOLERANCE &&
		abs(height-targetHeight) <= utils.DIMENSION_TOLERANCE) ||
		(abs(width-targetHeight) <= utils.DIMENSION_TOLERANCE &&
			abs(height-targetWidth) <= utils.DIMENSION_TOLERANCE)
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
	return p.config.CheckMarkers
}

func (p *Processor) ShouldCheckDimensions() bool {
	return p.config.CheckDimensions
}

func (p *Processor) Cleanup() error {
	return os.RemoveAll(p.config.TempDir)
}
