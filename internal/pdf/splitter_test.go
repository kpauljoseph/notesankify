package pdf_test

import (
	"fmt"
	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/pkg/logger"
	"github.com/kpauljoseph/notesankify/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

func splitterTestLogger() *logger.Logger {
	log := logger.New(
		logger.WithOutput(GinkgoWriter),
		logger.WithPrefix("[splitter-test] "),
		logger.WithFlags(0),
	)
	log.SetVerbose(true)
	log.SetLevel(logger.LevelTrace)
	return log
}

func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	midPoint := height / 2

	// Create distinct colors for question and answer sections
	for y := 0; y < midPoint; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	for y := midPoint; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{0, 0, 255, 255})
		}
	}

	return img
}

func readImage(path string) image.Image {
	f, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()

	img, err := png.Decode(f)
	Expect(err).NotTo(HaveOccurred())

	return img
}

var _ = Describe("Flashcard Splitter", func() {
	var (
		splitter   *pdf.Splitter
		sourceDir  string
		outputDir  string
		testLogger *logger.Logger
	)

	BeforeEach(func() {
		var err error
		sourceDir, err = os.MkdirTemp("", "splitter-test-source-*")
		Expect(err).NotTo(HaveOccurred())

		outputDir, err = os.MkdirTemp("", "splitter-test-output-*")
		Expect(err).NotTo(HaveOccurred())

		testLogger = splitterTestLogger()
		testLogger.Debug("Setting up test environment")
		testLogger.Debug("Source directory: %s", sourceDir)
		testLogger.Debug("Output directory: %s", outputDir)

		splitter, err = pdf.NewSplitter(outputDir, testLogger)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		testLogger.Debug("Cleaning up test environment")
		os.RemoveAll(sourceDir)
		os.RemoveAll(outputDir)
		testLogger.Debug("Test cleanup completed")
	})

	Context("when splitting a single image", func() {
		var (
			testImagePath string
			baseName      string
			fullHash      string
		)

		BeforeEach(func() {
			testLogger.Debug("Creating test image")
			img := createTestImage(200, 400)
			baseName = "test_page1"
			testImagePath = filepath.Join(sourceDir, baseName+".png")

			f, err := os.Create(testImagePath)
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()

			err = png.Encode(f, img)
			Expect(err).NotTo(HaveOccurred())
			testLogger.Debug("Created test image at: %s", testImagePath)

			fullHash, err = utils.GenerateImageHash(img)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should split the image into question and answer parts", func() {
			testLogger.Debug("Testing image splitting")
			pair, err := splitter.SplitImageWithHash(testImagePath, baseName, fullHash)
			Expect(err).NotTo(HaveOccurred())

			testLogger.Debug("Checking split results")
			testLogger.Debug("Question file: %s", pair.Question)
			testLogger.Debug("Answer file: %s", pair.Answer)

			Expect(pair.Question).To(BeAnExistingFile())
			Expect(pair.Answer).To(BeAnExistingFile())

			expectedQuestionName := fmt.Sprintf("%s_%s_question.png", baseName, fullHash[:8])
			expectedAnswerName := fmt.Sprintf("%s_%s_answer.png", baseName, fullHash[:8])

			Expect(filepath.Base(pair.Question)).To(Equal(expectedQuestionName))
			Expect(filepath.Base(pair.Answer)).To(Equal(expectedAnswerName))
			Expect(pair.Hash).To(Equal(fullHash))

			questionImg := readImage(pair.Question)
			answerImg := readImage(pair.Answer)

			testLogger.Debug("Verifying image dimensions")
			Expect(questionImg.Bounds().Dx()).To(Equal(200))
			Expect(questionImg.Bounds().Dy()).To(Equal(200))
			Expect(answerImg.Bounds().Dx()).To(Equal(200))
			Expect(answerImg.Bounds().Dy()).To(Equal(200))
		})
	})
})
