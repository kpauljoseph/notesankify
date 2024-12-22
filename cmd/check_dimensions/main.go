package main

import (
	"flag"
	"fmt"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"os"
)

func main() {
	pdfPath := flag.String("file", "", "Path to PDF file")
	flag.Parse()

	if *pdfPath == "" {
		fmt.Println("Please provide a PDF file path using -file flag")
		os.Exit(1)
	}

	fmt.Printf("Analyzing PDF: %s\n", *pdfPath)

	// Get page dimensions
	dims, err := api.PageDimsFile(*pdfPath)
	if err != nil {
		fmt.Printf("Error getting page dimensions: %v\n", err)
		os.Exit(1)
	}

	// Process each page's dimensions
	for i, dim := range dims {
		fmt.Printf("\nPage %d:\n", i+1)
		fmt.Printf("Dimensions (Width x Height): %.3f x %.3f points\n", dim.Width, dim.Height)
	}
}
