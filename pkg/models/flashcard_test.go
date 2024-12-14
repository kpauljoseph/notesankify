package models_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/models"
)

var _ = Describe("Flashcard Models", func() {
	Context("FlashcardPage", func() {
		It("should properly store page information", func() {
			page := models.FlashcardPage{
				PDFPath:   "/path/to/pdf",
				PageNum:   1,
				ImagePath: "/path/to/image",
			}

			Expect(page.PDFPath).To(Equal("/path/to/pdf"))
			Expect(page.PageNum).To(Equal(1))
			Expect(page.ImagePath).To(Equal("/path/to/image"))
		})
	})

	Context("Flashcard", func() {
		It("should properly initialize with all fields", func() {
			now := time.Now()
			card := models.Flashcard{
				ID:        "card1",
				DeckName:  "test-deck",
				Front:     "/path/to/front",
				Back:      "/path/to/back",
				Hash:      "hash123",
				CreatedAt: now,
				UpdatedAt: now,
			}

			Expect(card.ID).To(Equal("card1"))
			Expect(card.DeckName).To(Equal("test-deck"))
			Expect(card.Front).To(Equal("/path/to/front"))
			Expect(card.Back).To(Equal("/path/to/back"))
			Expect(card.Hash).To(Equal("hash123"))
			Expect(card.CreatedAt).To(Equal(now))
			Expect(card.UpdatedAt).To(Equal(now))
		})
	})

	Context("FlashcardMetadata", func() {
		It("should properly store metadata", func() {
			now := time.Now()
			metadata := models.FlashcardMetadata{
				FlashcardID: "card1",
				PDFPath:     "/path/to/pdf",
				PageNumbers: []int{1, 2},
				Hash:        "hash123",
				LastSync:    now,
			}

			Expect(metadata.FlashcardID).To(Equal("card1"))
			Expect(metadata.PDFPath).To(Equal("/path/to/pdf"))
			Expect(metadata.PageNumbers).To(Equal([]int{1, 2}))
			Expect(metadata.Hash).To(Equal("hash123"))
			Expect(metadata.LastSync).To(Equal(now))
		})
	})
})
