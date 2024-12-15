package scanner_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/internal/scanner"
)

var _ = Describe("Scanner", func() {
	var (
		testDir    string
		testLogger *log.Logger
		ctx        context.Context
	)

	BeforeEach(func() {
		var err error
		testDir, err = os.MkdirTemp("", "scanner-test-*")
		Expect(err).NotTo(HaveOccurred())

		testLogger = log.New(GinkgoWriter, "[test] ", log.LstdFlags)
		ctx = context.Background()
	})

	AfterEach(func() {
		os.RemoveAll(testDir)
	})

	Context("when scanning an empty directory", func() {
		It("should return an error", func() {
			s := scanner.New(testLogger)
			_, err := s.FindPDFs(ctx, testDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no PDF files found"))
		})
	})

	Context("when scanning a directory with PDFs", func() {
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
				Expect(pdf).To(HaveSuffix(".pdf"))
			}
		})
	})

	Context("when scanning nested directories", func() {
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
				filenames = append(filenames, filepath.Base(pdf))
			}
			Expect(filenames).To(ConsistOf("root.pdf", "nested.pdf"))
		})
	})

	Context("when context is cancelled", func() {
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
