package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SyncEntry records one synced project.
type SyncEntry struct {
	AbsPath    string    `json:"abs_path"`
	Slug       string    `json:"slug"`
	LastSyncAt time.Time `json:"last_sync_at"`
	DocCount   int       `json:"doc_count"`
}

// SyncRegistry holds the full registry.
type SyncRegistry struct {
	Projects map[string]SyncEntry `json:"projects"` // keyed by abs_path
}

// RegistryPath returns the canonical path to the sync registry file.
// It lives at <wikiDir>/.sage/sync-registry.json.
func RegistryPath(wikiDir string) string {
	return filepath.Join(wikiDir, ".sage", "sync-registry.json")
}

// LoadRegistry reads the registry from disk. Returns an empty registry if
// the file does not exist (not an error).
func LoadRegistry(path string) (*SyncRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SyncRegistry{Projects: map[string]SyncEntry{}}, nil
		}
		return nil, err
	}

	var reg SyncRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}

	if reg.Projects == nil {
		reg.Projects = map[string]SyncEntry{}
	}

	return &reg, nil
}

// SaveRegistry writes the registry to disk (creates parent dirs if needed).
func SaveRegistry(path string, reg *SyncRegistry) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// RecordSync updates or inserts an entry for the given project path.
func RecordSync(path string, projectPath string, slug string, at time.Time, docCount int) error {
	reg, err := LoadRegistry(path)
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}

	reg.Projects[absPath] = SyncEntry{
		AbsPath:    absPath,
		Slug:       slug,
		LastSyncAt: at,
		DocCount:   docCount,
	}

	return SaveRegistry(path, reg)
}

// LastSync returns the last sync time for the given project path.
// Returns zero time and false if the project has never been synced.
func LastSync(path string, projectPath string) (time.Time, bool, error) {
	reg, err := LoadRegistry(path)
	if err != nil {
		return time.Time{}, false, err
	}

	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return time.Time{}, false, err
	}

	entry, ok := reg.Projects[absPath]
	if !ok {
		return time.Time{}, false, nil
	}

	return entry.LastSyncAt, true, nil
}

// ListSynced returns all sync entries, sorted by LastSyncAt descending.
func ListSynced(path string) ([]SyncEntry, error) {
	reg, err := LoadRegistry(path)
	if err != nil {
		return nil, err
	}

	entries := make([]SyncEntry, 0, len(reg.Projects))
	for _, entry := range reg.Projects {
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastSyncAt.After(entries[j].LastSyncAt)
	})

	return entries, nil
}
