package acceptance_test

import (
	"context"
	"fmt"
	"github.com/gen2brain/go-fitz"
	"github.com/kpauljoseph/notesankify/pkg/utils"
	"log"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

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
		testLogger  *log.Logger
	)

	BeforeAll(func() {
		testDataDir = getTestDataPath()
		fmt.Printf("Using test data directory: %s\n", testDataDir)

		files := []string{
			"standard_flashcards.pdf",
			"mixed_content_sameSizeNormalPage_sameSizeFlashcardPage.pdf",
			"mixed_content_largeNormalPage_smallFlashcardPage.pdf",
		}

		for _, file := range files {
			path := filepath.Join(testDataDir, file)
			_, err := os.Stat(path)
			if err != nil {
				Fail(fmt.Sprintf("Required test file not found: %s", path))
			}
		}
	})

	BeforeEach(func() {
		var err error
		ctx = context.Background()
		tempDir, err = os.MkdirTemp("/tmp", "notesankify-acceptance-*")
		Expect(err).NotTo(HaveOccurred())

		outputDir, err = os.MkdirTemp("/tmp", "notesankify-output-*")
		Expect(err).NotTo(HaveOccurred())

		testLogger = log.New(GinkgoWriter, "[test] ", log.LstdFlags)

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
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
		err = os.RemoveAll(outputDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Standard Flashcard Processing", Label("happy-path"), func() {
		It("should process standard flashcard PDF correctly", func() {
			pdfPath := filepath.Join(testDataDir, "standard_flashcards.pdf")

			By("Processing a PDF with only standard flashcard pages")
			stats, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			// standard_flashcards.pdf file contains flash cards in all the 5 pages.
			expectedPageNums := []int{1, 2, 3, 4, 5}

			By("Verifying all pages were processed")
			Expect(stats.FlashcardCount).To(Equal(len(expectedPageNums)))

			files, err := os.ReadDir(outputDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveLen(len(expectedPageNums)))

			for i, file := range files {
				filePath := filepath.Join(outputDir, file.Name())
				fmt.Printf("\n=== Output File %d ===\n", i)
				fmt.Printf("Filename: %s\n", file.Name())
				fmt.Printf("Path: %s\n", filePath)

				Expect(filePath).To(BeAnExistingFile())

				// Get original page content for debugging
				pageNum := expectedPageNums[i]
				bounds, err := doc.Bound(pageNum)
				if err == nil {
					fmt.Printf("Original Dimensions: %.2f x %.2f\n", float64(bounds.Dx()), float64(bounds.Dy()))
				}

				text, err := doc.Text(pageNum)
				if err == nil {
					fmt.Printf("Original Content:\n%s\n", text)
				}
			}

			// Verify page order
			for _, pageNum := range expectedPageNums {
				filename := fmt.Sprintf("%s_page%d.png", filepath.Base(pdfPath[:len(pdfPath)-4]), pageNum)
				filePath := filepath.Join(outputDir, filename)
				Expect(filePath).To(BeAnExistingFile(),
					"Expected file for page %d not found: %s", pageNum, filename)
			}
		})

		It("should extract flashcards from mixed content PDF with same sized pages", func() {
			By("Processing a PDF with mixed content but same page sizes")
			pdfPath := filepath.Join(testDataDir, "mixed_content_sameSizeNormalPage_sameSizeFlashcardPage.pdf")

			stats, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			//Debug Page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			// mixed_content_sameSizeNormalPage_sameSizeFlashcardPage.pdf file contains flash cards in page indexes 1,2,4,5,7
			expectedPageNums := []int{2, 3, 5, 6, 8}

			By("Only extracting pages with QUESTION/ANSWER markers")
			Expect(stats.FlashcardCount).To(Equal(len(expectedPageNums)))

			files, err := os.ReadDir(outputDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveLen(len(expectedPageNums)))

			// Debug output files
			for i, file := range files {
				filePath := filepath.Join(outputDir, file.Name())
				fmt.Printf("\n=== Output File %d ===\n", i)
				fmt.Printf("Filename: %s\n", file.Name())
				fmt.Printf("Path: %s\n", filePath)

				Expect(filePath).To(BeAnExistingFile())

				pageNum := expectedPageNums[i]
				bounds, err := doc.Bound(pageNum)
				if err == nil {
					fmt.Printf("Original Dimensions: %.2f x %.2f\n", float64(bounds.Dx()), float64(bounds.Dy()))
				}

				text, err := doc.Text(pageNum)
				if err == nil {
					fmt.Printf("Original Content:\n%s\n", text)
				}
			}

			for _, pageNum := range expectedPageNums {
				filename := fmt.Sprintf("%s_page%d.png", filepath.Base(pdfPath[:len(pdfPath)-4]), pageNum)
				filePath := filepath.Join(outputDir, filename)
				Expect(filePath).To(BeAnExistingFile(),
					"Expected file for page %d not found: %s", pageNum, filename)
			}
		})

		It("should extract flashcards from mixed content PDF with different sized pages", func() {
			By("Processing a PDF with mixed content and different page sizes")
			pdfPath := filepath.Join(testDataDir, "mixed_content_largeNormalPage_smallFlashcardPage.pdf")

			stats, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			// Debug page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			// mixed_content_largeNormalPage_smallFlashcardPage.pdf file contains flash cards in page indexes 1,2,4,5,7
			expectedPageNums := []int{2, 3, 5, 6, 8}

			By("Extracting only Goodnotes standard sized pages with markers")
			Expect(stats.FlashcardCount).To(Equal(len(expectedPageNums)))

			files, err := os.ReadDir(outputDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveLen(len(expectedPageNums)))

			for i, file := range files {
				filePath := filepath.Join(outputDir, file.Name())
				fmt.Printf("\n=== Output File %d ===\n", i)
				fmt.Printf("Filename: %s\n", file.Name())
				fmt.Printf("Path: %s\n", filePath)

				Expect(filePath).To(BeAnExistingFile())

				pageNum := expectedPageNums[i]
				bounds, err := doc.Bound(pageNum)
				if err == nil {
					fmt.Printf("Original Dimensions: %.2f x %.2f\n", float64(bounds.Dx()), float64(bounds.Dy()))
				}

				text, err := doc.Text(pageNum)
				if err == nil {
					fmt.Printf("Original Content:\n%s\n", text)
				}
			}

			for _, pageNum := range expectedPageNums {
				filename := fmt.Sprintf("%s_page%d.png", filepath.Base(pdfPath[:len(pdfPath)-4]), pageNum)
				filePath := filepath.Join(outputDir, filename)
				Expect(filePath).To(BeAnExistingFile(),
					"Expected file for page %d not found: %s", pageNum, filename)
			}
		})
	})
})
