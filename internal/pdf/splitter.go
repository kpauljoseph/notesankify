package pdf

import (
	"fmt"
	"github.com/kpauljoseph/notesankify/pkg/logger"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

type ImagePair struct {
	Question string
	Answer   string
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

func (s *Splitter) SplitImage(imagePath string) (*ImagePair, error) {
	s.logger.Printf("Splitting image: %s", imagePath)

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

	baseFileName := strings.TrimSuffix(filepath.Base(imagePath), filepath.Ext(imagePath))
	questionPath := filepath.Join(s.outputDir, baseFileName+"_question.png")
	answerPath := filepath.Join(s.outputDir, baseFileName+"_answer.png")

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
	}, nil
}

func (s *Splitter) SplitAll(sourceDir string) ([]ImagePair, error) {
	var pairs []ImagePair

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".png") {
			continue
		}

		imagePath := filepath.Join(sourceDir, entry.Name())
		pair, err := s.SplitImage(imagePath)
		if err != nil {
			s.logger.Printf("Error splitting %s: %v", imagePath, err)
			continue
		}

		pairs = append(pairs, *pair)
	}

	return pairs, nil
}

func (s *Splitter) saveImage(img *image.RGBA, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}
