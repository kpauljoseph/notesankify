package main

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/kpauljoseph/notesankify/pkg/utils"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/kpauljoseph/notesankify/internal/anki"
	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/internal/scanner"
	"github.com/kpauljoseph/notesankify/pkg/logger"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

type ProcessingMode int

const (
	ModeProcessAll ProcessingMode = iota
	ModeOnlyMarkers
	ModeOnlyDimensions
	ModeBoth
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

	// Processing settings
	processingMode ProcessingMode
	dimensions     models.PageDimensions

	// UI components
	dirEntry       *widget.Entry
	rootDeckEntry  *widget.Entry
	modeSelect     *widget.Select
	widthEntry     *widget.Entry
	heightEntry    *widget.Entry
	outputDirEntry *widget.Entry
	dimContainer   *fyne.Container
	verboseCheck   *widget.Check
	progress       *widget.ProgressBarInfinite
	status         *widget.Label
}

func NewNotesAnkifyGUI() *NotesAnkifyGUI {
	log, logFileName, err := setupLogging()
	if err != nil {
		log = logger.New(logger.WithPrefix("[notesankify-gui] "))
		fmt.Printf("Warning: Failed to set up logging: %v\n", err)
	}

	notesankifyApp := app.New()
	window := notesankifyApp.NewWindow("NotesAnkify")
	if bundledIcon, err := fyne.LoadResourceFromPath("assets/icons/png/icon-128.png"); err == nil {
		notesankifyApp.SetIcon(bundledIcon)
		window.SetIcon(bundledIcon)
	}

	dimensions := models.PageDimensions{
		Width:  utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH,
		Height: utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT,
	}

	return &NotesAnkifyGUI{
		window:         window,
		log:            log,
		scanner:        scanner.New(log),
		ankiService:    anki.NewService(log),
		logFileName:    logFileName,
		dimensions:     dimensions,
		processingMode: ModeBoth, // Start with most strict mode
	}
}

func (gui *NotesAnkifyGUI) resetDimensions() {
	gui.dimensions = models.PageDimensions{
		Width:  utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH,
		Height: utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT,
	}
	gui.widthEntry.SetText(fmt.Sprintf("%.2f", gui.dimensions.Width))
	gui.heightEntry.SetText(fmt.Sprintf("%.2f", gui.dimensions.Height))
}

