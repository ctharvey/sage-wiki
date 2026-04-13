package wiki

import (
	"os"
	"path/filepath"

	"github.com/xoai/sage-wiki/internal/hybrid"
	"github.com/xoai/sage-wiki/internal/manifest"
)

func init() {
	postGetStatusShapeHook = countShapes
}

func countShapes(projectDir string, info *StatusInfo) {
	mfPath := filepath.Join(projectDir, ".manifest.json")
	mf, err := manifest.Load(mfPath)
	if err != nil {
		return
	}

	for _, concept := range mf.Concepts {
		data, err := os.ReadFile(filepath.Join(projectDir, concept.ArticlePath))
		if err != nil {
			continue
		}
		if hybrid.ExtractDocumentShape(string(data)) == "reference" {
			info.ReferenceCount++
		} else {
			info.WikiCount++
		}
	}
}
