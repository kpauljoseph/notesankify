// internal/pdf/processor_test.go
package pdf_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

var _ = Describe("PDF Processor", func() {
	var (
		processor *pdf.Processor
		tempDir   string
		ctx       context.Context
	)

	BeforeEach(func() {
		var err error
		ctx = context.Background()
		tempDir, err = os.MkdirTemp("", "notesankify-test-*")
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

	Context("Page dimension matching", func() {
		DescribeTable("matchesFlashcardSize",
			func(width, height float64, expected models.PageDimensions, shouldMatch bool) {
				result := pdf.MatchesFlashcardSize(width, height, expected)
				Expect(result).To(Equal(shouldMatch))
			},
			Entry("exact match", 595.0, 842.0, models.PageDimensions{Width: 595.0, Height: 842.0}, true),
			Entry("within tolerance", 595.1, 842.1, models.PageDimensions{Width: 595.0, Height: 842.0}, true),
			Entry("completely different", 100.0, 100.0, models.PageDimensions{Width: 595.0, Height: 842.0}, false),
		)
	})

	Context("PDF Processing", func() {
		It("should handle non-existent files", func() {
			_, err := processor.ProcessPDF(ctx, "nonexistent.pdf")
			Expect(err).To(HaveOccurred())
		})

		It("should process valid PDF files", func() {
			pdfPath := filepath.Join("testdata", "test.pdf")
			_, err := processor.ProcessPDF(ctx, pdfPath)
			Expect(err).NotTo(HaveOccurred())
		})

		When("context is cancelled", func() {
			It("should stop processing", func() {
				ctxWithCancel, cancel := context.WithCancel(ctx)
				cancel()
				_, err := processor.ProcessPDF(ctxWithCancel, "any.pdf")
				Expect(err).To(MatchError(context.Canceled))
			})
		})
	})

	Context("Temporary file management", func() {
		It("should clean up temporary files", func() {
			err := processor.Cleanup()
			Expect(err).NotTo(HaveOccurred())
			Expect(tempDir).NotTo(BeADirectory())
		})
	})
})