func (gui *NotesAnkifyGUI) setupUI() {
	// Directory selection
	gui.dirEntry = widget.NewEntry()
	gui.dirEntry.SetPlaceHolder("Select PDF Directory")

	browseDirBtn := widget.NewButton("Browse", gui.handleBrowse)
	browseDirBtn.Importance = widget.HighImportance

	dirContainer := container.NewBorder(
		nil, nil, nil, browseDirBtn,
		gui.dirEntry,
	)

	// Root deck name
	gui.rootDeckEntry = widget.NewEntry()
	gui.rootDeckEntry.SetPlaceHolder("Root Deck Name")

	// Processing mode selection
	gui.modeSelect = widget.NewSelect(
		[]string{
			"Pages with QUESTION/ANSWER Markers and Matching Dimensions",
			"Only Pages with QUESTION/ANSWER Markers",
			"Only Pages Matching Dimensions",
			"Process All Pages",
		},
		nil,
	)
	gui.modeSelect.SetSelected("Pages with QUESTION/ANSWER Markers and Matching Dimensions")

	gui.modeSelect.OnChanged = gui.handleModeChange

	// Dimension controls
	gui.widthEntry = widget.NewEntry()
	gui.heightEntry = widget.NewEntry()
	gui.resetDimensions() // Set default dimensions

	resetDimensionsBtn := widget.NewButton("Reset to Default", gui.resetDimensions)

	dimensionsForm := container.NewGridWithColumns(2,
		widget.NewLabel("Width:"),
		gui.widthEntry,
		widget.NewLabel("Height:"),
		gui.heightEntry,
	)

	gui.dimContainer = container.NewHBox(
		dimensionsForm,
		resetDimensionsBtn,
	)

	// Additional settings
	// Optional output directory
	gui.outputDirEntry = widget.NewEntry()
	gui.outputDirEntry.SetText(utils.GetDefaultOutputDir())
	gui.outputDirEntry.SetPlaceHolder("Output Directory (Optional - defaults to temporary directory)")
	browseOutputDirBtn := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, gui.window)
				return
			}
			if uri == nil {
				return
			}
			gui.outputDirEntry.SetText(uri.Path())
		}, gui.window)
	})
	outputDirContainer := container.NewBorder(
		nil, nil, nil, browseOutputDirBtn,
		gui.outputDirEntry,
	)

	gui.verboseCheck = widget.NewCheck("Verbose Logging", func(checked bool) {
		gui.log.SetVerbose(checked)
	})

	// Progress indicator
	gui.progress = widget.NewProgressBarInfinite()
	gui.progress.Hide()
	gui.status = widget.NewLabel("Ready to process files...")

	// Process button
	processBtn := widget.NewButton("Process and Send to Anki", gui.handleProcess)
	processBtn.Importance = widget.HighImportance

	// Create info sections
	pdfSourceInfo := gui.createInfoSection("PDF Source",
		"Select the directory containing your PDF files for processing into Anki flashcards.",
		container.NewVBox(dirContainer))

	deckInfo := gui.createInfoSection("Root Deck",
		"Specify a root deck name to organize your flashcards.\n"+
			"If not provided, folder names will be used for deck organization.\n"+
			"Example: 'MyStudies' will create 'MyStudies::Math::Calculus'",
		container.NewVBox(gui.rootDeckEntry))

	processingInfo := gui.createInfoSection("Processing Mode",
		"Choose how to identify flashcards in your PDF files:",
		container.NewVBox(
			container.NewVBox(
				widget.NewRichTextFromMarkdown(
					"• **Pages with QUESTION/ANSWER Markers and Matching Dimensions**:\n\n"+
						"	The Flashcard page must have QUESTION/ANSWER markers and match given dimensions\n\n\n\n"+
						"• **Only Pages with QUESTION/ANSWER Markers**:\n\n"+
						"	The Flashcard page must have uppercase QUESTION/ANSWER text in the page\n\n\n\n"+
						"• **Only Pages Matching Dimensions**:\n\n "+
						"	The Flashcard page must match specified dimensions\n\n\n\n"+
						"• **Process All Pages**:\n\n "+
						"	Split every PDF page into two halves top->question bottom->answer",
				),
			),
			gui.modeSelect,
			gui.dimContainer,
		))

	settingsInfo := gui.createInfoSection("Additional Settings",
		"Enable verbose logging to see detailed processing information.",
		container.NewVBox(gui.verboseCheck))
	outputDirInfo := gui.createInfoSection("Output Directory",
		"Optional: Specify where to save the processed flashcard images.\n"+
			"If not specified, a temporary directory will be used.\n"+
			"Useful for debugging or manual inspection of processed cards.",
		container.NewVBox(outputDirContainer))

	content := container.NewVBox(
		widget.NewLabelWithStyle("NotesAnkify", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		pdfSourceInfo,
		deckInfo,
		processingInfo,
		outputDirInfo,
		settingsInfo,
		processBtn,
		gui.progress,
		gui.status,
	)

	scrollContainer := container.NewScroll(content)
	paddedContainer := container.NewPadded(scrollContainer)

	gui.window.SetContent(paddedContainer)

	gui.window.Resize(fyne.NewSize(700, 800))
	gui.window.SetFixedSize(false)

	// Initial state
	gui.handleModeChange(gui.modeSelect.Selected)
}

