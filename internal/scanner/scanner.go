package scanner

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type PDFFile struct {
	AbsolutePath string // Full path to the file
	RelativePath string // Path relative to root directory
}

type DirectoryScanner struct {
	logger *log.Logger
}

func New(logger *log.Logger) *DirectoryScanner {
	return &DirectoryScanner{
		logger: logger,
	}
}

func (s *DirectoryScanner) FindPDFs(ctx context.Context, rootDir string) ([]PDFFile, error) {
	var pdfs []PDFFile

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
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

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			s.logger.Printf("Warning: couldn't get relative path for %s: %v", path, err)
			relPath = filepath.Base(path)
		}

		pdfs = append(pdfs, PDFFile{
			AbsolutePath: path,
			RelativePath: relPath,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(pdfs) == 0 {
		return nil, fmt.Errorf("no PDF files found in %s or its subdirectories", rootDir)
	}

	return pdfs, nil
}
