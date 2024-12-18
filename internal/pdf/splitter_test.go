package pdf_test

import (
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/kpauljoseph/notesankify/internal/pdf"
)

var _ = Describe("Flashcard Splitter", func() {
	var (
		splitter   *pdf.Splitter
		sourceDir  string
		outputDir  string
		testLogger *log.Logger
	)

	BeforeEach(func() {
		var err error
		sourceDir, err = os.MkdirTemp("", "splitter-test-source-*")
		Expect(err).NotTo(HaveOccurred())

		outputDir, err = os.MkdirTemp("", "splitter-test-output-*")
		Expect(err).NotTo(HaveOccurred())

		testLogger = log.New(GinkgoWriter, "[test] ", log.LstdFlags)

		splitter, err = pdf.NewSplitter(outputDir, testLogger)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(sourceDir)
		os.RemoveAll(outputDir)
	})

	Context("when splitting a single image", func() {
		var testImagePath string

		BeforeEach(func() {
			img := createTestImage(200, 400)
			testImagePath = filepath.Join(sourceDir, "test_page1.png")

			f, err := os.Create(testImagePath)
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()

			err = png.Encode(f, img)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should split the image into question and answer parts", func() {
			pair, err := splitter.SplitImage(testImagePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(pair.Question).To(BeAnExistingFile())
			Expect(pair.Answer).To(BeAnExistingFile())

			Expect(filepath.Base(pair.Question)).To(Equal("test_page1_question.png"))
			Expect(filepath.Base(pair.Answer)).To(Equal("test_page1_answer.png"))

			questionImg := readImage(pair.Question)
			answerImg := readImage(pair.Answer)

			Expect(questionImg.Bounds().Dx()).To(Equal(200))
			Expect(questionImg.Bounds().Dy()).To(Equal(200))
			Expect(answerImg.Bounds().Dx()).To(Equal(200))
			Expect(answerImg.Bounds().Dy()).To(Equal(200))
		})
	})

	Context("when processing a directory", func() {
		BeforeEach(func() {
			for i := 1; i <= 3; i++ {
				img := createTestImage(200, 400)
				path := filepath.Join(sourceDir, fmt.Sprintf("test_page%d.png", i))

				f, err := os.Create(path)
				Expect(err).NotTo(HaveOccurred())

				err = png.Encode(f, img)
				Expect(err).NotTo(HaveOccurred())
				f.Close()
			}

			err := os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("test"), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should process all PNG files in the directory", func() {
			pairs, err := splitter.SplitAll(sourceDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(pairs).To(HaveLen(3))

			for i, pair := range pairs {
				Expect(pair.Question).To(BeAnExistingFile())
				Expect(pair.Answer).To(BeAnExistingFile())
				pageNum := i + 1
				Expect(filepath.Base(pair.Question)).To(Equal(fmt.Sprintf("test_page%d_question.png", pageNum)))
				Expect(filepath.Base(pair.Answer)).To(Equal(fmt.Sprintf("test_page%d_answer.png", pageNum)))
			}
		})
	})
})

func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	midPoint := height / 2

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
