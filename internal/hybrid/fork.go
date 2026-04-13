package hybrid

import "strings"

// ExtractDocumentShape extracts the document_shape value from YAML frontmatter.
// Returns "reference" if the frontmatter specifies document_shape: reference,
// otherwise returns "wiki" as the default shape.
func ExtractDocumentShape(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return "wiki"
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return "wiki"
	}
	fmText := content[4 : 4+end]
	for _, line := range strings.Split(fmText, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if key == "document_shape" {
				shape := strings.TrimSpace(strings.Trim(val, "\"'"))
				if shape == "reference" {
					return "reference"
				}
				return "wiki"
			}
		}
	}
	return "wiki"
}
