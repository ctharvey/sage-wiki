package extract

import (
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// extractPDF extracts text from a PDF file using ledongthuc/pdf (pure Go).
func extractPDF(path string) (*SourceContent, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("extract pdf: open: %w", err)
	}
	defer f.Close()

	var text strings.Builder
	numPages := r.NumPage()

	for i := 1; i <= numPages; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}

		content, err := page.GetPlainText(nil)
		if err != nil {
			// Skip pages that fail to extract
			continue
		}
		text.WriteString(content)
		text.WriteString("\n\n")
	}

	extracted := strings.TrimSpace(text.String())
	if extracted == "" {
		return nil, fmt.Errorf("extract pdf: no text content in %s (scanned PDF or images only)", path)
	}

	return &SourceContent{
		Path: path,
		Type: "paper",
		Text: extracted,
	}, nil
}
