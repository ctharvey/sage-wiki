package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xoai/sage-wiki/internal/manifest"
	"github.com/xoai/sage-wiki/internal/memory"
	"github.com/xoai/sage-wiki/internal/ontology"
	"github.com/xoai/sage-wiki/internal/storage"
	"github.com/xoai/sage-wiki/internal/vectors"
)

func TestGetStatusShapeCounts(t *testing.T) {
	dir := t.TempDir()

	// Initialize greenfield project
	if err := InitGreenfield(dir, "test-wiki", "gemini-2.5-flash"); err != nil {
		t.Fatalf("InitGreenfield: %v", err)
	}

	// Create test concept articles with different shapes
	conceptsDir := filepath.Join(dir, "wiki", "concepts")
	os.MkdirAll(conceptsDir, 0755)

	// Wiki document (no document_shape)
	os.WriteFile(filepath.Join(conceptsDir, "wiki-concept.md"), []byte(`---
concept: wiki-concept
---

# Wiki Concept

This is a wiki-style article.
`), 0644)

	// Reference document
	os.WriteFile(filepath.Join(conceptsDir, "ref-concept.md"), []byte(`---
concept: ref-concept
document_shape: reference
---

## Scope
This is a reference article.
`), 0644)

	// Another reference document
	os.WriteFile(filepath.Join(conceptsDir, "another-ref.md"), []byte(`---
concept: another-ref
document_shape: reference
---

## Key Claims
| Claim | Source | Confidence | Notes |
|---|---|---|---|
| Test claim | [Source: raw/test.md] | high | Test notes |
`), 0644)

	// Create manifest with concept entries
	mf := manifest.New()
	mf.Sources["raw/source1.md"] = manifest.Source{
		Hash:   "abc123",
		Type:   "auto",
		Status: "compiled",
	}
	mf.Concepts["wiki-concept"] = manifest.Concept{
		ArticlePath: "wiki/concepts/wiki-concept.md",
		Sources:     []string{"raw/source1.md"},
	}
	mf.Concepts["ref-concept"] = manifest.Concept{
		ArticlePath: "wiki/concepts/ref-concept.md",
		Sources:     []string{"raw/source1.md"},
	}
	mf.Concepts["another-ref"] = manifest.Concept{
		ArticlePath: "wiki/concepts/another-ref.md",
		Sources:     []string{"raw/source1.md"},
	}
	mf.Save(filepath.Join(dir, ".manifest.json"))

	// Open database
	dbPath := filepath.Join(dir, ".sage", "wiki.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open db: %v", err)
	}
	defer db.Close()

	memStore := memory.NewStore(db)
	vecStore := vectors.NewStore(db)
	onStore := ontology.NewStore(db, nil, nil)

	// Get status
	stores := &Stores{Mem: memStore, Vec: vecStore, Ont: onStore}
	info, err := GetStatus(dir, stores)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}

	// Verify counts
	if info.ConceptCount != 3 {
		t.Errorf("expected ConceptCount=3, got %d", info.ConceptCount)
	}
	if info.ReferenceCount != 2 {
		t.Errorf("expected ReferenceCount=2, got %d", info.ReferenceCount)
	}
	if info.WikiCount != 1 {
		t.Errorf("expected WikiCount=1, got %d", info.WikiCount)
	}

	// Verify sum equals total
	if info.ReferenceCount+info.WikiCount != info.ConceptCount {
		t.Errorf("ReferenceCount + WikiCount (%d) != ConceptCount (%d)",
			info.ReferenceCount+info.WikiCount, info.ConceptCount)
	}

	// Verify FormatStatus includes shape breakdown
	formatted := FormatStatus(info)
	if !strings.Contains(formatted, "Wiki:") {
		t.Error("FormatStatus should include 'Wiki:' breakdown")
	}
	if !strings.Contains(formatted, "Reference:") {
		t.Error("FormatStatus should include 'Reference:' breakdown")
	}
}

