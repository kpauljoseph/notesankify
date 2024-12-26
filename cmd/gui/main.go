package main

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2/theme"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
	logFileName string

	// UI components
	dirEntry             *widget.Entry
	rootDeckEntry        *widget.Entry
	skipMarkersCheck     *widget.Check
	skipDimensionsCheck  *widget.Check
	checkMarkersCheck    *widget.Check
	checkDimensionsCheck *widget.Check
	verboseCheck         *widget.Check
	widthEntry           *widget.Entry
	heightEntry          *widget.Entry
	dimensionsContainer  *fyne.Container
	progress             *widget.ProgressBarInfinite
	status               *widget.Label
}

func NewNotesAnkifyGUI() *NotesAnkifyGUI {
	log, logFileName, err := setupLogging()
	if err != nil {
		// If logging setup fails, fall back to basic logger
		log = logger.New(
			logger.WithPrefix("[notesankify-gui] "),
		)
		fmt.Printf("Warning: Failed to set up logging: %v\n", err)
	}

	notesankifyApp := app.New()
	window := notesankifyApp.NewWindow("NotesAnkify")

	return &NotesAnkifyGUI{
		window:      window,
		log:         log,
		scanner:     scanner.New(log),
		ankiService: anki.NewService(log),
		logFileName: logFileName,
	}
}

func (gui *NotesAnkifyGUI) resetDimensions() {
	gui.widthEntry.SetText(fmt.Sprintf("%.2f", utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH))
	gui.heightEntry.SetText(fmt.Sprintf("%.2f", utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT))
}

