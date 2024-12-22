package main

import (
	"fmt"
	"github.com/gen2brain/go-fitz"
	"github.com/kpauljoseph/notesankify/pkg/utils"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: debug_pdf file1.pdf file2.pdf")
		os.Exit(1)
	}

	pdf1Path := os.Args[1]
	pdf2Path := os.Args[2]

	// Create temp directory for image comparison
	tempDir, err := os.MkdirTemp("", "pdf-debug-*")
	if err != nil {
		fmt.Printf("Error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	// Compare the PDFs
	doc1, err := fitz.New(pdf1Path)
	if err != nil {
		fmt.Printf("Error opening first PDF: %v\n", err)
		os.Exit(1)
	}
	defer doc1.Close()

	doc2, err := fitz.New(pdf2Path)
	if err != nil {
		fmt.Printf("Error opening second PDF: %v\n", err)
		os.Exit(1)
	}
	defer doc2.Close()

	// Compare basic properties
	fmt.Printf("\nBasic Properties:\n")
	fmt.Printf("PDF 1 pages: %d\n", doc1.NumPage())
	fmt.Printf("PDF 2 pages: %d\n", doc2.NumPage())

	// Compare each page
	maxPages := doc1.NumPage()
	if doc2.NumPage() < maxPages {
		maxPages = doc2.NumPage()
	}

	for pageNum := 0; pageNum < maxPages; pageNum++ {
		fmt.Printf("\nAnalyzing Page %d:\n", pageNum+1)

		// Compare dimensions
		bounds1, _ := doc1.Bound(pageNum)
		bounds2, _ := doc2.Bound(pageNum)
		fmt.Printf("PDF 1 dimensions: %.2f x %.2f\n", float64(bounds1.Dx()), float64(bounds1.Dy()))
		fmt.Printf("PDF 2 dimensions: %.2f x %.2f\n", float64(bounds2.Dx()), float64(bounds2.Dy()))

		// Compare text content
		text1, _ := doc1.Text(pageNum)
		text2, _ := doc2.Text(pageNum)
		fmt.Printf("\nText content identical: %v\n", text1 == text2)
		if text1 != text2 {
			fmt.Printf("\nPDF 1 text:\n%s\n", text1)
			fmt.Printf("\nPDF 2 text:\n%s\n", text2)
		}

		// Extract and compare images
		img1, err := doc1.Image(pageNum)
		if err != nil {
			fmt.Printf("Error extracting image from PDF 1: %v\n", err)
			continue
		}

		img2, err := doc2.Image(pageNum)
		if err != nil {
			fmt.Printf("Error extracting image from PDF 2: %v\n", err)
			continue
		}

		// Save images for manual inspection
		img1Path := filepath.Join(tempDir, fmt.Sprintf("page%d_pdf1.png", pageNum+1))
		img2Path := filepath.Join(tempDir, fmt.Sprintf("page%d_pdf2.png", pageNum+1))

		f1, _ := os.Create(img1Path)
		png.Encode(f1, img1)
		f1.Close()

		f2, _ := os.Create(img2Path)
		png.Encode(f2, img2)
		f2.Close()

		// Generate and compare hashes
		hash1, _ := utils.GenerateImageHash(img1)
		hash2, _ := utils.GenerateImageHash(img2)

		fmt.Printf("\nImage comparison:\n")
		fmt.Printf("PDF 1 hash: %s\n", hash1)
		fmt.Printf("PDF 2 hash: %s\n", hash2)
		fmt.Printf("Hashes match: %v\n", hash1 == hash2)

		// Save images for inspection
		fmt.Printf("\nSaved page images to:\n")
		fmt.Printf("PDF 1: %s\n", img1Path)
		fmt.Printf("PDF 2: %s\n", img2Path)
	}
}
