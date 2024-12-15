package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/kpauljoseph/notesankify/internal/config"
	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/internal/scanner"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	pdfDir := flag.String("pdf-dir", "", "directory containing PDF files (overrides config)")
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	flag.Parse()

	logger := log.New(os.Stdout, "[notesankify] ", log.LstdFlags)
	if *verbose {
		logger.Printf("Verbose logging enabled")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatalf("Error loading config: %v", err)
	}

	if *pdfDir != "" {
		cfg.PDFSourceDir = *pdfDir
	}

	if _, err := os.Stat(cfg.PDFSourceDir); os.IsNotExist(err) {
		logger.Fatalf("PDF directory does not exist: %s", cfg.PDFSourceDir)
	}

	processor, err := pdf.NewProcessor(
		filepath.Join(os.TempDir(), "notesankify"),
		models.PageDimensions{
			Width:  cfg.FlashcardSize.Width,
			Height: cfg.FlashcardSize.Height,
		},
	)
	if err != nil {
		logger.Fatalf("Error initializing processor: %v", err)
	}
	defer processor.Cleanup()

	dirScanner := scanner.New(processor, logger)

	logger.Printf("Processing directory: %s", cfg.PDFSourceDir)

	stats, err := dirScanner.ScanDirectory(context.Background(), cfg.PDFSourceDir)
	if err != nil {
		logger.Fatalf("Error processing directory: %v", err)
	}

	logger.Printf("Processing complete:")
	logger.Printf("- Total PDFs processed: %d", stats.PDFCount)
	logger.Printf("- Total flashcards found: %d", stats.FlashcardCount)
}
