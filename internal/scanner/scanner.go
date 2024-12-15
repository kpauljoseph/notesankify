package scanner

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kpauljoseph/notesankify/internal/pdf"
)

type Stats struct {
	PDFCount       int
	FlashcardCount int
}

type DirectoryScanner struct {
	processor pdf.PDFProcessor
	logger    *log.Logger
}

func New(processor pdf.PDFProcessor, logger *log.Logger) *DirectoryScanner {
	return &DirectoryScanner{
		processor: processor,
		logger:    logger,
	}
}

func (s *DirectoryScanner) ScanDirectory(ctx context.Context, dir string) (Stats, error) {
	var stats Stats

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		if info.IsDir() {
			s.logger.Printf("Scanning directory: %s", path)
			return nil
		}

		if filepath.Ext(path) != ".pdf" {
			return nil
		}

		stats.PDFCount++
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			relPath = path
		}
		s.logger.Printf("Processing PDF (%d): %s", stats.PDFCount, relPath)

		pages, err := s.processor.ProcessPDF(ctx, path)
		if err != nil {
			if err == context.Canceled {
				return err
			}
			s.logger.Printf("Error processing %s: %v", relPath, err)
			return nil
		}

		if len(pages) > 0 {
			s.logger.Printf("Found %d flashcard pages in %s:", len(pages), relPath)
			for i, page := range pages {
				s.logger.Printf("  Page %d: original page %d", i+1, page.PageNum)
			}
			stats.FlashcardCount += len(pages)
		}

		return nil
	})

	if err != nil {
		return stats, err
	}

	if stats.PDFCount == 0 {
		return stats, fmt.Errorf("no PDF files found in %s or its subdirectories", dir)
	}

	return stats, nil
}
