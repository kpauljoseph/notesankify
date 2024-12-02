package acceptance_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

var _ = Describe("NotesAnkify End-to-End", func() {
	var (
		processor *pdf.Processor
		tempDir   string
		ctx       context.Context
	)

	BeforeEach(func() {
		var err error
		ctx = context.Background()
		tempDir, err = os.MkdirTemp("", "notesankify-acceptance-*")
		Expect(err).NotTo(HaveOccurred())

		processor, err = pdf.NewProcessor(tempDir, models.PageDimensions{
			Width:  595.0,
			Height: 842.0,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Standard Flashcard Processing", Label("happy-path"), func() {
		It("should process standard flashcards correctly", func() {
			By("Given a PDF with standard flashcard pages")
			pdfPath := filepath.Join("testdata", "standard_flashcards.pdf")

			By("When processing the PDF")
			pages, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			By("Then it should extract all flashcard pages")
			Expect(pages).To(HaveLen(BeNumerically(">", 0)))

			By("And create image files for each page")
			for _, page := range pages {
				Expect(page.ImagePath).To(BeARegularFile())
			}
		})
	})

	Context("Mixed Content Processing", Label("content-handling"), func() {
		It("should handle mixed content PDFs correctly", func() {
			By("Given a PDF with mixed content")
			pdfPath := filepath.Join("testdata", "mixed_content.pdf")

			By("When processing the PDF")
			pages, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			By("Then it should only extract flashcard pages")
			for _, page := range pages {
				Expect(page.ImagePath).To(BeARegularFile())
			}
		})
	})

	Context("Flashcard Modifications", Label("change-detection"), func() {
		It("should detect modified flashcards", func() {
			By("Given a PDF with modified flashcards")
			pdfPath := filepath.Join("testdata", "modified_flashcards.pdf")

			By("When processing the PDF")
			pages, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			By("Then it should detect the modifications")
			// Add specific change detection assertions
		})
	})

	Context("Error Handling", Label("error-cases"), func() {
		It("should handle missing files gracefully", func() {
			_, err := processor.ProcessPDF(ctx, "nonexistent.pdf")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no such file"))
		})

		It("should handle corrupted PDFs gracefully", func() {
			pdfPath := filepath.Join("testdata", "corrupted.pdf")
			_, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).To(HaveOccurred())
		})

		It("should handle cancellation gracefully", func() {
			ctxWithCancel, cancel := context.WithCancel(ctx)
			cancel()
			_, err := processor.ProcessPDF(ctxWithCancel, "any.pdf")
			Expect(err).To(MatchError(context.Canceled))
		})
	})
})
