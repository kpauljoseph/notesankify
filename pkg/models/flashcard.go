package models

import (
	"time"
)

type PageDimensions struct {
	Width  float64
	Height float64
}

type FlashcardPage struct {
	PDFPath   string
	PageNum   int
	ImagePath string
}

type Flashcard struct {
	ID        string    `json:"id"`
	DeckName  string    `json:"deck_name"`
	Front     string    `json:"front"`
	Back      string    `json:"back"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FlashcardMetadata struct {
	FlashcardID string    `json:"flashcard_id"`
	PDFPath     string    `json:"pdf_path"`
	PageNumbers []int     `json:"page_numbers"`
	Hash        string    `json:"hash"`
	LastSync    time.Time `json:"last_sync"`
}
