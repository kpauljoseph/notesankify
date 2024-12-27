package utils

import "os"

func GetDefaultOutputDir() string {
	tmpDir, err := os.MkdirTemp("", "notesankify-output-*")
	if err != nil {
		// If we can't create a temp directory, fall back to local directory
		return "notesankify-flashcards"
	}
	return tmpDir
}
