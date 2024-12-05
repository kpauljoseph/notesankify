package pdf_test

import (
	"github.com/kpauljoseph/notesankify/pkg/utils"
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
			Width:  utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH,  // Goodnotes standard width
			Height: utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT, // Goodnotes standard height
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Goodnotes dimensions", func() {
		DescribeTable("matchesGoodnotesDimensions",
			func(width, height float64, shouldMatch bool) {
				result := pdf.MatchesGoodnotesDimensions(width, height)
				Expect(result).To(Equal(shouldMatch))
			},
			Entry("exact match",
				utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH, utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT,
				true,
			),
			Entry("within tolerance",
				455.5, 587.9,
				true,
			),
			Entry("rotated exact match",
				587.52, 455.04,
				true,
			),
			Entry("rotated within tolerance",
				587.9, 455.5,
				true,
			),
			Entry("completely different",
				595.28, 841.89, // A4 size
				false,
			),
		)
	})

	Context("Flashcard marker detection", func() {
		DescribeTable("containsFlashcardMarkers",
			func(text string, shouldMatch bool) {
				result := pdf.ContainsFlashcardMarkers(text)
				Expect(result).To(Equal(shouldMatch))
			},
			Entry("standard markers",
				"QUESTION\nsome text\nANSWER\nmore text",
				true,
			),
			Entry("markers with different case",
				"Question\nsome text\nanswer\nmore text",
				true,
			),
			Entry("only question marker",
				"QUESTION\nsome text",
				false,
			),
			Entry("only answer marker",
				"ANSWER\nsome text",
				false,
			),
			Entry("no markers",
				"some random text",
				false,
			),
		)
	})

	Context("when creating a new processor", func() {
		It("should create the temporary directory", func() {
			newTempDir := filepath.Join(tempDir, "newtemp")
			_, err := pdf.NewProcessor(newTempDir, models.PageDimensions{
				Width:  455.04,
				Height: 587.52,
			})
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
