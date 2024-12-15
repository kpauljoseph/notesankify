package pdf

import (
	"context"
	"github.com/kpauljoseph/notesankify/pkg/models"
)

type PDFProcessor interface {
	ProcessPDF(ctx context.Context, pdfPath string) ([]models.FlashcardPage, error)
	Cleanup() error
}
