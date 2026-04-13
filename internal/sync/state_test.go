package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryRoundtrip(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "sync-registry.json")

	// Create a test entry
	projectPath := "/tmp/test-project"
	slug := "test-project"
	syncTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	docCount := 42

	// Record the sync
	if err := RecordSync(registryPath, projectPath, slug, syncTime, docCount); err != nil {
		t.Fatalf("RecordSync failed: %v", err)
	}

	// Load the registry back
	reg, err := LoadRegistry(registryPath)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	// Verify the entry survives
	absPath, _ := filepath.Abs(projectPath)
	entry, ok := reg.Projects[absPath]
	if !ok {
		t.Fatalf("Expected entry for %s not found", absPath)
	}

	if entry.AbsPath != absPath {
		t.Errorf("AbsPath mismatch: got %q, want %q", entry.AbsPath, absPath)
	}
	if entry.Slug != slug {
		t.Errorf("Slug mismatch: got %q, want %q", entry.Slug, slug)
	}
	if !entry.LastSyncAt.Equal(syncTime) {
		t.Errorf("LastSyncAt mismatch: got %v, want %v", entry.LastSyncAt, syncTime)
	}
	if entry.DocCount != docCount {
		t.Errorf("DocCount mismatch: got %d, want %d", entry.DocCount, docCount)
	}
}

func TestLastSyncMiss(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "sync-registry.json")

	// Try to get last sync for an unknown project
	unknownProject := "/tmp/unknown-project"
	lastSync, ok, err := LastSync(registryPath, unknownProject)
	if err != nil {
		t.Fatalf("LastSync failed: %v", err)
	}
	if ok {
		t.Error("Expected ok=false for unknown project, got true")
	}
	if !lastSync.IsZero() {
		t.Errorf("Expected zero time for unknown project, got %v", lastSync)
	}
}

func TestListSyncedOrder(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "sync-registry.json")

	// Create two entries with different times
	project1 := "/tmp/project-alpha"
	project2 := "/tmp/project-beta"

	// Project 2 synced more recently
	time1 := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
	time2 := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	if err := RecordSync(registryPath, project1, "alpha", time1, 10); err != nil {
		t.Fatalf("RecordSync for project1 failed: %v", err)
	}
	if err := RecordSync(registryPath, project2, "beta", time2, 20); err != nil {
		t.Fatalf("RecordSync for project2 failed: %v", err)
	}

	// List synced entries
	entries, err := ListSynced(registryPath)
	if err != nil {
		t.Fatalf("ListSynced failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	// Verify most recent first (project2 should be first)
	if entries[0].Slug != "beta" {
		t.Errorf("Expected first entry to be 'beta' (most recent), got %q", entries[0].Slug)
	}
	if entries[1].Slug != "alpha" {
		t.Errorf("Expected second entry to be 'alpha' (older), got %q", entries[1].Slug)
	}

	// Verify the times are in descending order
	if !entries[0].LastSyncAt.After(entries[1].LastSyncAt) {
		t.Error("Entries not sorted by LastSyncAt descending")
	}
}

func TestRegistryPath(t *testing.T) {
	wikiDir := "/home/user/wiki"
	expected := filepath.Join(wikiDir, ".sage", "sync-registry.json")
	got := RegistryPath(wikiDir)
	if got != expected {
		t.Errorf("RegistryPath mismatch: got %q, want %q", got, expected)
	}
}

func TestLoadRegistryNotExist(t *testing.T) {
	// Try to load a registry that doesn't exist
	tmpDir := t.TempDir()
	nonExistentPath := filepath.Join(tmpDir, "non-existent", "sync-registry.json")

	reg, err := LoadRegistry(nonExistentPath)
	if err != nil {
		t.Fatalf("LoadRegistry should not error for non-existent file: %v", err)
	}
	if reg == nil {
		t.Fatal("Expected non-nil registry for non-existent file")
	}
	if reg.Projects == nil {
		t.Fatal("Expected initialized Projects map for non-existent file")
	}
	if len(reg.Projects) != 0 {
		t.Errorf("Expected empty Projects map, got %d entries", len(reg.Projects))
	}
}

func TestSaveRegistryCreatesDirs(t *testing.T) {
	// Create a path in a nested directory that doesn't exist yet
	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "a", "b", "c", "sync-registry.json")

	reg := &SyncRegistry{
		Projects: map[string]SyncEntry{
			"/tmp/test": {
				AbsPath:    "/tmp/test",
				Slug:       "test",
				LastSyncAt: time.Now(),
				DocCount:   1,
			},
		},
	}

	if err := SaveRegistry(deepPath, reg); err != nil {
		t.Fatalf("SaveRegistry failed to create directories: %v", err)
	}

	// Verify the file was created
	if _, err := os.Stat(deepPath); os.IsNotExist(err) {
		t.Error("SaveRegistry did not create the file")
	}
}
