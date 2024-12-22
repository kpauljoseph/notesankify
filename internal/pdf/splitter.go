package pdf

import (
	"fmt"
	"github.com/kpauljoseph/notesankify/pkg/logger"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

type ImagePair struct {
	Question string
	Answer   string
	Hash     string
}

type Splitter struct {
	outputDir string
	logger    *logger.Logger
}

func NewSplitter(outputDir string, logger *logger.Logger) (*Splitter, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &Splitter{
		outputDir: outputDir,
		logger:    logger,
	}, nil
}

func (s *Splitter) SplitImageWithHash(imagePath, baseName, fullHash string) (*ImagePair, error) {
	s.logger.Debug("Splitting image: %s", imagePath)

	srcFile, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer srcFile.Close()

	src, err := png.Decode(srcFile)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	midPoint := height / 2

	questionPath := filepath.Join(s.outputDir, fmt.Sprintf("%s_%s_question.png", baseName, fullHash[:8]))
	answerPath := filepath.Join(s.outputDir, fmt.Sprintf("%s_%s_answer.png", baseName, fullHash[:8]))

	questionImg := image.NewRGBA(image.Rect(0, 0, width, midPoint))
	for y := 0; y < midPoint; y++ {
		for x := 0; x < width; x++ {
			questionImg.Set(x, y, src.At(x, y))
		}
	}

	answerImg := image.NewRGBA(image.Rect(0, 0, width, height-midPoint))
	for y := midPoint; y < height; y++ {
		for x := 0; x < width; x++ {
			answerImg.Set(x, y-midPoint, src.At(x, y))
		}
	}

	if err := s.saveImage(questionImg, questionPath); err != nil {
		return nil, fmt.Errorf("failed to save question image: %w", err)
	}

	if err := s.saveImage(answerImg, answerPath); err != nil {
		return nil, fmt.Errorf("failed to save answer image: %w", err)
	}

	s.logger.Debug("Created question image: %s", questionPath)
	s.logger.Debug("Created answer image: %s", answerPath)

	return &ImagePair{
		Question: questionPath,
		Answer:   answerPath,
		Hash:     fullHash,
	}, nil
}

func (s *Splitter) saveImage(img *image.RGBA, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}
