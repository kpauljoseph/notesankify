package acceptance_test

import (
	"context"
	"fmt"
	"github.com/gen2brain/go-fitz"
	"github.com/kpauljoseph/notesankify/pkg/logger"
	"github.com/kpauljoseph/notesankify/pkg/utils"
	. "github.com/kpauljoseph/notesankify/tests/acceptance"
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
		hashStore   *HashStore
	)

	BeforeAll(func() {
		testLogger = acceptanceTestLogger()
		testDataDir = getTestDataPath()
		testLogger.Info("Using test data directory: %s", testDataDir)

		hashStore = NewHashStore(testDataDir)
		Expect(hashStore.Load()).To(Succeed())

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

		config := pdf.ProcessorConfig{
			TempDir:   tempDir,
			OutputDir: outputDir,
			Dimensions: models.PageDimensions{
				Width:  utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH,
				Height: utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT,
			},
			ProcessingOptions: pdf.ProcessingOptions{
				CheckDimensions: true,
				CheckMarkers:    true,
			},
			Logger: testLogger,
		}

		processor, err = pdf.NewProcessor(config)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		testLogger.Debug("Cleaning up test directories")
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
		err = os.RemoveAll(outputDir)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		// Save updated hashes if in update mode
		Expect(hashStore.Save()).To(Succeed())
	})

	Context("Standard Flashcard Processing", Label("happy-path"), func() {
		It("should process standard flashcard PDF correctly", func() {
			By("Processing a PDF with only standard flashcard pages")
			pdfPath := filepath.Join(testDataDir, "standard_flashcards.pdf")
			filename := filepath.Base(pdfPath)
			testLogger.Info("Testing standard flashcard processing: %s", filename)

			stats, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			currentHashes := make(map[string]PageHash)
			for i, pair := range stats.ImagePairs {
				pageNum := fmt.Sprintf("%d", stats.PageNumbers[i])
				currentHashes[pageNum] = PageHash{Hash: pair.Hash}
				testLogger.Debug("Processed page %s with hash %s", pageNum, pair.Hash)
			}

			if hashStore.IsUpdateMode() {
				hashStore.UpdateFileHashes(filename, currentHashes)
				testLogger.Info("Updated hashes for %s with pages: %v",
					filename,
					GetPageNumbers(currentHashes))
			} else {
				expectedHashes, exists := hashStore.GetFileHashes(filename)
				Expect(exists).To(BeTrue(), "No expected hashes found for %s", filename)

				expectedPages := GetPageNumbers(expectedHashes.Pages)
				actualPages := GetPageNumbers(currentHashes)
				testLogger.Debug("Comparing pages - Expected: %v, Got: %v", expectedPages, actualPages)
				Expect(actualPages).To(Equal(expectedPages),
					"Processed page numbers don't match expected")

				for pageNum, currentHash := range currentHashes {
					expected, ok := expectedHashes.Pages[pageNum]
					Expect(ok).To(BeTrue(), "No expected hash for page %s", pageNum)
					Expect(currentHash.Hash).To(Equal(expected.Hash),
						"Hash mismatch for page %s", pageNum)
				}
			}

			// Debug page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			// standard_flashcards.pdf file contains flash cards in the following:
			// page indexes - 0,1,2,3,4
			// page numbers - 1,2,3,4,5
			// expectedPages is zero-based for internal use
			expectedPageIndices := []int{0, 1, 2, 3, 4}
			testLogger.Debug("Expected page indices to process: %v", expectedPageIndices)

			By("Verifying all pages were processed")
			Expect(stats.FlashcardCount).To(Equal(len(expectedPageIndices)))
			Expect(stats.ImagePairs).To(HaveLen(len(expectedPageIndices)))

			// Debug extracted files
			for i, pair := range stats.ImagePairs {
				pageNum := stats.PageNumbers[i]
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
				baseName := strings.TrimSuffix(filename, filepath.Ext(pdfPath))
				shortHash := pair.Hash[:8]

				Expect(pair.Question).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Question)).To(Equal(fmt.Sprintf("%s_%s_question.png", baseName, shortHash)))

				Expect(pair.Answer).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Answer)).To(Equal(fmt.Sprintf("%s_%s_answer.png", baseName, shortHash)))
			}
		})
	})

	Context("Mixed PDF Processing - same sized pages", Label("happy-path"), func() {
		It("should extract flashcards from mixed content PDF with same sized pages", func() {
			By("Processing a PDF with mixed content but same page sizes")
			pdfPath := filepath.Join(testDataDir, "mixed_content_sameSizeNormalPage_sameSizeFlashcardPage.pdf")
			filename := filepath.Base(pdfPath)
			testLogger.Info("Testing mixed content processing (same size): %s", filename)

			stats, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			currentHashes := make(map[string]PageHash)
			for i, pair := range stats.ImagePairs {
				pageNum := fmt.Sprintf("%d", stats.PageNumbers[i])
				currentHashes[pageNum] = PageHash{Hash: pair.Hash}
				testLogger.Debug("Processed page %s with hash %s", pageNum, pair.Hash)
			}

			if hashStore.IsUpdateMode() {
				hashStore.UpdateFileHashes(filename, currentHashes)
				testLogger.Info("Updated hashes for %s with pages: %v",
					filename,
					GetPageNumbers(currentHashes))
			} else {
				expectedHashes, exists := hashStore.GetFileHashes(filename)
				Expect(exists).To(BeTrue(), "No expected hashes found for %s", filename)

				expectedPages := GetPageNumbers(expectedHashes.Pages)
				actualPages := GetPageNumbers(currentHashes)
				testLogger.Debug("Comparing pages - Expected: %v, Got: %v", expectedPages, actualPages)
				Expect(actualPages).To(Equal(expectedPages),
					"Processed page numbers don't match expected")

				for pageNum, currentHash := range currentHashes {
					expected, ok := expectedHashes.Pages[pageNum]
					Expect(ok).To(BeTrue(), "No expected hash for page %s", pageNum)
					Expect(currentHash.Hash).To(Equal(expected.Hash),
						"Hash mismatch for page %s", pageNum)
				}
			}

			// Debug page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			// mixed_content_sameSizeNormalPage_sameSizeFlashcardPage.pdf file contains flash cards in the following:
			// page indices - 1, 2, 4, 5, 7
			// page numbers - 2, 3, 5, 6, 8
			// expectedPages is zero-based for internal use
			expectedPageIndices := []int{1, 2, 4, 5, 7}
			testLogger.Debug("Expected page indices to process: %v", expectedPageIndices)

			By("Only extracting pages with QUESTION/ANSWER markers")
			Expect(stats.FlashcardCount).To(Equal(len(expectedPageIndices)))
			Expect(stats.ImagePairs).To(HaveLen(len(expectedPageIndices)))

			// Debug and verify extracted files
			for i, pair := range stats.ImagePairs {
				pageNum := stats.PageNumbers[i]
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
				baseName := strings.TrimSuffix(filename, filepath.Ext(pdfPath))
				shortHash := pair.Hash[:8]

				Expect(pair.Question).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Question)).To(Equal(fmt.Sprintf("%s_%s_question.png", baseName, shortHash)))

				Expect(pair.Answer).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Answer)).To(Equal(fmt.Sprintf("%s_%s_answer.png", baseName, shortHash)))
			}
		})
	})

	Context("Mixed PDF Processing - different sized pages", Label("happy-path"), func() {
		It("should extract flashcards from mixed content PDF with different sized pages", func() {
			By("Processing a PDF with mixed content and different page sizes")
			pdfPath := filepath.Join(testDataDir, "mixed_content_largeNormalPage_smallFlashcardPage.pdf")
			filename := filepath.Base(pdfPath)
			testLogger.Info("Testing mixed content processing (different sizes): %s", filename)

			stats, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			currentHashes := make(map[string]PageHash)
			for i, pair := range stats.ImagePairs {
				pageNum := fmt.Sprintf("%d", stats.PageNumbers[i])
				currentHashes[pageNum] = PageHash{Hash: pair.Hash}
				testLogger.Debug("Processed page %s with hash %s", pageNum, pair.Hash)
			}

			if hashStore.IsUpdateMode() {
				hashStore.UpdateFileHashes(filename, currentHashes)
				testLogger.Info("Updated hashes for %s with pages: %v",
					filename,
					GetPageNumbers(currentHashes))
			} else {
				expectedHashes, exists := hashStore.GetFileHashes(filename)
				Expect(exists).To(BeTrue(), "No expected hashes found for %s", filename)

				expectedPages := GetPageNumbers(expectedHashes.Pages)
				actualPages := GetPageNumbers(currentHashes)
				testLogger.Debug("Comparing pages - Expected: %v, Got: %v", expectedPages, actualPages)
				Expect(actualPages).To(Equal(expectedPages),
					"Processed page numbers don't match expected")

				for pageNum, currentHash := range currentHashes {
					expected, ok := expectedHashes.Pages[pageNum]
					Expect(ok).To(BeTrue(), "No expected hash for page %s", pageNum)
					Expect(currentHash.Hash).To(Equal(expected.Hash),
						"Hash mismatch for page %s", pageNum)
				}
			}

			// Debug page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			// mixed_content_largeNormalPage_smallFlashcardPage.pdf file contains flash cards in the following:
			// page indexes - 1, 2, 4, 5, 7
			// page numbers - 2, 3, 5, 6, 8
			// expectedPages is zero-based for internal use
			expectedPageIndices := []int{1, 2, 4, 5, 7}
			testLogger.Debug("Expected page indices to process: %v", expectedPageIndices)

			By("Extracting only standard sized flashcard pages with Question/Answer markers")
			Expect(stats.FlashcardCount).To(Equal(len(expectedPageIndices)))
			Expect(stats.ImagePairs).To(HaveLen(len(expectedPageIndices)))

			// Debug and verify extracted files
			for i, pair := range stats.ImagePairs {
				pageNum := stats.PageNumbers[i]
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
				baseName := strings.TrimSuffix(filename, filepath.Ext(pdfPath))
				shortHash := pair.Hash[:8]

				Expect(pair.Question).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Question)).To(Equal(fmt.Sprintf("%s_%s_question.png", baseName, shortHash)))

				Expect(pair.Answer).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Answer)).To(Equal(fmt.Sprintf("%s_%s_answer.png", baseName, shortHash)))
			}
		})
	})

	Context("A4 Size PDF Processing - all pages are top question + bottom answer format without question/answer markers", Label("happy-path"), func() {
		It("should extract flashcards from A4 size pdf pages without flashcard markers", func() {
			By("Processing a PDF with same A4 page sizes without question answer markers")
			pdfPath := filepath.Join(testDataDir, "A4_size_normalPage_TopQnBottomAns_without_markers.pdf")
			filename := filepath.Base(pdfPath)
			testLogger.Info("Testing mixed content processing (same size): %s", filename)

			// Create processor with A4 dimensions and marker checking disabled
			config := pdf.ProcessorConfig{
				TempDir:   tempDir,
				OutputDir: outputDir,
				Dimensions: models.PageDimensions{
					Width:  utils.A4_PAGE_WIDTH,
					Height: utils.A4_PAGE_HEIGHT,
				},
				ProcessingOptions: pdf.ProcessingOptions{
					CheckDimensions: true,
					CheckMarkers:    false, // Don't check for QUESTION/ANSWER markers
				},
				Logger: testLogger,
			}

			processorWithoutMarkersCheck, err := pdf.NewProcessor(config)
			Expect(err).NotTo(HaveOccurred())

			stats, err := processorWithoutMarkersCheck.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			currentHashes := make(map[string]PageHash)
			for i, pair := range stats.ImagePairs {
				pageNum := fmt.Sprintf("%d", stats.PageNumbers[i])
				currentHashes[pageNum] = PageHash{Hash: pair.Hash}
				testLogger.Debug("Processed page %s with hash %s", pageNum, pair.Hash)
			}

			if hashStore.IsUpdateMode() {
				hashStore.UpdateFileHashes(filename, currentHashes)
				testLogger.Info("Updated hashes for %s with pages: %v",
					filename,
					GetPageNumbers(currentHashes))
			} else {
				expectedHashes, exists := hashStore.GetFileHashes(filename)
				Expect(exists).To(BeTrue(), "No expected hashes found for %s", filename)

				expectedPages := GetPageNumbers(expectedHashes.Pages)
				actualPages := GetPageNumbers(currentHashes)
				testLogger.Debug("Comparing pages - Expected: %v, Got: %v", expectedPages, actualPages)
				Expect(actualPages).To(Equal(expectedPages),
					"Processed page numbers don't match expected")

				for pageNum, currentHash := range currentHashes {
					expected, ok := expectedHashes.Pages[pageNum]
					Expect(ok).To(BeTrue(), "No expected hash for page %s", pageNum)
					Expect(currentHash.Hash).To(Equal(expected.Hash),
						"Hash mismatch for page %s", pageNum)
				}
			}

			// Debug page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			// A4_size_normalPage_TopQnBottomAns_without_markers.pdf file contains flash cards in the following:
			// page indices - 0, 1, 2, 3, 4
			// page numbers - 1, 2, 3, 4, 5
			// expectedPages is zero-based for internal use
			expectedPageIndices := []int{0, 1, 2, 3, 4}
			testLogger.Debug("Expected page indices to process: %v", expectedPageIndices)

			By("Extracting all pages with top half question and bottom half answer")
			Expect(stats.FlashcardCount).To(Equal(len(expectedPageIndices)))
			Expect(stats.ImagePairs).To(HaveLen(len(expectedPageIndices)))

			// Debug and verify extracted files
			for i, pair := range stats.ImagePairs {
				pageNum := stats.PageNumbers[i]
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
				baseName := strings.TrimSuffix(filename, filepath.Ext(pdfPath))
				shortHash := pair.Hash[:8]

				Expect(pair.Question).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Question)).To(Equal(fmt.Sprintf("%s_%s_question.png", baseName, shortHash)))

				Expect(pair.Answer).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Answer)).To(Equal(fmt.Sprintf("%s_%s_answer.png", baseName, shortHash)))
			}
		})
	})

	Context("Mixed Size PDF Processing - Process all pages as flashcards - No dimension or marker check", Label("happy-path"), func() {
		It("should extract flashcards from all pdf pages with mixed dimensions", func() {
			By("Processing a PDF with different page sizes but all pages have top half question bottom half answer")
			pdfPath := filepath.Join(testDataDir, "mixedSize_allPages_topQuestionBottomAnswer.pdf")
			filename := filepath.Base(pdfPath)
			testLogger.Info("Testing mixed content processing (different size): %s", filename)

			// Create processor with dimension and marker checking disabled
			config := pdf.ProcessorConfig{
				TempDir:   tempDir,
				OutputDir: outputDir,
				Dimensions: models.PageDimensions{
					Width:  0,
					Height: 0,
				},
				ProcessingOptions: pdf.ProcessingOptions{
					CheckDimensions: false, // Process all page sizes
					CheckMarkers:    false, // Don't check for markers
				},
				Logger: testLogger,
			}

			processorForAllPagesWithoutDimensionMarkerChecks, err := pdf.NewProcessor(config)
			Expect(err).NotTo(HaveOccurred())

			stats, err := processorForAllPagesWithoutDimensionMarkerChecks.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			currentHashes := make(map[string]PageHash)
			for i, pair := range stats.ImagePairs {
				pageNum := fmt.Sprintf("%d", stats.PageNumbers[i])
				currentHashes[pageNum] = PageHash{Hash: pair.Hash}
				testLogger.Debug("Processed page %s with hash %s", pageNum, pair.Hash)
			}

			if hashStore.IsUpdateMode() {
				hashStore.UpdateFileHashes(filename, currentHashes)
				testLogger.Info("Updated hashes for %s with pages: %v",
					filename,
					GetPageNumbers(currentHashes))
			} else {
				expectedHashes, exists := hashStore.GetFileHashes(filename)
				Expect(exists).To(BeTrue(), "No expected hashes found for %s", filename)

				expectedPages := GetPageNumbers(expectedHashes.Pages)
				actualPages := GetPageNumbers(currentHashes)
				testLogger.Debug("Comparing pages - Expected: %v, Got: %v", expectedPages, actualPages)
				Expect(actualPages).To(Equal(expectedPages),
					"Processed page numbers don't match expected")

				for pageNum, currentHash := range currentHashes {
					expected, ok := expectedHashes.Pages[pageNum]
					Expect(ok).To(BeTrue(), "No expected hash for page %s", pageNum)
					Expect(currentHash.Hash).To(Equal(expected.Hash),
						"Hash mismatch for page %s", pageNum)
				}
			}

			// Debug page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			// mixedSize_allPages_topQuestionBottomAnswer.pdf file contains flash cards in the following:
			// page indices - 0, 1, 2, 3, 4
			// page numbers - 1, 2, 3, 4, 5
			// expectedPages is zero-based for internal use
			expectedPageIndices := []int{0, 1, 2, 3, 4}
			testLogger.Debug("Expected page indices to process: %v", expectedPageIndices)

			By("Extracting all pages with top half question and bottom half answer")
			Expect(stats.FlashcardCount).To(Equal(len(expectedPageIndices)))
			Expect(stats.ImagePairs).To(HaveLen(len(expectedPageIndices)))

			// Debug and verify extracted files
			for i, pair := range stats.ImagePairs {
				pageNum := stats.PageNumbers[i]
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
				baseName := strings.TrimSuffix(filename, filepath.Ext(pdfPath))
				shortHash := pair.Hash[:8]

				Expect(pair.Question).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Question)).To(Equal(fmt.Sprintf("%s_%s_question.png", baseName, shortHash)))

				Expect(pair.Answer).To(BeAnExistingFile())
				Expect(filepath.Base(pair.Answer)).To(Equal(fmt.Sprintf("%s_%s_answer.png", baseName, shortHash)))
			}
		})
	})
})