func TestGetStatusMissingFiles(t *testing.T) {
	dir := t.TempDir()

	// Initialize greenfield project
	if err := InitGreenfield(dir, "test-wiki", "gemini-2.5-flash"); err != nil {
		t.Fatalf("InitGreenfield: %v", err)
	}

	// Create manifest with concept entries but no files
	mf := manifest.New()
	mf.Concepts["missing-wiki"] = manifest.Concept{
		ArticlePath: "wiki/concepts/missing-wiki.md",
		Sources:     []string{"raw/source.md"},
	}
	mf.Concepts["missing-ref"] = manifest.Concept{
		ArticlePath: "wiki/concepts/missing-ref.md",
		Sources:     []string{"raw/source.md"},
	}
	mf.Save(filepath.Join(dir, ".manifest.json"))

	// Open database
	dbPath := filepath.Join(dir, ".sage", "wiki.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open db: %v", err)
	}
	defer db.Close()

	memStore := memory.NewStore(db)
	vecStore := vectors.NewStore(db)
	onStore := ontology.NewStore(db, nil, nil)

	// Get status - should not fail for missing files
	stores := &Stores{Mem: memStore, Vec: vecStore, Ont: onStore}
	info, err := GetStatus(dir, stores)
	if err != nil {
		t.Fatalf("GetStatus should not fail for missing files: %v", err)
	}

	// Missing files should not be counted (count only what's readable)
	if info.WikiCount != 0 {
		t.Errorf("expected WikiCount=0 for missing files, got %d", info.WikiCount)
	}
	if info.ReferenceCount != 0 {
		t.Errorf("expected ReferenceCount=0 for missing files, got %d", info.ReferenceCount)
	}
	if info.ConceptCount != 2 {
		t.Errorf("expected ConceptCount=2 from manifest, got %d", info.ConceptCount)
	}
}

func TestGetStatusZeroReferences(t *testing.T) {
	dir := t.TempDir()

	// Initialize greenfield project
	if err := InitGreenfield(dir, "test-wiki", "gemini-2.5-flash"); err != nil {
		t.Fatalf("InitGreenfield: %v", err)
	}

	// Create only wiki documents
	conceptsDir := filepath.Join(dir, "wiki", "concepts")
	os.MkdirAll(conceptsDir, 0755)

	os.WriteFile(filepath.Join(conceptsDir, "wiki1.md"), []byte(`---
concept: wiki1
---

# Wiki 1
`), 0644)
	os.WriteFile(filepath.Join(conceptsDir, "wiki2.md"), []byte(`---
concept: wiki2
---

# Wiki 2
`), 0644)

	// Create manifest
	mf := manifest.New()
	mf.Concepts["wiki1"] = manifest.Concept{
		ArticlePath: "wiki/concepts/wiki1.md",
		Sources:     []string{"raw/source.md"},
	}
	mf.Concepts["wiki2"] = manifest.Concept{
		ArticlePath: "wiki/concepts/wiki2.md",
		Sources:     []string{"raw/source.md"},
	}
	mf.Save(filepath.Join(dir, ".manifest.json"))

	// Open database
	dbPath := filepath.Join(dir, ".sage", "wiki.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open db: %v", err)
	}
	defer db.Close()

	memStore := memory.NewStore(db)
	vecStore := vectors.NewStore(db)
	onStore := ontology.NewStore(db, nil, nil)

	// Get status
	stores := &Stores{Mem: memStore, Vec: vecStore, Ont: onStore}
	info, err := GetStatus(dir, stores)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}

	// Should show 0 references
	if info.ReferenceCount != 0 {
		t.Errorf("expected ReferenceCount=0, got %d", info.ReferenceCount)
	}
	if info.WikiCount != 2 {
		t.Errorf("expected WikiCount=2, got %d", info.WikiCount)
	}

	// Format should show References: 0
	formatted := FormatStatus(info)
	if !strings.Contains(formatted, "Reference: 0") {
		t.Error("FormatStatus should show 'Reference: 0' when no references")
	}
}
