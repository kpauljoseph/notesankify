package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/kpauljoseph/notesankify/internal/anki"
	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/internal/scanner"
	"github.com/kpauljoseph/notesankify/pkg/logger"
	"github.com/kpauljoseph/notesankify/pkg/models"
	"github.com/kpauljoseph/notesankify/pkg/utils"
)

type NotesAnkifyGUI struct {
	// Core components
	window      fyne.Window
	log         *logger.Logger
	processor   *pdf.Processor
	scanner     *scanner.DirectoryScanner
	ankiService *anki.Service
	mutex       sync.Mutex

	// UI components
	dirEntry            *widget.Entry
	rootDeckEntry       *widget.Entry
	skipMarkersCheck    *widget.Check
	skipDimensionsCheck *widget.Check
	verboseCheck        *widget.Check
	progress            *widget.ProgressBarInfinite
	status              *widget.Label
}

func NewNotesAnkifyGUI() *NotesAnkifyGUI {
	log := logger.New(
		logger.WithPrefix("[notesankify-gui] "),
	)

	notesankifyApp := app.New()
	window := notesankifyApp.NewWindow("NotesAnkify")

	return &NotesAnkifyGUI{
		window:      window,
		log:         log,
		scanner:     scanner.New(log),
		ankiService: anki.NewService(log),
	}
}

func (gui *NotesAnkifyGUI) setupUI() {
	// Directory selection
	gui.dirEntry = widget.NewEntry()
	gui.dirEntry.SetPlaceHolder("Select PDF Directory")

	browseDirBtn := widget.NewButton("Browse", gui.handleBrowse)

	// Settings
	gui.skipMarkersCheck = widget.NewCheck("Skip QUESTION/ANSWER Markers", nil)
	gui.skipDimensionsCheck = widget.NewCheck("Skip Dimension Check", nil)
	gui.verboseCheck = widget.NewCheck("Verbose Logging", func(checked bool) {
		gui.log.SetVerbose(checked)
	})

	gui.rootDeckEntry = widget.NewEntry()
	gui.rootDeckEntry.SetPlaceHolder("Root Deck Name (Optional)")

	// Progress and status
	gui.progress = widget.NewProgressBarInfinite()
	gui.progress.Hide()
	gui.status = widget.NewLabel("Ready to process files...")

	// Process button
	processBtn := widget.NewButton("Process and Send to Anki", gui.handleProcess)

	// Layout
	dirContainer := container.NewBorder(
		nil, nil, nil, browseDirBtn,
		gui.dirEntry,
	)

	settingsContainer := container.NewVBox(
		widget.NewLabel("Settings"),
		gui.skipMarkersCheck,
		gui.skipDimensionsCheck,
		gui.verboseCheck,
		widget.NewLabel("Root Deck Name:"),
		gui.rootDeckEntry,
	)

	content := container.NewVBox(
		widget.NewLabelWithStyle("NotesAnkify", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Select PDF Directory:"),
		dirContainer,
		settingsContainer,
		processBtn,
		gui.progress,
		gui.status,
	)

	gui.window.SetContent(container.NewPadded(content))
	gui.window.Resize(fyne.NewSize(600, 400))
}

func (gui *NotesAnkifyGUI) handleBrowse() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, gui.window)
			return
		}
		if uri == nil {
			return
		}
		gui.dirEntry.SetText(uri.Path())
	}, gui.window)
}

func (gui *NotesAnkifyGUI) handleProcess() {
	if gui.dirEntry.Text == "" {
		dialog.ShowError(fmt.Errorf("please select a PDF directory"), gui.window)
		return
	}

	// Check Anki connection first
	if err := gui.ankiService.CheckConnection(); err != nil {
		dialog.ShowError(fmt.Errorf("Anki connection error: %v\nPlease make sure Anki is running and AnkiConnect is installed", err), gui.window)
		return
	}

	// Initialize processor
	var err error
	outputDir := filepath.Join(os.TempDir(), "notesankify-output")
	gui.processor, err = pdf.NewProcessor(
		filepath.Join(os.TempDir(), "notesankify-temp"),
		outputDir,
		models.PageDimensions{
			Width:  utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH,
			Height: utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT,
		},
		gui.skipMarkersCheck.Checked,
		gui.skipDimensionsCheck.Checked,
		gui.log,
	)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to initialize processor: %v", err), gui.window)
		return
	}

	gui.progress.Show()
	gui.updateStatus("Processing files...")

	// Process in goroutine to keep UI responsive
	go func() {
		defer func() {
			gui.mutex.Lock()
			gui.progress.Hide()
			gui.mutex.Unlock()
		}()

		report := &anki.ProcessingReport{
			StartTime: time.Now(),
		}

		pdfs, err := gui.scanner.FindPDFs(context.Background(), gui.dirEntry.Text)
		if err != nil {
			gui.showError(fmt.Sprintf("Error finding PDFs: %v", err))
			return
		}

		gui.updateStatus(fmt.Sprintf("Found %d PDFs to process", len(pdfs)))

		for _, pdf := range pdfs {
			report.ProcessedPDFs++
			gui.updateStatus(fmt.Sprintf("Processing: %s", pdf.RelativePath))

			stats, err := gui.processor.ProcessPDF(context.Background(), pdf.AbsolutePath)
			if err != nil {
				gui.showError(fmt.Sprintf("Error processing %s: %v", pdf.RelativePath, err))
				continue
			}

			if stats.FlashcardCount > 0 {
				deckName := anki.GetDeckNameFromPath(gui.rootDeckEntry.Text, pdf.RelativePath)
				report.TotalFlashcards += stats.FlashcardCount

				if err := gui.ankiService.CreateDeck(deckName); err != nil {
					gui.showError(fmt.Sprintf("Error creating deck %s: %v", deckName, err))
					continue
				}

				if err := gui.ankiService.AddAllFlashcards(deckName, stats.ImagePairs, report); err != nil {
					gui.showError(fmt.Sprintf("Error adding flashcards to deck %s: %v", deckName, err))
					continue
				}
			}
		}

		report.EndTime = time.Now()
		gui.showCompletionDialog(report)
	}()
}

func (gui *NotesAnkifyGUI) showError(message string) {
	gui.mutex.Lock()
	defer gui.mutex.Unlock()

	notification := fyne.NewNotification("Error", message)
	fyne.CurrentApp().SendNotification(notification)
	gui.status.SetText("Error occurred during processing")
}

func (gui *NotesAnkifyGUI) updateStatus(message string) {
	gui.mutex.Lock()
	defer gui.mutex.Unlock()

	gui.status.SetText(message)
}

func (gui *NotesAnkifyGUI) showCompletionDialog(report *anki.ProcessingReport) {
	gui.mutex.Lock()
	defer gui.mutex.Unlock()

	message := fmt.Sprintf(
		"Processing Complete!\n\n"+
			"PDFs Processed: %d\n"+
			"Total Flashcards: %d\n"+
			"Cards Added: %d\n"+
			"Cards Skipped: %d\n"+
			"Time Taken: %v",
		report.ProcessedPDFs,
		report.TotalFlashcards,
		report.AddedCount,
		report.SkippedCount,
		report.TimeTaken(),
	)

	dialog.ShowInformation("Processing Complete", message, gui.window)
	gui.status.SetText("Ready to process files...")
}

func (gui *NotesAnkifyGUI) Run() {
	gui.setupUI()
	gui.window.ShowAndRun()
}

func main() {
	gui := NewNotesAnkifyGUI()
	gui.Run()
}
