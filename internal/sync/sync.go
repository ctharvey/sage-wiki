package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xoai/sage-wiki/internal/compiler"
)

// SyncConfig configures a sync run.
type SyncConfig struct {
	WikiDir     string // sage-wiki project directory (has raw/, wiki/, .sage/)
	ProjectPath string // external project to scan
	DryRun      bool   // if true, stage files but don't compile
	Force       bool   // if true, ignore last-sync time and include all files
}

// SyncResult summarizes what happened.
type SyncResult struct {
	Slug       string                   // collision-resistant slug for the project
	DocsStaged int                      // number of docs written to raw/sync/<slug>/
	Compile    *compiler.CompileResult // nil if DryRun
}

// Run scans an external project and compiles it into the wiki.
// Steps:
// 1. Compute slug via ProjectSlug(cfg.ProjectPath)
// 2. Look up last sync time via LastSync(registryPath, cfg.ProjectPath)
//    - if Force or no prior sync, since = time.Time{} (zero, meaning all files)
//    - otherwise since = last sync time
// 3. Call ScanProject(cfg.ProjectPath, since) to get []SyncDoc
// 4. For each SyncDoc, write to <cfg.WikiDir>/raw/sync/<slug>/<doc.StagedName>
//    - Create parent dirs as needed (os.MkdirAll)
//    - Write content as UTF-8
// 5. If DryRun, return SyncResult{Slug: slug, DocsStaged: len(docs), Compile: nil}
// 6. Call compiler.Compile(cfg.WikiDir, compiler.CompileOpts{}) and store result
// 7. Call RecordSync(registryPath, cfg.ProjectPath, slug, time.Now(), len(docs)) to update registry
// 8. Return SyncResult{Slug: slug, DocsStaged: len(docs), Compile: &compileResult}
func Run(cfg SyncConfig) (*SyncResult, error) {
	// Step 1: Compute slug
	slug := ProjectSlug(cfg.ProjectPath)

	// Helper for registry path
	registryPath := RegistryPath(cfg.WikiDir)

	// Step 2: Look up last sync time
	var since time.Time
	if !cfg.Force {
		lastSync, found, err := LastSync(registryPath, cfg.ProjectPath)
		if err != nil {
			return nil, fmt.Errorf("sync: lookup last sync: %w", err)
		}
		if found {
			since = lastSync
		}
		// If not found, since remains zero time (all files)
	}
	// If Force is true, since remains zero time (all files)

	// Step 3: Scan project
	docs, err := ScanProject(cfg.ProjectPath, since)
	if err != nil {
		return nil, fmt.Errorf("sync: scan project: %w", err)
	}

	// Step 4: Write docs to staging directory
	stagingDir := filepath.Join(cfg.WikiDir, "raw", "sync", slug)
	if len(docs) > 0 {
		if err := os.MkdirAll(stagingDir, 0755); err != nil {
			return nil, fmt.Errorf("sync: create staging dir: %w", err)
		}
		for _, doc := range docs {
			destPath := filepath.Join(stagingDir, doc.StagedName)
			if err := os.WriteFile(destPath, []byte(doc.Content), 0644); err != nil {
				return nil, fmt.Errorf("sync: write %s: %w", doc.StagedName, err)
			}
		}
	}

	// Step 5: Handle DryRun
	if cfg.DryRun {
		return &SyncResult{
			Slug:       slug,
			DocsStaged: len(docs),
			Compile:    nil,
		}, nil
	}

	// If no docs and not DryRun, still record sync and return
	if len(docs) == 0 {
		if err := RecordSync(registryPath, cfg.ProjectPath, slug, time.Now(), 0); err != nil {
			return nil, fmt.Errorf("sync: record sync: %w", err)
		}
		return &SyncResult{
			Slug:       slug,
			DocsStaged: 0,
			Compile:    nil,
		}, nil
	}

	// Step 6: Compile
	compileResult, err := compiler.Compile(cfg.WikiDir, compiler.CompileOpts{})
	if err != nil {
		return nil, fmt.Errorf("sync: compile: %w", err)
	}

	// Step 7: Record sync
	if err := RecordSync(registryPath, cfg.ProjectPath, slug, time.Now(), len(docs)); err != nil {
		return nil, fmt.Errorf("sync: record sync: %w", err)
	}

	// Step 8: Return result
	return &SyncResult{
		Slug:       slug,
		DocsStaged: len(docs),
		Compile:    compileResult,
	}, nil
}
