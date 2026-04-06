package extract

import (
	"archive/zip"
	"fmt"
	"path/filepath"
	"strings"
)

// extractEpub extracts text from an EPUB file (ZIP containing XHTML chapters).
func extractEpub(path string) (*SourceContent, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("extract epub: %w", err)
	}
	defer r.Close()

	var text strings.Builder
	chapterNum := 0

	for _, f := range r.File {
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext != ".xhtml" && ext != ".html" && ext != ".htm" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}

		content := extractXMLText(rc)
		rc.Close()

		if content != "" {
			chapterNum++
			text.WriteString(fmt.Sprintf("\n--- Chapter %d (%s) ---\n", chapterNum, filepath.Base(f.Name)))
			text.WriteString(content)
			text.WriteString("\n")
		}
	}

	extracted := strings.TrimSpace(text.String())
	if extracted == "" {
		return nil, fmt.Errorf("extract epub: no text content in %s", filepath.Base(path))
	}

	return &SourceContent{
		Path: path,
		Type: "article",
		Text: extracted,
	}, nil
}