func (gui *NotesAnkifyGUI) setupUI() {
	gui.dirEntry = widget.NewEntry()
	gui.dirEntry.SetPlaceHolder("Select PDF Directory")

	browseDirBtn := widget.NewButton("Browse", gui.handleBrowse)
	browseDirBtn.Importance = widget.HighImportance

	dirContainer := container.NewBorder(
		nil, nil, nil, browseDirBtn,
		gui.dirEntry,
	)

	// Settings
	gui.checkMarkersCheck = widget.NewCheck("Check for QUESTION/ANSWER markers", func(checked bool) {
		gui.skipMarkersCheck = &widget.Check{Checked: !checked}
	})
	gui.checkMarkersCheck.SetChecked(true)

	// Dimension controls
	gui.widthEntry = widget.NewEntry()
	gui.heightEntry = widget.NewEntry()
	gui.resetDimensions()

	resetDimensionsBtn := widget.NewButton("Reset to Default", func() {
		gui.resetDimensions()
	})

	dimensionsForm := container.NewHBox(
		container.NewGridWithColumns(4,
			widget.NewLabel("Width:"),
			gui.widthEntry,
			widget.NewLabel("Height:"),
			gui.heightEntry,
		),
		resetDimensionsBtn,
	)

	gui.checkDimensionsCheck = widget.NewCheck("Check page dimensions", func(checked bool) {
		gui.skipDimensionsCheck = &widget.Check{Checked: !checked}
		if checked {
			gui.widthEntry.Enable()
			gui.heightEntry.Enable()
			resetDimensionsBtn.Enable()
		} else {
			gui.widthEntry.Disable()
			gui.heightEntry.Disable()
			resetDimensionsBtn.Disable()
		}
	})
	gui.checkDimensionsCheck.SetChecked(true)

	gui.dimensionsContainer = container.NewVBox(
		gui.checkDimensionsCheck,
		dimensionsForm,
	)

	gui.verboseCheck = widget.NewCheck("Verbose Logging", func(checked bool) {
		gui.log.SetVerbose(checked)
	})

	gui.rootDeckEntry = widget.NewEntry()
	gui.rootDeckEntry.SetPlaceHolder("Root Deck Name")

	gui.progress = widget.NewProgressBarInfinite()
	gui.progress.Hide()
	gui.status = widget.NewLabel("Ready to process files...")

	processBtn := widget.NewButton("Process and Send to Anki", gui.handleProcess)
	processBtn.Importance = widget.HighImportance

	pdfSourceInfo := createInfoSection("PDF Source",
		"Select the directory containing all your PDF files containing notes/flashcards from which you want to extract flashcards and send to Anki.",
		container.NewVBox(dirContainer))

	markerInfo := createInfoSection("Marker Detection",
		"When enabled, only extract flashcards from pages containing the term 'QUESTION' AND 'ANSWER'.\n"+
			"Disable this if your flashcards don't use these keywords.\n"+
			"The standard flashcard template contains these markers in top(QUESTION) and bottom(ANSWER) halves.\n"+
			"If this option is disabled, then the tool will cut the page in half, and consider top half as question,\n"+
			"and bottom half as the answer for a given page(according to appropriate dimension check).",
		container.NewVBox(gui.checkMarkersCheck))

	deckInfo := createInfoSection("Deck Organization",
		"Specify a root deck name to organize your flashcards. Default to folder name if nothing is provided.\n"+
			"Example: 'MyStudyDeck' will create decks like 'MyStudyDeck::SubFolder::Topic'",
		container.NewVBox(
			widget.NewLabel("Root Deck Name:"),
			gui.rootDeckEntry,
		))

	dimensionInfo := createInfoSection("Page Dimensions",
		"When enabled, only extracts flashcard pages if the PDF file contains pages matching the specified dimensions.\n"+
			"Default dimensions are set to standard flashcard size.\n"+
			"Modify the this if your flashcards use different dimensions.\n"+
			"When disabled, all files of varying dimensions will be considered for flashcard processing(based on marker check).\n"+
			"This will work along with the QUESTION/ANSWER marker check if it is also enabled.",
		gui.dimensionsContainer)

	loggingInfo := createInfoSection("Logging",
		"Enable detailed logging for troubleshooting.\nShows additional information about the processing steps.",
		container.NewVBox(gui.verboseCheck))

	mainContent := container.NewVBox(
		widget.NewLabelWithStyle("NotesAnkify", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		pdfSourceInfo,
		widget.NewLabel(""),
		widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		deckInfo,
		widget.NewLabel(""),
		markerInfo,
		widget.NewLabel(""),
		dimensionInfo,
		widget.NewLabel(""),
		widget.NewLabelWithStyle("Additional Options", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		loggingInfo,
		widget.NewLabel(""),
		processBtn,
		gui.progress,
		gui.status,
	)

	scrollContainer := container.NewScroll(mainContent)

	paddedContainer := container.NewPadded(scrollContainer)

	gui.window.SetContent(paddedContainer)

	gui.window.Resize(fyne.NewSize(700, 800))
	gui.window.SetFixedSize(false)
}

func createInfoSection(title, tooltip string, content fyne.CanvasObject) *widget.Card {
	infoIcon := widget.NewIcon(theme.InfoIcon())
	tooltipLabel := widget.NewLabelWithStyle(tooltip, fyne.TextAlignLeading, fyne.TextStyle{})
	tooltipLabel.Wrapping = fyne.TextWrapWord

	header := container.NewHBox(
		widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		infoIcon,
	)

	return widget.NewCard(
		"",
		"",
		container.NewVBox(
			header,
			tooltipLabel,
			content,
		),
	)
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

	// Validate dimensions if checking is enabled
	if gui.checkDimensionsCheck.Checked {
		width, err := strconv.ParseFloat(gui.widthEntry.Text, 64)
		if err != nil {
			dialog.ShowError(fmt.Errorf("invalid width value"), gui.window)
			return
		}
		height, err := strconv.ParseFloat(gui.heightEntry.Text, 64)
		if err != nil {
			dialog.ShowError(fmt.Errorf("invalid height value"), gui.window)
			return
		}
		if width <= 0 || height <= 0 {
			dialog.ShowError(fmt.Errorf("dimensions must be greater than 0"), gui.window)
			return
		}
	}

	// Check Anki connection first
	if err := gui.ankiService.CheckConnection(); err != nil {
		dialog.ShowError(fmt.Errorf("Anki connection error: %v\nPlease make sure Anki is running and AnkiConnect is installed", err), gui.window)
		return
	}

	var err error
	outputDir := filepath.Join(os.TempDir(), "notesankify-output")

	width := utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH
	height := utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT
	if gui.checkDimensionsCheck.Checked {
		width, _ = strconv.ParseFloat(gui.widthEntry.Text, 64)
		height, _ = strconv.ParseFloat(gui.heightEntry.Text, 64)
	}

	gui.processor, err = pdf.NewProcessor(
		filepath.Join(os.TempDir(), "notesankify-temp"),
		outputDir,
		models.PageDimensions{
			Width:  width,
			Height: height,
		},
		!gui.checkMarkersCheck.Checked,    // Invert the logic for skip flags
		!gui.checkDimensionsCheck.Checked, // Invert the logic for skip flags
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
			"Time Taken: %v\n"+
			"Logs saved in: %s",
		report.ProcessedPDFs,
		report.TotalFlashcards,
		report.AddedCount,
		report.SkippedCount,
		report.TimeTaken(),
		gui.logFileName,
	)

	customDialog := dialog.NewCustom("Processing Complete", "Close", container.NewVBox(
		widget.NewLabel(message),
		widget.NewButton("Open Log File", func() {
			// Open log file in default text editor
			var cmd *exec.Cmd
			switch runtime.GOOS {
			case "windows":
				cmd = exec.Command("cmd", "/c", "start", gui.logFileName)
			case "darwin":
				cmd = exec.Command("open", gui.logFileName)
			default: // Linux and other Unix-like systems
				cmd = exec.Command("xdg-open", gui.logFileName)
			}
			if err := cmd.Run(); err != nil {
				dialog.ShowError(fmt.Errorf("failed to open log file: %v", err), gui.window)
			}
		}),
	), gui.window)

	customDialog.Show()

	gui.status.SetText("Ready to process files...")
}
func setupLogging() (*logger.Logger, string, error) {
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create logs directory: %v", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFileName := filepath.Join(logsDir, fmt.Sprintf("notesankify_%s.log", timestamp))

	logFile, err := os.Create(logFileName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create log file: %v", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)

	log := logger.New(
		logger.WithPrefix("[notesankify-gui] "),
		logger.WithOutput(multiWriter),
	)

	return log, logFileName, nil
}

func (gui *NotesAnkifyGUI) Run() {
	gui.setupUI()
	gui.window.ShowAndRun()
}

func main() {
	gui := NewNotesAnkifyGUI()
	gui.Run()
}
