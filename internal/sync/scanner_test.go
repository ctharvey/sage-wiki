package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProjectSlug(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "full path",
			path:     "/home/user/myorg/api",
			expected: "myorg-api",
		},
		{
			name:     "single component path",
			path:     "myproject",
			expected: "myproject",
		},
		{
			name:     "relative path",
			path:     "projects/myapp",
			expected: "projects-myapp",
		},
		{
			name:     "path with trailing slash",
			path:     "/home/user/myorg/api/",
			expected: "myorg-api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProjectSlug(tt.path)
			if result != tt.expected {
				t.Errorf("ProjectSlug(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestScanProjectBasic(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "scanproject-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write README.md
	readmeContent := "# Test Project\n\nThis is a test project.\n"
	readmePath := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		t.Fatalf("Failed to write README.md: %v", err)
	}

	// Create docs directory and write intro.md
	docsDir := filepath.Join(tempDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("Failed to create docs dir: %v", err)
	}
	introContent := "# Introduction\n\nWelcome to the docs.\n"
	introPath := filepath.Join(docsDir, "intro.md")
	if err := os.WriteFile(introPath, []byte(introContent), 0644); err != nil {
		t.Fatalf("Failed to write intro.md: %v", err)
	}

	// Scan with zero time (include all files)
	docs, err := ScanProject(tempDir, time.Time{})
	if err != nil {
		t.Fatalf("ScanProject failed: %v", err)
	}

	// Build a map of StagedName to SyncDoc for easy lookup
	docMap := make(map[string]SyncDoc)
	for _, doc := range docs {
		docMap[doc.StagedName] = doc
	}

	// Verify README.md exists
	if readmeDoc, ok := docMap["README.md"]; !ok {
		t.Errorf("README.md not found in results")
	} else {
		if readmeDoc.Content != readmeContent {
			t.Errorf("README.md content mismatch: got %q, want %q", readmeDoc.Content, readmeContent)
		}
		if !strings.HasSuffix(readmeDoc.OriginalPath, "README.md") {
			t.Errorf("README.md OriginalPath should end with 'README.md', got %q", readmeDoc.OriginalPath)
		}
	}

	// Verify docs-intro.md exists
	if introDoc, ok := docMap["docs-intro.md"]; !ok {
		t.Errorf("docs-intro.md not found in results")
	} else {
		if introDoc.Content != introContent {
			t.Errorf("docs-intro.md content mismatch: got %q, want %q", introDoc.Content, introContent)
		}
		if !strings.HasSuffix(introDoc.OriginalPath, "intro.md") {
			t.Errorf("docs-intro.md OriginalPath should end with 'intro.md', got %q", introDoc.OriginalPath)
		}
	}
}

func TestScanProjectWithSince(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "scanproject-since-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write an old file
	oldContent := "# Old File\n"
	oldPath := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(oldPath, []byte(oldContent), 0644); err != nil {
		t.Fatalf("Failed to write old file: %v", err)
	}

	// Wait a bit and record the time
	time.Sleep(100 * time.Millisecond)
	cutoffTime := time.Now()
	time.Sleep(100 * time.Millisecond)

	// Write a new file
	newDocsDir := filepath.Join(tempDir, "docs")
	if err := os.MkdirAll(newDocsDir, 0755); err != nil {
		t.Fatalf("Failed to create docs dir: %v", err)
	}
	newContent := "# New File\n"
	newPath := filepath.Join(newDocsDir, "new.md")
	if err := os.WriteFile(newPath, []byte(newContent), 0644); err != nil {
		t.Fatalf("Failed to write new file: %v", err)
	}

	// Scan with cutoff time
	docs, err := ScanProject(tempDir, cutoffTime)
	if err != nil {
		t.Fatalf("ScanProject failed: %v", err)
	}

	// Should only have the new file
	if len(docs) != 1 {
		t.Errorf("Expected 1 doc, got %d: %v", len(docs), docs)
	}

	if len(docs) > 0 && docs[0].StagedName != "docs-new.md" {
		t.Errorf("Expected docs-new.md, got %s", docs[0].StagedName)
	}
}

func TestScanProjectDependencies(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "scanproject-deps-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write go.mod
	goModContent := "module example.com/test\n\ngo 1.21\n"
	goModPath := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Scan
	docs, err := ScanProject(tempDir, time.Time{})
	if err != nil {
		t.Fatalf("ScanProject failed: %v", err)
	}

	// Find dependencies.md
	var found bool
	for _, doc := range docs {
		if doc.StagedName == "dependencies.md" {
			found = true
			if !strings.Contains(doc.Content, "go.mod") {
				t.Errorf("dependencies.md should mention go.mod")
			}
			if !strings.Contains(doc.Content, goModContent) {
				t.Errorf("dependencies.md should contain go.mod content")
			}
			break
		}
	}

	if !found {
		t.Errorf("dependencies.md not found in results")
	}
}

func TestScanProjectAgentDir(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "scanproject-agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create .agent directory and write a file
	agentDir := filepath.Join(tempDir, ".agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("Failed to create .agent dir: %v", err)
	}
	agentContent := "# Agent Instructions\n"
	agentPath := filepath.Join(agentDir, "instructions.md")
	if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to write agent file: %v", err)
	}

	// Scan
	docs, err := ScanProject(tempDir, time.Time{})
	if err != nil {
		t.Fatalf("ScanProject failed: %v", err)
	}

	// Find agent-instructions.md
	var found bool
	for _, doc := range docs {
		if doc.StagedName == "agent-instructions.md" {
			found = true
			if doc.Content != agentContent {
				t.Errorf("agent-instructions.md content mismatch")
			}
			break
		}
	}

	if !found {
		t.Errorf("agent-instructions.md not found in results")
	}
}
