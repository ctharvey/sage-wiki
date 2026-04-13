package compiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/xoai/sage-wiki/internal/claims"
	"github.com/xoai/sage-wiki/internal/log"
	"github.com/xoai/sage-wiki/internal/storage"
)

func init() {
	postWriteArticleHook = forkPostWriteArticle
}

func forkPostWriteArticle(db *storage.DB, conceptID, content string) {
	if db == nil {
		return
	}
	store := claims.NewStore(db)
	if err := store.Init(); err != nil {
		log.Warn("claims init failed", "concept", conceptID, "error", err)
		return
	}
	if err := extractAndStoreClaims(conceptID, content, store); err != nil {
		log.Warn("claims extraction failed", "concept", conceptID, "error", err)
	}
}

// extractAndStoreClaims parses structured sections from a reference article
// and stores typed claim records. Idempotent — deletes existing claims first.
func extractAndStoreClaims(conceptID, content string, store *claims.Store) error {
	var extracted []claims.Claim
	idx := 0
	lines := strings.Split(content, "\n")
	inKeyClaims := false
	inEvidence := false
	headerSeen := false
	sourceRe := regexp.MustCompile(`\[Source:\s*([^\]]+)\]`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			section := strings.TrimPrefix(trimmed, "## ")
			inKeyClaims = strings.EqualFold(section, "Key Claims")
			inEvidence = strings.EqualFold(section, "Evidence")
			headerSeen = false
			continue
		}

		if inKeyClaims && strings.HasPrefix(trimmed, "|") {
			cols := strings.Split(trimmed, "|")
			if len(cols) < 3 {
				continue
			}
			cell0 := strings.TrimSpace(cols[1])
			if !headerSeen {
				headerSeen = true
				continue
			}
			if strings.Contains(cell0, "---") {
				continue
			}
			claimText := cell0
			sourcePath := ""
			if len(cols) >= 3 {
				rawSrc := strings.TrimSpace(cols[2])
				if m := sourceRe.FindStringSubmatch(rawSrc); len(m) > 1 {
					sourcePath = strings.TrimSpace(m[1])
				} else {
					sourcePath = strings.Trim(rawSrc, "[] ")
				}
			}
			if claimText == "" {
				continue
			}
			extracted = append(extracted, claims.Claim{
				ID:         fmt.Sprintf("claim:%s:%d", conceptID, idx),
				ConceptID:  conceptID,
				Section:    "key_claims",
				Text:       claimText,
				SourcePath: sourcePath,
			})
			idx++
		}

		if inEvidence && strings.HasPrefix(trimmed, "- ") {
			text := strings.TrimPrefix(trimmed, "- ")
			sourcePath := ""
			if m := sourceRe.FindStringSubmatch(text); len(m) > 1 {
				sourcePath = strings.TrimSpace(m[1])
				text = strings.TrimSpace(sourceRe.ReplaceAllString(text, ""))
			}
			if text == "" {
				continue
			}
			extracted = append(extracted, claims.Claim{
				ID:         fmt.Sprintf("claim:%s:%d", conceptID, idx),
				ConceptID:  conceptID,
				Section:    "evidence",
				Text:       text,
				SourcePath: sourcePath,
			})
			idx++
		}
	}

	if err := store.DeleteByConcept(conceptID); err != nil {
		return err
	}
	for _, c := range extracted {
		if err := store.Add(c); err != nil {
			return err
		}
	}
	return nil
}
