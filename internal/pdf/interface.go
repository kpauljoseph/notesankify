package pdf

import (
	"context"
)

type PDFProcessor interface {
	ProcessPDF(ctx context.Context, pdfPath string) (ProcessingStats, error)
	Cleanup() error
}
