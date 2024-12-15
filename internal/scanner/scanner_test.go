// internal/scanner/scanner_test.go
package scanner_test

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"log"
	"os"
	"path/filepath"

	"github.com/kpauljoseph/notesankify/internal/pdf"
	"github.com/kpauljoseph/notesankify/internal/scanner"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

type MockProcessor struct {
	ProcessPDFFunc func(ctx context.Context, pdfPath string) ([]models.FlashcardPage, error)
}

var _ pdf.PDFProcessor = (*MockProcessor)(nil)

func (m *MockProcessor) ProcessPDF(ctx context.Context, pdfPath string) ([]models.FlashcardPage, error) {
	if m.ProcessPDFFunc != nil {
		return m.ProcessPDFFunc(ctx, pdfPath)
	}
	return nil, nil
}

func (m *MockProcessor) Cleanup() error {
	return nil
}

var _ = Describe("Scanner", func() {
	var (
		tempDir    string
		testLogger *log.Logger
		ctx        context.Context
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "scanner-test-*")
		Expect(err).NotTo(HaveOccurred())

		testLogger = log.New(GinkgoWriter, "[test] ", log.LstdFlags)
		ctx = context.Background()
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Context("when scanning an empty directory", func() {
		It("should return an error", func() {
			mockProcessor := &MockProcessor{}
			s := scanner.New(mockProcessor, testLogger)

			_, err := s.ScanDirectory(ctx, tempDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no PDF files found"))
		})
	})

	Context("when scanning a directory with PDFs", func() {
		BeforeEach(func() {
			for i := 1; i <= 3; i++ {
				err := os.WriteFile(
					filepath.Join(tempDir, fmt.Sprintf("test%d.pdf", i)),
					[]byte("dummy pdf content"),
					0644,
				)
				Expect(err).NotTo(HaveOccurred())
			}

			err := os.WriteFile(
				filepath.Join(tempDir, "test.txt"),
				[]byte("text file"),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should process only PDF files", func() {
			processedFiles := make(map[string]bool)
			mockProcessor := &MockProcessor{
				ProcessPDFFunc: func(ctx context.Context, pdfPath string) ([]models.FlashcardPage, error) {
					processedFiles[filepath.Base(pdfPath)] = true
					return []models.FlashcardPage{
						{PDFPath: pdfPath, PageNum: 0},
						{PDFPath: pdfPath, PageNum: 1},
					}, nil
				},
			}

			s := scanner.New(mockProcessor, testLogger)
			stats, err := s.ScanDirectory(ctx, tempDir)

			Expect(err).NotTo(HaveOccurred())
			Expect(stats.PDFCount).To(Equal(3))
			Expect(stats.FlashcardCount).To(Equal(6)) // 2 flashcards * 3 PDFs

			// Verify only PDFs were processed
			Expect(processedFiles).To(HaveLen(3))
			Expect(processedFiles).To(HaveKey("test1.pdf"))
			Expect(processedFiles).To(HaveKey("test2.pdf"))
			Expect(processedFiles).To(HaveKey("test3.pdf"))
		})
	})

	Context("when scanning nested directories", func() {
		BeforeEach(func() {
			nestedDir := filepath.Join(tempDir, "nested")
			err := os.MkdirAll(nestedDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			files := []string{
				filepath.Join(tempDir, "root.pdf"),
				filepath.Join(nestedDir, "nested.pdf"),
			}

			for _, file := range files {
				err := os.WriteFile(file, []byte("dummy pdf content"), 0644)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should process PDFs in all subdirectories", func() {
			var processedPaths []string
			mockProcessor := &MockProcessor{
				ProcessPDFFunc: func(ctx context.Context, pdfPath string) ([]models.FlashcardPage, error) {
					processedPaths = append(processedPaths, filepath.Base(pdfPath))
					return []models.FlashcardPage{{PDFPath: pdfPath, PageNum: 0}}, nil
				},
			}

			s := scanner.New(mockProcessor, testLogger)
			stats, err := s.ScanDirectory(ctx, tempDir)

			Expect(err).NotTo(HaveOccurred())
			Expect(stats.PDFCount).To(Equal(2))
			Expect(stats.FlashcardCount).To(Equal(2))
			Expect(processedPaths).To(ConsistOf("root.pdf", "nested.pdf"))
		})
	})

	Context("when context is cancelled", func() {
		It("should stop processing", func() {
			for i := 1; i <= 3; i++ {
				err := os.WriteFile(
					filepath.Join(tempDir, fmt.Sprintf("test%d.pdf", i)),
					[]byte("dummy pdf content"),
					0644,
				)
				Expect(err).NotTo(HaveOccurred())
			}

			ctx, cancel := context.WithCancel(context.Background())
			processCount := 0

			mockProcessor := &MockProcessor{
				ProcessPDFFunc: func(ctx context.Context, pdfPath string) ([]models.FlashcardPage, error) {
					processCount++
					if processCount == 2 {
						cancel()
						return nil, ctx.Err()
					}
					return []models.FlashcardPage{{PDFPath: pdfPath, PageNum: 0}}, nil
				},
			}

			s := scanner.New(mockProcessor, testLogger)
			_, err := s.ScanDirectory(ctx, tempDir)

			Expect(err).To(Equal(context.Canceled))
			Expect(processCount).To(Equal(2))
		})
	})
})
