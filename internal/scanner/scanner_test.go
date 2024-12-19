package scanner_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/internal/scanner"
	"github.com/kpauljoseph/notesankify/pkg/logger"
)

var _ = Describe("Scanner", func() {
	var (
		testDir    string
		testLogger *logger.Logger
		ctx        context.Context
	)

	BeforeEach(func() {
		var err error
		testDir, err = os.MkdirTemp("", "scanner-test-*")
		Expect(err).NotTo(HaveOccurred())

		testLogger = logger.New(
			logger.WithOutput(GinkgoWriter),
			logger.WithPrefix("[test] "),
			logger.WithFlags(0), // Minimal flags for test output
		)
		testLogger.SetVerbose(true)
		testLogger.SetLevel(logger.LevelTrace)

		ctx = context.Background()
	})

	AfterEach(func() {
		os.RemoveAll(testDir)
	})

	When("when scanning an empty directory", func() {
		It("should return an error", func() {
			s := scanner.New(testLogger)
			_, err := s.FindPDFs(ctx, testDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no PDF files found"))
		})
	})

	When("when scanning a directory with PDFs", func() {
		BeforeEach(func() {
			for i := 1; i <= 3; i++ {
				err := os.WriteFile(
					filepath.Join(testDir, fmt.Sprintf("test%d.pdf", i)),
					[]byte("dummy pdf content"),
					0644,
				)
				Expect(err).NotTo(HaveOccurred())
			}

			err := os.WriteFile(
				filepath.Join(testDir, "test.txt"),
				[]byte("text file"),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should find only PDF files", func() {
			s := scanner.New(testLogger)
			pdfs, err := s.FindPDFs(ctx, testDir)

			Expect(err).NotTo(HaveOccurred())
			Expect(pdfs).To(HaveLen(3))

			for _, pdf := range pdfs {
				Expect(pdf.RelativePath).To(HaveSuffix(".pdf"))
			}
		})
	})

	When("when scanning nested directories", func() {
		BeforeEach(func() {
			nestedDir := filepath.Join(testDir, "nested")
			err := os.MkdirAll(nestedDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			files := []string{
				filepath.Join(testDir, "root.pdf"),
				filepath.Join(nestedDir, "nested.pdf"),
			}

			for _, file := range files {
				err := os.WriteFile(file, []byte("dummy pdf content"), 0644)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should find PDFs in all subdirectories", func() {
			s := scanner.New(testLogger)
			pdfs, err := s.FindPDFs(ctx, testDir)

			Expect(err).NotTo(HaveOccurred())
			Expect(pdfs).To(HaveLen(2))

			var filenames []string
			for _, pdf := range pdfs {
				filenames = append(filenames, filepath.Base(pdf.AbsolutePath))
			}
			Expect(filenames).To(ConsistOf("root.pdf", "nested.pdf"))
		})
	})

	When("when context is cancelled", func() {
		It("should stop scanning", func() {
			deepDir := filepath.Join(testDir, "deep", "deeper", "deepest")
			err := os.MkdirAll(deepDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			s := scanner.New(testLogger)
			_, err = s.FindPDFs(ctx, testDir)

			Expect(err).To(Equal(context.Canceled))
		})
	})
})
