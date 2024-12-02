package acceptance_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"os"

	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

var _ = Describe("NotesAnkify End-to-End", Ordered, func() {
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
			Width:  2480, // A4 size at 300 DPI
			Height: 3508,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Standard Flashcard Processing", Label("happy-path"), func() {
		It("should process flashcard pages correctly", func() {
			By("Processing a PDF with flashcard pages")
			pdfPath := "./testdata/test.pdf" // Use your existing test PDF
			Expect(pdfPath).To(BeAnExistingFile())

			pages, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying extracted pages")
			Expect(pages).NotTo(BeEmpty())

			for _, page := range pages {
				Expect(page.ImagePath).To(BeAnExistingFile())
				Expect(page.PDFPath).To(Equal(pdfPath))
			}
		})
	})

	Context("Error Handling", Label("error-cases"), func() {
		It("should handle missing files gracefully", func() {
			_, err := processor.ProcessPDF(ctx, "nonexistent.pdf")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no such file"))
		})

		It("should handle context cancellation", func() {
			ctxWithCancel, cancel := context.WithCancel(ctx)
			cancel()
			_, err := processor.ProcessPDF(ctxWithCancel, "./testdata/test.pdf")
			Expect(err).To(MatchError(context.Canceled))
		})
	})
})
