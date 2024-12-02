package pdf_test

import (
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
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "notesankify-test-*")
		Expect(err).NotTo(HaveOccurred())

		processor, err = pdf.NewProcessor(tempDir, models.PageDimensions{
			Width:  2480, // A4 size at 300 DPI: 8.27 × 11.69 inches
			Height: 3508,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("matchesFlashcardSize", func() {
		DescribeTable("dimension matching",
			func(width, height float64, expected models.PageDimensions, shouldMatch bool) {
				result := pdf.MatchesFlashcardSize(width, height, expected)
				Expect(result).To(Equal(shouldMatch))
			},
			Entry("exact match",
				2480.0, 3508.0,
				models.PageDimensions{Width: 2480.0, Height: 3508.0},
				true,
			),
			Entry("within tolerance",
				2482.0, 3510.0,
				models.PageDimensions{Width: 2480.0, Height: 3508.0},
				true,
			),
			Entry("outside tolerance",
				2500.0, 3520.0,
				models.PageDimensions{Width: 2480.0, Height: 3508.0},
				false,
			),
			Entry("completely different",
				1000.0, 1000.0,
				models.PageDimensions{Width: 2480.0, Height: 3508.0},
				false,
			),
		)
	})

	Context("when creating a new processor", func() {
		It("should create the temporary directory", func() {
			newTempDir := filepath.Join(tempDir, "newtemp")
			_, err := pdf.NewProcessor(newTempDir, models.PageDimensions{Width: 100, Height: 100})
			Expect(err).NotTo(HaveOccurred())
			Expect(newTempDir).To(BeADirectory())
		})
	})

	Context("when cleaning up", func() {
		It("should remove the temporary directory", func() {
			Expect(tempDir).To(BeADirectory())
			err := processor.Cleanup()
			Expect(err).NotTo(HaveOccurred())
			Expect(tempDir).NotTo(BeADirectory())
		})
	})
})
