package compiler

import (
	"github.com/xoai/sage-wiki/internal/log"
)

// ExtractImages runs Pass 4: image handling.
// Images are now processed inline during Pass 1 (summarize) via vision LLM.
// This pass logs a summary of image sources found.
func ExtractImages(projectDir string, outputDir string, sources []SourceInfo) {
	imageCount := 0
	for _, s := range sources {
		if s.Type == "image" {
			imageCount++
		}
	}

	if imageCount > 0 {
		log.Info("Pass 4: image sources processed via vision in Pass 1", "image_sources", imageCount)
	}
}
