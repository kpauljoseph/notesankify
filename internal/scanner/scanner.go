package scanner

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type Stats struct {
	PDFCount int
}

type DirectoryScanner struct {
	logger *log.Logger
}

func New(logger *log.Logger) *DirectoryScanner {
	return &DirectoryScanner{
		logger: logger,
	}
}

func (s *DirectoryScanner) FindPDFs(ctx context.Context, dir string) ([]string, error) {
	var pdfs []string

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

		pdfs = append(pdfs, path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(pdfs) == 0 {
		return nil, fmt.Errorf("no PDF files found in %s or its subdirectories", dir)
	}

	return pdfs, nil
}
