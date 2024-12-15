package pdf_test

import (
	"github.com/kpauljoseph/notesankify/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"log"
	"os"
	"path/filepath"

	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

var _ = Describe("PDF Processor", func() {
	var (
		processor  *pdf.Processor
		tempDir    string
		outputDir  string
		testLogger *log.Logger
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "notesankify-test-*")
		Expect(err).NotTo(HaveOccurred())

		outputDir, err = os.MkdirTemp("", "notesankify-output-*")
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
				utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT, utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH,
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

	Context("Directory management", func() {
		It("should create output directory if it doesn't exist", func() {
			newOutputDir := filepath.Join(outputDir, "nested", "output")
			_, err := pdf.NewProcessor(
				tempDir,
				newOutputDir,
				models.PageDimensions{Width: utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH, Height: utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT},
				testLogger,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(newOutputDir).To(BeADirectory())
		})

		It("should cleanup temporary directory", func() {
			Expect(tempDir).To(BeADirectory())
			err := processor.Cleanup()
			Expect(err).NotTo(HaveOccurred())
			Expect(tempDir).NotTo(BeADirectory())
			Expect(outputDir).To(BeADirectory())
		})
	})
})