func (gui *NotesAnkifyGUI) createInfoSection(title, tooltip string, content fyne.CanvasObject) *widget.Card {
	infoBtn := widget.NewButtonWithIcon("", theme.InfoIcon(), nil)
	infoBtn.Importance = widget.LowImportance

	helpText := widget.NewRichTextFromMarkdown(tooltip)
	helpText.Wrapping = fyne.TextWrapWord

	helpContainer := container.NewVBox(
		container.NewPadded(helpText),
	)

	infoBtn.OnTapped = func() {
		d := dialog.NewCustom(
			title+" - Help",
			"Close",
			helpContainer,
			gui.window,
		)
		d.Resize(fyne.NewSize(500, 0))
		d.Show()
	}

	header := container.NewHBox(
		widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		infoBtn,
	)

	card := widget.NewCard(
		"",
		"",
		container.NewVBox(
			header,
			content,
		),
	)

	return card
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

	if gui.outputDirEntry.Text != "" {
		if err := os.MkdirAll(gui.outputDirEntry.Text, 0755); err != nil {
			dialog.ShowError(fmt.Errorf("failed to create output directory: %v", err), gui.window)
			return
		}
	}

	// Validate dimensions if needed
	if gui.processingMode == ModeOnlyDimensions || gui.processingMode == ModeBoth {
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
		gui.dimensions.Width = width
		gui.dimensions.Height = height
	}

	// Check Anki connection
	if err := gui.ankiService.CheckConnection(); err != nil {
		dialog.ShowError(fmt.Errorf("Anki connection error: %v\nPlease make sure Anki is running and AnkiConnect is installed", err), gui.window)
		return
	}

	outputDir := gui.outputDirEntry.Text

	// Create processor configuration based on mode
	config := pdf.ProcessorConfig{
		TempDir:    filepath.Join(os.TempDir(), "notesankify-temp"),
		OutputDir:  outputDir,
		Dimensions: gui.dimensions,
		ProcessingOptions: pdf.ProcessingOptions{
			CheckDimensions: gui.processingMode == ModeOnlyDimensions || gui.processingMode == ModeBoth,
			CheckMarkers:    gui.processingMode == ModeOnlyMarkers || gui.processingMode == ModeBoth,
		},
		Logger: gui.log,
	}

	var err error
	gui.processor, err = pdf.NewProcessor(config)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to initialize processor: %v", err), gui.window)
		return
	}

	gui.progress.Show()
	gui.updateStatus("Processing files...")

	go gui.processFiles()
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

	processingCompleteBanner := `
+------------------------------------------------------------------------------+
|                           PROCESSING COMPLETE                                |
+------------------------------------------------------------------------------+`

	skippedCardsBanner := `
+------------------------------------------------------------------------------+
|                            SKIPPED CARDS                                     |
+------------------------------------------------------------------------------+`

	gui.log.Info("\n%s\n", processingCompleteBanner)
	gui.log.Info("- Total PDFs processed: %d", report.ProcessedPDFs)
	gui.log.Info("- Total flashcards found: %d", report.TotalFlashcards)
	gui.log.Info("- Cards Added: %d", report.AddedCount)
	gui.log.Info("- Cards Skipped: %d", report.SkippedCount)
	gui.log.Info("- Time Taken: %v", report.TimeTaken())
	gui.log.Info("- Output directory: %s", gui.outputDirEntry.Text)

	gui.log.Info("- Log file saved to: %s\n\n\n\n\n\n", gui.logFileName)

	// If there were skipped cards, log them too
	if report.SkippedCount > 0 {
		gui.log.Info("\n%s\n", skippedCardsBanner)
		for _, card := range report.SkippedCards {
			gui.log.Info("- %s (Page %d, Hash:%s)",
				card.DeckName,
				card.PageNumber,
				card.Hash)
		}
	}

	message := fmt.Sprintf(
		"Processing Complete!\n\n"+
			"PDFs Processed: %d\n"+
			"Total Flashcards: %d\n"+
			"Cards Added: %d\n"+
			"Cards Skipped: %d\n"+
			"Time Taken: %v\n"+
			"Output directory: %s\n\n"+
			"Log file saved to: %s",
		report.ProcessedPDFs,
		report.TotalFlashcards,
		report.AddedCount,
		report.SkippedCount,
		report.TimeTaken(),
		gui.outputDirEntry.Text,
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
	logsDir := "notesankify-logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create logs directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFileName := filepath.Join(logsDir, fmt.Sprintf("notesankify_%s.log", timestamp))

	absLogPath, err := filepath.Abs(logFileName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	logFile, err := os.Create(absLogPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create log file: %w", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log := logger.New(
		logger.WithPrefix("[notesankify-gui] "),
		logger.WithOutput(multiWriter),
	)

	return log, absLogPath, nil
}

func (gui *NotesAnkifyGUI) handleModeChange(selected string) {
	switch selected {
	case "Pages with QUESTION/ANSWER Markers and Matching Dimensions":
		gui.processingMode = ModeBoth
		gui.dimContainer.Show()
	case "Only Pages with QUESTION/ANSWER Markers":
		gui.processingMode = ModeOnlyMarkers
		gui.dimContainer.Hide()
	case "Only Pages Matching Dimensions":
		gui.processingMode = ModeOnlyDimensions
		gui.dimContainer.Show()
	case "Process All Pages":
		gui.processingMode = ModeProcessAll
		gui.dimContainer.Hide()
	}
}

func (gui *NotesAnkifyGUI) processFiles() {
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

			if err := gui.ankiService.AddAllFlashcards(deckName, stats.ImagePairs, stats.PageNumbers, report); err != nil {
				gui.showError(fmt.Sprintf("Error adding flashcards to deck %s: %v", deckName, err))
				continue
			}
		}
	}

	report.EndTime = time.Now()
	gui.showCompletionDialog(report)
}

func (gui *NotesAnkifyGUI) Run() {
	gui.setupUI()
	gui.window.ShowAndRun()
}

func main() {
	gui := NewNotesAnkifyGUI()
	gui.Run()
}
