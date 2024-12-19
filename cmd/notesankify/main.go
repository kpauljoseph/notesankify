package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	"github.com/kpauljoseph/notesankify/internal/anki"
	"github.com/kpauljoseph/notesankify/internal/config"
	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/internal/scanner"
	"github.com/kpauljoseph/notesankify/pkg/logger"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	pdfDir := flag.String("pdf-dir", "", "directory containing PDF files (overrides config)")
	outputDir := flag.String("output-dir", "flashcards", "directory to save processed flashcards")
	rootDeckName := flag.String("root-deck", "", "root deck name for organizing flashcards (optional)")
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	debug := flag.Bool("debug", false, "enable debug mode with trace logging")
	flag.Parse()

	logOptions := []logger.Option{
		logger.WithPrefix("[notesankify] "),
	}

	log := logger.New(logOptions...)
	log.SetVerbose(*verbose)

	if *debug {
		log.SetLevel(logger.LevelTrace)
	}

	if *verbose {
		log.Debug("Verbose logging enabled")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal("Error loading config: %v", err)
	}

	if *pdfDir != "" {
		cfg.PDFSourceDir = *pdfDir
	}

	if _, err := os.Stat(cfg.PDFSourceDir); os.IsNotExist(err) {
		log.Fatal("PDF directory does not exist: %s", cfg.PDFSourceDir)
	}

	processor, err := pdf.NewProcessor(
		filepath.Join(os.TempDir(), "notesankify-temp"),
		*outputDir,
		models.PageDimensions{
			Width:  cfg.FlashcardSize.Width,
			Height: cfg.FlashcardSize.Height,
		},
		log,
	)
	if err != nil {
		log.Fatal("Error initializing processor: %v", err)
	}
	defer processor.Cleanup()

	dirScanner := scanner.New(log)

	log.Info("Scanning directory: %s", cfg.PDFSourceDir)
	pdfs, err := dirScanner.FindPDFs(context.Background(), cfg.PDFSourceDir)
	if err != nil {
		log.Fatal("Error finding PDFs: %v", err)
	}

	log.Info("Found %d PDFs to process", len(pdfs))

	ankiService := anki.NewService(log)

	log.Debug("Checking Anki connection...")
	if err := ankiService.CheckConnection(); err != nil {
		log.Fatal("Anki connection error: %v", err)
	}
	log.Info("Successfully connected to Anki")

	var totalFlashcards int
	for _, pdf := range pdfs {
		stats, err := processor.ProcessPDF(context.Background(), pdf.AbsolutePath)
		if err != nil {
			log.Info("Error processing %s: %v", pdf.RelativePath, err)
			continue
		}

		if stats.FlashcardCount > 0 {
			deckName := anki.GetDeckNameFromPath(*rootDeckName, pdf.RelativePath)
			log.Info("Found %d flashcards in %s", stats.FlashcardCount, pdf.RelativePath)
			totalFlashcards += stats.FlashcardCount

			if err := ankiService.CreateDeck(deckName); err != nil {
				log.Info("Error creating deck %s: %v", deckName, err)
				continue
			}
			log.Debug("Created/Updated deck: %s", deckName)

			if err := ankiService.AddAllFlashcards(deckName, stats.ImagePairs); err != nil {
				log.Info("Error adding flashcards to deck %s: %v", deckName, err)
				continue
			}
		}
	}

	log.Info("Processing complete:")
	log.Info("- Total PDFs processed: %d", len(pdfs))
	log.Info("- Total flashcards found: %d", totalFlashcards)
	log.Info("- Flashcards saved to: %s", *outputDir)
}
