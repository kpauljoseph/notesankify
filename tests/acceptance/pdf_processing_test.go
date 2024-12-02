package acceptance_test

import (
	"context"
	"fmt"
	"github.com/gen2brain/go-fitz"
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
		ctx         context.Context
		testDataDir string
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
		tempDir, err = os.MkdirTemp("", "notesankify-acceptance-*")
		Expect(err).NotTo(HaveOccurred())

		processor, err = pdf.NewProcessor(tempDir, models.PageDimensions{
			Width:  455.04,
			Height: 587.52,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Standard Flashcard Processing", Label("happy-path"), func() {
		It("should process standard flashcard PDF correctly", func() {
			pdfPath := filepath.Join(testDataDir, "standard_flashcards.pdf")

			By("Processing a PDF with only standard flashcard pages")
			pages, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			// Debug page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			for i, page := range pages {
				fmt.Printf("\n=== Page %d ===\n", i)
				fmt.Printf("PDFPath: %s\n", page.PDFPath)
				fmt.Printf("PageNum: %d\n", page.PageNum)
				fmt.Printf("ImagePath: %s\n", page.ImagePath)

				bounds, err := doc.Bound(page.PageNum)
				if err == nil {
					fmt.Printf("Dimensions: %.2f x %.2f\n", float64(bounds.Dx()), float64(bounds.Dy()))
				}

				text, err := doc.Text(page.PageNum)
				if err == nil {
					fmt.Printf("Content:\n%s\n", text)
				}
			}
			Expect(err).NotTo(HaveOccurred())

			By("Verifying all pages were processed")
			Expect(pages).To(HaveLen(5))

			for _, page := range pages {
				Expect(page.ImagePath).To(BeAnExistingFile())
				Expect(page.PDFPath).To(Equal(pdfPath))
			}
		})

		It("should extract flashcards from mixed content PDF with same sized pages", func() {
			By("Processing a PDF with mixed content but same page sizes")
			pdfPath := filepath.Join(testDataDir, "mixed_content_sameSizeNormalPage_sameSizeFlashcardPage.pdf")

			pages, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			//Debug Page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			for i, page := range pages {
				fmt.Printf("\n=== Page %d ===\n", i)
				fmt.Printf("PDFPath: %s\n", page.PDFPath)
				fmt.Printf("PageNum: %d\n", page.PageNum)
				fmt.Printf("ImagePath: %s\n", page.ImagePath)

				bounds, err := doc.Bound(page.PageNum)
				if err == nil {
					fmt.Printf("Dimensions: %.2f x %.2f\n", float64(bounds.Dx()), float64(bounds.Dy()))
				}

				text, err := doc.Text(page.PageNum)
				if err == nil {
					fmt.Printf("Content:\n%s\n", text)
				}
			}
			Expect(err).NotTo(HaveOccurred())

			By("Only extracting pages with QUESTION/ANSWER markers")
			for _, page := range pages {
				Expect(page.ImagePath).To(BeAnExistingFile())
				Expect(page.PDFPath).To(Equal(pdfPath))
			}
		})

		It("should extract flashcards from mixed content PDF with different sized pages", func() {
			By("Processing a PDF with mixed content and different page sizes")
			pdfPath := filepath.Join(testDataDir, "mixed_content_largeNormalPage_smallFlashcardPage.pdf")

			pages, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			// Debug page content
			doc, err := fitz.New(pdfPath)
			Expect(err).NotTo(HaveOccurred())
			defer doc.Close()

			for i, page := range pages {
				fmt.Printf("\n=== Page %d ===\n", i)
				fmt.Printf("PDFPath: %s\n", page.PDFPath)
				fmt.Printf("PageNum: %d\n", page.PageNum)
				fmt.Printf("ImagePath: %s\n", page.ImagePath)

				bounds, err := doc.Bound(page.PageNum)
				if err == nil {
					fmt.Printf("Dimensions: %.2f x %.2f\n", float64(bounds.Dx()), float64(bounds.Dy()))
				}

				text, err := doc.Text(page.PageNum)
				if err == nil {
					fmt.Printf("Content:\n%s\n", text)
				}
			}
			Expect(err).NotTo(HaveOccurred())

			By("Extracting only Goodnotes standard sized pages with markers")
			for _, page := range pages {
				Expect(page.ImagePath).To(BeAnExistingFile())
				Expect(page.PDFPath).To(Equal(pdfPath))
			}
		})
	})
})
