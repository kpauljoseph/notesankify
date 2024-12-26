package pdf_test

import (
	"github.com/kpauljoseph/notesankify/pkg/logger"
	"github.com/kpauljoseph/notesankify/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"os"
	"path/filepath"

	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

func processorTestLogger() *logger.Logger {
	log := logger.New(
		logger.WithOutput(GinkgoWriter),
		logger.WithPrefix("[pdf-test] "),
		logger.WithFlags(0),
	)
	log.SetVerbose(true)
	log.SetLevel(logger.LevelTrace)
	return log
}

var _ = Describe("PDF Processor", func() {
	var (
		processor  *pdf.Processor
		tempDir    string
		outputDir  string
		testLogger *logger.Logger
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "notesankify-test-*")
		Expect(err).NotTo(HaveOccurred())

		outputDir, err = os.MkdirTemp("", "notesankify-output-*")
		Expect(err).NotTo(HaveOccurred())

		testLogger = processorTestLogger()
		testLogger.Debug("Setting up test environment")
		testLogger.Debug("Temp directory: %s", tempDir)
		testLogger.Debug("Output directory: %s", outputDir)

		processor, err = pdf.NewProcessor(
			tempDir,
			outputDir,
			models.PageDimensions{
				Width:  utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH,
				Height: utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT,
			},
			false, // skipMarkerCheck default to false
			false, // skipDimensionCheck default to false
			testLogger,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		testLogger.Debug("Cleaning up test environment")
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
		err = os.RemoveAll(outputDir)
		Expect(err).NotTo(HaveOccurred())
		testLogger.Debug("Test cleanup completed")
	})

	Context("Standard Size Flashcard dimensions", func() {
		DescribeTable("matchesFlashcardDimensions",
			func(width, height float64, shouldMatch bool) {
				testLogger.Trace("Testing dimensions: %.2f x %.2f", width, height)
				processor, err := pdf.NewProcessor(
					tempDir,
					outputDir,
					models.PageDimensions{
						Width:  utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH,
						Height: utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT,
					},
					false,
					false,
					testLogger,
				)
				Expect(err).NotTo(HaveOccurred())
				result := processor.MatchesFlashcardDimensions(width, height)
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
				testLogger.Trace("Testing marker text: %q", text)
				result := pdf.ContainsFlashcardMarkers(text)
				Expect(result).To(Equal(shouldMatch))
			},
			Entry("standard markers",
				"QUESTION\nsome text\nANSWER\nmore text",
				true,
			),
			Entry("markers with different case",
				"Question\nsome text\nanswer\nmore text",
				false,
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
			testLogger.Debug("Testing nested output directory creation: %s", newOutputDir)

			_, err := pdf.NewProcessor(
				tempDir,
				newOutputDir,
				models.PageDimensions{
					Width:  utils.GOODNOTES_STANDARD_FLASHCARD_WIDTH,
					Height: utils.GOODNOTES_STANDARD_FLASHCARD_HEIGHT,
				},
				false,
				false,
				testLogger,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(newOutputDir).To(BeADirectory())
			testLogger.Debug("Successfully created nested output directory")
		})

		It("should cleanup temporary directory", func() {
			testLogger.Debug("Testing temporary directory cleanup")
			Expect(tempDir).To(BeADirectory())
			err := processor.Cleanup()
			Expect(err).NotTo(HaveOccurred())
			Expect(tempDir).NotTo(BeADirectory())
			Expect(outputDir).To(BeADirectory())
			testLogger.Debug("Successfully cleaned up temporary directory")
		})
	})

	Context("Marker checking behavior", func() {
		It("should respect marker checking flag", func() {
			normalProcessor, err := pdf.NewProcessor(
				tempDir,
				outputDir,
				models.PageDimensions{Width: 455.04, Height: 587.52},
				false,
				false,
				testLogger,
			)
			Expect(err).NotTo(HaveOccurred())

			skipMarkersProcessor, err := pdf.NewProcessor(
				tempDir,
				outputDir,
				models.PageDimensions{Width: 455.04, Height: 587.52},
				true,
				false,
				testLogger,
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(normalProcessor.ShouldCheckMarkers()).To(BeTrue())

			Expect(skipMarkersProcessor.ShouldCheckMarkers()).To(BeFalse())
		})
	})

	Context("Dimension checking behavior", func() {
		It("should respect dimension checking flag", func() {
			normalProcessor, err := pdf.NewProcessor(
				tempDir,
				outputDir,
				models.PageDimensions{Width: 455.04, Height: 587.52},
				false,
				false,
				testLogger,
			)
			Expect(err).NotTo(HaveOccurred())

			skipDimensionsProcessor, err := pdf.NewProcessor(
				tempDir,
				outputDir,
				models.PageDimensions{Width: 455.04, Height: 587.52},
				false,
				true,
				testLogger,
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(normalProcessor.ShouldCheckDimensions()).To(BeTrue())

			Expect(skipDimensionsProcessor.ShouldCheckDimensions()).To(BeFalse())
		})
	})
})
