package main

import (
	"context"
	"flag"
	"github.com/kpauljoseph/notesankify/internal/anki"
	"github.com/kpauljoseph/notesankify/internal/config"
	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/internal/scanner"
	"github.com/kpauljoseph/notesankify/pkg/logger"
	"github.com/kpauljoseph/notesankify/pkg/models"
	"os"
	"path/filepath"
	"time"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	pdfDir := flag.String("pdf-dir", "", "directory containing PDF files (overrides config)")
	outputDir := flag.String("output-dir", "flashcards", "directory to save processed flashcards")
	rootDeckName := flag.String("root-deck", "", "root deck name for organizing flashcards (optional)")
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	debug := flag.Bool("debug", false, "enable debug mode with trace logging")
	flag.Parse()

	report := &anki.ProcessingReport{
		StartTime: time.Now(),
	}

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

	// TODO: Add cleanup to account for termination case
	//ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()
	//sigChan := make(chan os.Signal, 1)
	//signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	//go func() {
	//	<-sigChan
	//	log.Info("Received interrupt signal, starting cleanup...")
	//	cancel()
	//}()

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

	//cleanUp := func() {
	//	log.Info("Running cleanup operations...")
	//	err := processor.Cleanup()
	//	if err != nil {
	//		log.Info("Error during cleanup: %v", err)
	//	}
	//	log.Info("Cleanup completed")
	//}
	//defer cleanUp()

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

	for _, pdf := range pdfs {
		report.ProcessedPDFs++
		stats, err := processor.ProcessPDF(context.Background(), pdf.AbsolutePath)
		if err != nil {
			log.Info("Error processing %s: %v", pdf.RelativePath, err)
			continue
		}

		if stats.FlashcardCount > 0 {
			deckName := anki.GetDeckNameFromPath(*rootDeckName, pdf.RelativePath)
			log.Info("Found %d flashcards in %s", stats.FlashcardCount, pdf.RelativePath)
			report.TotalFlashcards += stats.FlashcardCount

			if err := ankiService.CreateDeck(deckName); err != nil {
				log.Info("Error creating deck %s: %v", deckName, err)
				continue
			}
			log.Debug("Created/Updated deck: %s", deckName)

			if err := ankiService.AddAllFlashcards(deckName, stats.ImagePairs, report); err != nil {
				log.Info("Error adding flashcards to deck %s: %v", deckName, err)
				continue
			}
		}
	}

	log.Info("Processing complete:")
	log.Info("- Total PDFs processed: %d", len(pdfs))
	log.Info("- Total flashcards found: %d", report.TotalFlashcards)
	log.Info("- Flashcards saved to: %s", *outputDir)

	report.EndTime = time.Now()
	report.Print(log)
}
