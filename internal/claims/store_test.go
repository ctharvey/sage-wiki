package claims

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xoai/sage-wiki/internal/storage"
)

func setupTestStore(t *testing.T) (*Store, func()) {
	// Use a temp file instead of :memory: because storage.DB uses separate
	// read/write connections, and each :memory: connection is isolated
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	store := NewStore(db)
	if err := store.Init(); err != nil {
		db.Close()
		t.Fatalf("failed to init store: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestAddAndGetByConcept(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Add claims for concept-1
	claim1 := Claim{
		ID:         "claim:concept-1:0",
		ConceptID:  "concept-1",
		Section:    "key_claims",
		Text:       "First key claim",
		SourcePath: "/wiki/article1",
	}
	claim2 := Claim{
		ID:          "claim:concept-1:1",
		ConceptID:   "concept-1",
		Section:     "evidence",
		Text:        "Evidence claim",
		SourcePath:  "/wiki/article2",
		SourceQuote: "verbatim quote",
	}

	if err := store.Add(claim1); err != nil {
		t.Fatalf("Add claim1 failed: %v", err)
	}
	if err := store.Add(claim2); err != nil {
		t.Fatalf("Add claim2 failed: %v", err)
	}

	// Retrieve claims for concept-1
	claims, err := store.GetByConcept("concept-1")
	if err != nil {
		t.Fatalf("GetByConcept failed: %v", err)
	}

	if len(claims) != 2 {
		t.Errorf("expected 2 claims, got %d", len(claims))
	}

	// Verify claim data
	claimMap := make(map[string]Claim)
	for _, c := range claims {
		claimMap[c.ID] = c
	}

	if c, ok := claimMap["claim:concept-1:0"]; !ok {
		t.Error("claim:concept-1:0 not found")
	} else {
		if c.Text != "First key claim" {
			t.Errorf("expected text 'First key claim', got %q", c.Text)
		}
		if c.Section != "key_claims" {
			t.Errorf("expected section 'key_claims', got %q", c.Section)
		}
		if c.SourcePath != "/wiki/article1" {
			t.Errorf("expected source_path '/wiki/article1', got %q", c.SourcePath)
		}
	}

	if c, ok := claimMap["claim:concept-1:1"]; !ok {
		t.Error("claim:concept-1:1 not found")
	} else {
		if c.Text != "Evidence claim" {
			t.Errorf("expected text 'Evidence claim', got %q", c.Text)
		}
		if c.SourceQuote != "verbatim quote" {
			t.Errorf("expected source_quote 'verbatim quote', got %q", c.SourceQuote)
		}
	}

	// Verify no claims for different concept
	claims, err = store.GetByConcept("concept-2")
	if err != nil {
		t.Fatalf("GetByConcept for concept-2 failed: %v", err)
	}
	if len(claims) != 0 {
		t.Errorf("expected 0 claims for concept-2, got %d", len(claims))
	}
}

func TestDeleteByConcept(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Add claims for multiple concepts
	claims := []Claim{
		{ID: "claim:c1:0", ConceptID: "c1", Section: "key_claims", Text: "C1 Claim 1"},
		{ID: "claim:c1:1", ConceptID: "c1", Section: "evidence", Text: "C1 Claim 2"},
		{ID: "claim:c2:0", ConceptID: "c2", Section: "key_claims", Text: "C2 Claim 1"},
		{ID: "claim:c3:0", ConceptID: "c3", Section: "key_claims", Text: "C3 Claim 1"},
	}

	for _, c := range claims {
		if err := store.Add(c); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Verify initial count
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 4 {
		t.Errorf("expected count 4, got %d", count)
	}

	// Delete all claims for c1
	if err := store.DeleteByConcept("c1"); err != nil {
		t.Fatalf("DeleteByConcept failed: %v", err)
	}

	// Verify c1 claims are gone
	c1Claims, err := store.GetByConcept("c1")
	if err != nil {
		t.Fatalf("GetByConcept for c1 failed: %v", err)
	}
	if len(c1Claims) != 0 {
		t.Errorf("expected 0 claims for c1 after delete, got %d", len(c1Claims))
	}

	// Verify c2 and c3 claims remain
	c2Claims, err := store.GetByConcept("c2")
	if err != nil {
		t.Fatalf("GetByConcept for c2 failed: %v", err)
	}
	if len(c2Claims) != 1 {
		t.Errorf("expected 1 claim for c2, got %d", len(c2Claims))
	}

	c3Claims, err := store.GetByConcept("c3")
	if err != nil {
		t.Fatalf("GetByConcept for c3 failed: %v", err)
	}
	if len(c3Claims) != 1 {
		t.Errorf("expected 1 claim for c3, got %d", len(c3Claims))
	}

	// Verify count after delete
	count, err = store.Count()
	if err != nil {
		t.Fatalf("Count after delete failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2 after delete, got %d", count)
	}
}

func TestCount(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Initial count should be 0
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected initial count 0, got %d", count)
	}

	// Add claims
	for i := 0; i < 5; i++ {
		c := Claim{
			ID:        "claim:test:" + string(rune('0'+i)),
			ConceptID: "test",
			Section:   "key_claims",
			Text:      "Claim " + string(rune('0'+i)),
		}
		if err := store.Add(c); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Count should be 5
	count, err = store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected count 5, got %d", count)
	}
}

func TestAddDuplicateReplaces(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Add initial claim
	claim := Claim{
		ID:         "claim:dup:0",
		ConceptID:  "dup",
		Section:    "key_claims",
		Text:       "Original text",
		SourcePath: "/original",
	}
	if err := store.Add(claim); err != nil {
		t.Fatalf("Add initial claim failed: %v", err)
	}

	// Verify initial claim
	claims, err := store.GetByConcept("dup")
	if err != nil {
		t.Fatalf("GetByConcept failed: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	if claims[0].Text != "Original text" {
		t.Errorf("expected text 'Original text', got %q", claims[0].Text)
	}

	// Add duplicate with same ID but different content (should replace)
	updatedClaim := Claim{
		ID:         "claim:dup:0",
		ConceptID:  "dup",
		Section:    "evidence",
		Text:       "Updated text",
		SourcePath: "/updated",
	}
	if err := store.Add(updatedClaim); err != nil {
		t.Fatalf("Add duplicate claim failed: %v", err)
	}

	// Verify claim was replaced, not duplicated
	claims, err = store.GetByConcept("dup")
	if err != nil {
		t.Fatalf("GetByConcept after update failed: %v", err)
	}
	if len(claims) != 1 {
		t.Errorf("expected 1 claim after replace, got %d", len(claims))
	}
	if claims[0].Text != "Updated text" {
		t.Errorf("expected text 'Updated text' after replace, got %q", claims[0].Text)
	}
	if claims[0].Section != "evidence" {
		t.Errorf("expected section 'evidence' after replace, got %q", claims[0].Section)
	}
	if claims[0].SourcePath != "/updated" {
		t.Errorf("expected source_path '/updated' after replace, got %q", claims[0].SourcePath)
	}

	// Verify count is still 1
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1 after replace, got %d", count)
	}
}
