package acceptance_test

import (
	"context"
	"fmt"
	"github.com/gen2brain/go-fitz"
	"github.com/kpauljoseph/notesankify/pkg/logger"
	"github.com/kpauljoseph/notesankify/pkg/utils"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

func acceptanceTestLogger() *logger.Logger {
	log := logger.New(
		logger.WithOutput(GinkgoWriter),
		logger.WithPrefix("[acceptance-test] "),
		logger.WithFlags(0),
	)
	log.SetVerbose(true)
	log.SetLevel(logger.LevelTrace)
	return log
}

func getTestDataPath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Could not get current file path")
	}

	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	return filepath.Join(projectRoot, "tests", "acceptance", "testdata")
}

var _ = Describe("NotesAnkify End-to-End", Ordered, func() {
	var (
		processor   *pdf.Processor
		tempDir     string
		outputDir   string
		ctx         context.Context
		testDataDir string
		testLogger  *logger.Logger
	)

	BeforeAll(func() {
		testLogger = acceptanceTestLogger()
		testDataDir = getTestDataPath()
		testLogger.Info("Using test data directory: %s", testDataDir)

		files := []string{
			"standard_flashcards.pdf",
			"mixed_content_sameSizeNormalPage_sameSizeFlashcardPage.pdf",
			"mixed_content_largeNormalPage_smallFlashcardPage.pdf",
		}

		for _, file := range files {
			path := filepath.Join(testDataDir, file)
			_, err := os.Stat(path)
			if err != nil {
				testLogger.Fatal("Required test file not found: %s", path)
			}
			testLogger.Debug("Found required test file: %s", file)
		}
	})

	BeforeEach(func() {
		var err error
		ctx = context.Background()
		tempDir, err = os.MkdirTemp("/tmp", "notesankify-acceptance-*")
		Expect(err).NotTo(HaveOccurred())

		outputDir, err = os.MkdirTemp("/tmp", "notesankify-output-*")
		Expect(err).NotTo(HaveOccurred())

		testLogger.Debug("Created temp directories:")
		testLogger.Debug("- Temp dir: %s", tempDir)
		testLogger.Debug("- Output dir: %s", outputDir)

		processor, err = pdf.NewProcessor(
			tempDir,
			outputDir,
			models.PageDimensions{
				Width:  utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH,
				Height: utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT,
			},
			testLogger,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		testLogger.Debug("Cleaning up test directories")
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
		err = os.RemoveAll(outputDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Standard Flashcard Processing", Label("happy-path"), func() {
		It("should process standard flashcard PDF correctly", func() {
			pdfPath := filepath.Join(testDataDir, "standard_flashcards.pdf")
			testLogger.Info("Testing standard flashcard processing: %s", filepath.Base(pdfPath))

			By("Processing a PDF with only standard flashcard pages")
			stats, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			// Debug page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			// standard_flashcards.pdf file contains flash cards in all the 5 pages.
			// expectedPages is zero-based for internal use
			expectedPages := []int{0, 1, 2, 3, 4}
			testLogger.Debug("Expected pages to process: %v", expectedPages)

			By("Verifying all pages were processed")
			Expect(stats.FlashcardCount).To(Equal(len(expectedPages)))
			Expect(stats.ImagePairs).To(HaveLen(len(expectedPages)))

			// Debug extracted files
			for i, pair := range stats.ImagePairs {
				pageNum := expectedPages[i] + 1 // Convert to 1-based for display
				testLogger.Debug("\n=== Processing Flashcard %d ===", pageNum)
				testLogger.Debug("Question: %s", pair.Question)
				testLogger.Debug("Answer: %s", pair.Answer)

				// Get original page content for debugging
				//bounds, err := doc.Bound(expectedPages[i])
				//if err == nil {
				//	fmt.Printf("Original Dimensions: %.2f x %.2f\n", float64(bounds.Dx()), float64(bounds.Dy()))
				//}
				//
				//text, err := doc.Text(expectedPages[i])
				//if err == nil {
				//	fmt.Printf("Original Content:\n%s\n", text)
				//}

				// Verify files exist and follow naming convention
				By(fmt.Sprintf("Checking page %d files", pageNum))
				baseName := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))

				Expect(pair.Question).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Question)).To(Equal(fmt.Sprintf("%s_%s_question.png", baseName, pair.Hash[:8])))

				Expect(pair.Answer).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Answer)).To(Equal(fmt.Sprintf("%s_%s_answer.png", baseName, pair.Hash[:8])))
			}
		})

		It("should extract flashcards from mixed content PDF with same sized pages", func() {
			By("Processing a PDF with mixed content but same page sizes")
			pdfPath := filepath.Join(testDataDir, "mixed_content_sameSizeNormalPage_sameSizeFlashcardPage.pdf")
			testLogger.Info("Testing mixed content processing (same size): %s", filepath.Base(pdfPath))

			By("Processing a PDF with mixed content but same page sizes")
			stats, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			// Debug page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			// mixed_content_sameSizeNormalPage_sameSizeFlashcardPage.pdf file contains flash cards in page indexes 1,2,4,5,7
			expectedPages := []int{1, 2, 4, 5, 7}
			testLogger.Debug("Expected pages to process: %v", expectedPages)

			By("Only extracting pages with QUESTION/ANSWER markers")
			Expect(stats.FlashcardCount).To(Equal(len(expectedPages)))
			Expect(stats.ImagePairs).To(HaveLen(len(expectedPages)))

			// Debug and verify extracted files
			for i, pair := range stats.ImagePairs {
				pageNum := expectedPages[i] + 1 // Convert to 1-based for display
				testLogger.Debug("\n=== Processing Flashcard %d ===", pageNum)
				testLogger.Debug("Question: %s", pair.Question)
				testLogger.Debug("Answer: %s", pair.Answer)

				// Get original page content for debugging
				//bounds, err := doc.Bound(expectedPages[i])
				//if err == nil {
				//	fmt.Printf("Original Dimensions: %.2f x %.2f\n", float64(bounds.Dx()), float64(bounds.Dy()))
				//}
				//
				//text, err := doc.Text(expectedPages[i])
				//if err == nil {
				//	fmt.Printf("Original Content:\n%s\n", text)
				//}

				// Verify files exist and follow naming convention
				By(fmt.Sprintf("Checking page %d files", pageNum))
				baseName := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))

				Expect(pair.Question).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Question)).To(Equal(fmt.Sprintf("%s_%s_question.png", baseName, pair.Hash[:8])))

				Expect(pair.Answer).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Answer)).To(Equal(fmt.Sprintf("%s_%s_answer.png", baseName, pair.Hash[:8])))
			}
		})

		It("should extract flashcards from mixed content PDF with different sized pages", func() {
			By("Processing a PDF with mixed content and different page sizes")
			pdfPath := filepath.Join(testDataDir, "mixed_content_largeNormalPage_smallFlashcardPage.pdf")
			testLogger.Info("Testing mixed content processing (different sizes): %s", filepath.Base(pdfPath))

			By("Processing a PDF with mixed content and different page sizes")
			stats, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			// Debug page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			// mixed_content_largeNormalPage_smallFlashcardPage.pdf file contains flash cards in page indexes 1,2,4,5,7
			expectedPages := []int{1, 2, 4, 5, 7}
			testLogger.Debug("Expected pages to process: %v", expectedPages)

			By("Extracting only Goodnotes standard sized pages with markers")
			Expect(stats.FlashcardCount).To(Equal(len(expectedPages)))
			Expect(stats.ImagePairs).To(HaveLen(len(expectedPages)))

			// Debug and verify extracted files
			for i, pair := range stats.ImagePairs {
				pageNum := expectedPages[i] + 1
				testLogger.Debug("\n=== Processing Flashcard %d ===", pageNum)
				testLogger.Debug("Question: %s", pair.Question)
				testLogger.Debug("Answer: %s", pair.Answer)

				// Get original page content for debugging
				//bounds, err := doc.Bound(expectedPages[i])
				//if err == nil {
				//	fmt.Printf("Original Dimensions: %.2f x %.2f\n", float64(bounds.Dx()), float64(bounds.Dy()))
				//}
				//
				//text, err := doc.Text(expectedPages[i])
				//if err == nil {
				//	fmt.Printf("Original Content:\n%s\n", text)
				//}

				// Verify files exist and follow naming convention
				By(fmt.Sprintf("Checking page %d files", pageNum))
				baseName := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))

				Expect(pair.Question).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Question)).To(Equal(fmt.Sprintf("%s_%s_question.png", baseName, pair.Hash[:8])))

				Expect(pair.Answer).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Answer)).To(Equal(fmt.Sprintf("%s_%s_answer.png", baseName, pair.Hash[:8])))
			}
		})
	})
})
