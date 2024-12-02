package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/kpauljoseph/notesankify/internal/config"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "path to config file")
	pdfDir := flag.String("pdf-dir", "", "directory containing PDF files")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Override PDF directory if provided via command line
	if *pdfDir != "" {
		cfg.PDFSourceDir = *pdfDir
	}

	// Ensure PDF directory exists
	if err := os.MkdirAll(cfg.PDFSourceDir, 0755); err != nil {
		log.Fatalf("Error creating PDF directory: %v", err)
	}

	// Initialize logger
	logger := log.New(os.Stdout, "[notesankify] ", log.LstdFlags)
	logger.Printf("Starting NotesAnkify...")
	logger.Printf("Monitoring directory: %s", filepath.Clean(cfg.PDFSourceDir))

	// TODO: Initialize and start services
}
