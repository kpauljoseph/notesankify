package main

import (
	"context"
	"flag"
	"github.com/kpauljoseph/notesankify/internal/anki"
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
	outputDir := flag.String("output-dir", "flashcards", "directory to save processed flashcards")
	ankiDeckName := flag.String("deck-name", "", "Deck name in Anki")
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

	if *ankiDeckName != "" {
		cfg.AnkiDeckName = *ankiDeckName
	}

	processor, err := pdf.NewProcessor(
		filepath.Join(os.TempDir(), "notesankify-temp"),
		*outputDir,
		models.PageDimensions{
			Width:  cfg.FlashcardSize.Width,
			Height: cfg.FlashcardSize.Height,
		},
		logger,
	)
	if err != nil {
		logger.Fatalf("Error initializing processor: %v", err)
	}
	defer processor.Cleanup()

	dirScanner := scanner.New(logger)

	logger.Printf("Scanning directory: %s", cfg.PDFSourceDir)
	pdfs, err := dirScanner.FindPDFs(context.Background(), cfg.PDFSourceDir)
	if err != nil {
		logger.Fatalf("Error finding PDFs: %v", err)
	}

	var totalFlashcards int
	logger.Printf("Found %d PDFs to process", len(pdfs))

	ankiService := anki.NewService(logger)

	logger.Printf("Checking Anki connection...")
	if err := ankiService.CheckConnection(); err != nil {
		logger.Fatalf("Anki connection error: %v", err)
	}
	logger.Printf("Successfully connected to Anki")

	if err := ankiService.CreateDeck(cfg.AnkiDeckName); err != nil {
		logger.Fatalf("Error creating Anki deck: %v", err)
	}

	for _, pdfPath := range pdfs {
		stats, err := processor.ProcessPDF(context.Background(), pdfPath)
		if err != nil {
			logger.Printf("Error processing %s: %v", pdfPath, err)
			continue
		}
		if stats.FlashcardCount > 0 {
			logger.Printf("Found %d flashcards in %s", stats.FlashcardCount, filepath.Base(pdfPath))
			totalFlashcards += stats.FlashcardCount

			if err := ankiService.AddAllFlashcards(cfg.AnkiDeckName, stats.ImagePairs); err != nil {
				logger.Printf("Error adding flashcards to Anki: %v", err)
			}

		}
	}

	logger.Printf("Processing complete:")
	logger.Printf("- Total PDFs processed: %d", len(pdfs))
	logger.Printf("- Total flashcards found: %d", totalFlashcards)
	logger.Printf("- Flashcards saved to: %s", *outputDir)
}
