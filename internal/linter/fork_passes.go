package linter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xoai/sage-wiki/internal/hybrid"
)

func init() {
	RegisterPass(&ProvenancePass{})
	RegisterPass(&ClaimsContradictionPass{})
}

// --- Provenance Pass ---

type ProvenancePass struct{}

func (p *ProvenancePass) Name() string { return "provenance" }
func (p *ProvenancePass) CanAutoFix() bool { return false }
func (p *ProvenancePass) Fix(_ *LintContext, _ []Finding) error { return nil }

func (p *ProvenancePass) Run(ctx *LintContext) ([]Finding, error) {
	var findings []Finding
	conceptsDir := filepath.Join(ctx.ProjectDir, ctx.OutputDir, "concepts")
	entries, err := os.ReadDir(conceptsDir)
	if err != nil {
		return nil, nil
	}

	sourceRe := regexp.MustCompile(`\[Source:[^\]]+\]`)
	inferredRe := regexp.MustCompile(`\^\[inferred\]`)
	ambiguousRe := regexp.MustCompile(`\^\[ambiguous\]`)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(conceptsDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		docShape := hybrid.ExtractDocumentShape(content)
		if docShape != "reference" {
			continue
		}

		sourceMarkers := len(sourceRe.FindAllString(content, -1))
		inferredMarkers := len(inferredRe.FindAllString(content, -1))
		ambiguousMarkers := len(ambiguousRe.FindAllString(content, -1))

		claimLines := 0
		lines := strings.Split(content, "\n")
		inKeyClaims := false
		inEvidence := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "## Key Claims") {
				inKeyClaims = true
				inEvidence = false
				continue
			}
			if strings.HasPrefix(trimmed, "## Evidence") {
				inKeyClaims = false
				inEvidence = true
				continue
			}
			if strings.HasPrefix(trimmed, "## ") {
				inKeyClaims = false
				inEvidence = false
				continue
			}
			if inKeyClaims && strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
				claimLines++
			}
			if inEvidence && strings.HasPrefix(trimmed, "- ") {
				claimLines++
			}
		}

		maxClaims := claimLines
		if maxClaims < 1 {
			maxClaims = 1
		}
		pct := float64(sourceMarkers) / float64(maxClaims)
		if pct < 0.8 {
			findings = append(findings, Finding{
				Pass:     "provenance",
				Severity: SevWarning,
				Path:     filepath.Join(ctx.OutputDir, "concepts", e.Name()),
				Message:  fmt.Sprintf("provenance: %.0f%% source-backed (%d/%d claims), %d inferred, %d ambiguous", pct*100, sourceMarkers, claimLines, inferredMarkers, ambiguousMarkers),
			})
		} else {
			findings = append(findings, Finding{
				Pass:     "provenance",
				Severity: SevInfo,
				Path:     filepath.Join(ctx.OutputDir, "concepts", e.Name()),
				Message:  fmt.Sprintf("provenance: %.0f%% source-backed (%d/%d claims), %d inferred, %d ambiguous [pass]", pct*100, sourceMarkers, claimLines, inferredMarkers, ambiguousMarkers),
			})
		}
	}
	return findings, nil
}

// --- Claims Contradiction Pass ---

type ClaimsContradictionPass struct{}

func (p *ClaimsContradictionPass) Name() string { return "claims_contradiction" }
func (p *ClaimsContradictionPass) CanAutoFix() bool { return false }
func (p *ClaimsContradictionPass) Fix(_ *LintContext, _ []Finding) error { return nil }

func (p *ClaimsContradictionPass) Run(ctx *LintContext) ([]Finding, error) {
	if ctx.DB == nil {
		return nil, nil
	}
	rows, err := ctx.DB.ReadDB().Query("SELECT concept_id, text FROM claims ORDER BY concept_id")
	if err != nil {
		return nil, nil
	}
	defer rows.Close()

	conceptClaims := map[string][]string{}
	for rows.Next() {
		var conceptID, text string
		if err := rows.Scan(&conceptID, &text); err != nil {
			continue
		}
		conceptClaims[conceptID] = append(conceptClaims[conceptID], text)
	}

	negRe := regexp.MustCompile(`(?i)\b(not|no|never|cannot|can't|isn't|aren't|doesn't|don't|didn't)\b`)
	var findings []Finding
	for conceptID, clms := range conceptClaims {
		for i, claimA := range clms {
			for _, claimB := range clms[i+1:] {
				aHasNeg := negRe.MatchString(claimA)
				bHasNeg := negRe.MatchString(claimB)
				if aHasNeg == bHasNeg {
					continue
				}
				if wordOverlapCount(sigWords(claimA), sigWords(claimB)) >= 3 {
					findings = append(findings, Finding{
						Pass:     "claims_contradiction",
						Severity: SevWarning,
						Message:  fmt.Sprintf("[%s] conflicting claims: A: %q B: %q", conceptID, claimTrunc(claimA, 120), claimTrunc(claimB, 120)),
					})
				}
			}
		}
	}
	return findings, nil
}

func sigWords(s string) []string {
	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "in": true, "of": true, "to": true,
		"and": true, "or": true, "for": true, "with": true, "as": true,
		"that": true, "this": true, "it": true, "be": true, "has": true,
	}
	var words []string
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,;:!?\"'()[]^")
		if len(w) > 3 && !stopwords[w] {
			words = append(words, w)
		}
	}
	return words
}

func wordOverlapCount(a, b []string) int {
	set := map[string]bool{}
	for _, w := range a {
		set[w] = true
	}
	count := 0
	for _, w := range b {
		if set[w] {
			count++
		}
	}
	return count
}

func claimTrunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
