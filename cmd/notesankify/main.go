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
	"github.com/kpauljoseph/notesankify/pkg/utils"
	"os"
	"path/filepath"
	"time"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	pdfDir := flag.String("pdf-dir", "", "directory containing PDF files (overrides config)")
	outputDir := flag.String("output-dir", getDefaultOutputDir(), "directory to save processed flashcards")
	rootDeckName := flag.String("root-deck", "", "root deck name for organizing flashcards (optional)")
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	debug := flag.Bool("debug", false, "enable debug mode with trace logging")
	width := flag.Float64("width", 0.0, "custom flashcard width (defaults to Goodnotes standard if not specified)")
	height := flag.Float64("height", 0.0, "custom flashcard height (defaults to Goodnotes standard if not specified)")
	disableMarkerCheck := flag.Bool("no-markers", false, "disable checking for QUESTION/ANSWER markers in pages")
	disableDimensionCheck := flag.Bool("no-dimensions", false, "disable checking page dimensions")

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

	// Set up dimensions
	dimensions := models.PageDimensions{
		Width:  utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH,
		Height: utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT,
	}

	if *width > 0 && *height > 0 {
		dimensions.Width = *width
		dimensions.Height = *height
		log.Debug("Using custom dimensions: %.2f x %.2f", *width, *height)
	} else {
		log.Debug("Using default standard size dimensions: %.2f x %.2f",
			dimensions.Width, dimensions.Height)
	}

	if _, err := os.Stat(cfg.PDFSourceDir); os.IsNotExist(err) {
		log.Fatal("PDF directory does not exist: %s", cfg.PDFSourceDir)
	}

	processorConfig := pdf.ProcessorConfig{
		TempDir:    filepath.Join(os.TempDir(), "notesankify-temp"),
		OutputDir:  *outputDir,
		Dimensions: dimensions,
		ProcessingOptions: pdf.ProcessingOptions{
			CheckDimensions: !*disableDimensionCheck, // Enabled by default
			CheckMarkers:    !*disableMarkerCheck,    // Enabled by default
		},
		Logger: log,
	}

	processor, err := pdf.NewProcessor(processorConfig)
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

	// Initialize and check Anki connection
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

func getDefaultOutputDir() string {
	tmpDir, err := os.MkdirTemp("", "notesankify-output-*")
	if err != nil {
		// If we can't create a temp directory, fall back to local directory
		return "flashcards"
	}
	return tmpDir
}
