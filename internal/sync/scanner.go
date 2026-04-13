package sync

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SyncDoc is a source document produced by scanning a project.
type SyncDoc struct {
	StagedName   string // filename under raw/sync/<slug>/
	Content      string // markdown content
	OriginalPath string // source path for attribution
}

// ProjectSlug returns a collision-resistant slug for a project path.
// Format: <parent-dir>-<dir-name>. E.g. "/home/user/myorg/api" → "myorg-api".
// Falls back to just the dir name if the path has no parent.
func ProjectSlug(projectPath string) string {
	// Check if the original path has a parent
	// Clean the path first to handle things like "./myproject" or "myproject/"
	cleanPath := filepath.Clean(projectPath)
	originalDir := filepath.Dir(cleanPath)
	originalBase := filepath.Base(cleanPath)

	// If the original path has no parent (it's just a single component),
	// return just the base name
	if originalDir == "." {
		return originalBase
	}

	// Otherwise, get the absolute path and use parent-dir-dir-name format
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		// If Abs fails, use the cleaned path's base
		return originalBase
	}

	dirName := filepath.Base(absPath)
	parentPath := filepath.Dir(absPath)
	parentName := filepath.Base(parentPath)

	// If parent is ".", "/", or empty, just use dir name
	if parentName == "." || parentName == "/" || parentName == "" {
		return dirName
	}

	return parentName + "-" + dirName
}

// ScanProject scans a project directory and returns SyncDocs for all
// relevant files modified after 'since'. If since.IsZero(), all files are included.
// It reads:
//   - README.md, CHANGELOG.md, CLAUDE.md, AGENTS.md (root-level)
//   - docs/**/*.md
//   - .agent/**/*.md
//   - go.mod / package.json / pyproject.toml → synthetic dependencies.md
//   - git log → synthetic git-history.md (uses --since=<date> when !since.IsZero())
//
// Source code files are intentionally excluded.
func ScanProject(projectPath string, since time.Time) ([]SyncDoc, error) {
	var docs []SyncDoc

	// Ensure project path is absolute for consistent handling
	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// 1. Root-level markdown files
	rootFiles := []string{"README.md", "CHANGELOG.md", "CLAUDE.md", "AGENTS.md"}
	for _, filename := range rootFiles {
		filePath := filepath.Join(absProjectPath, filename)
		info, err := os.Stat(filePath)
		if err != nil {
			// File doesn't exist, skip silently
			continue
		}

		// Check mtime if since is specified
		if !since.IsZero() && info.ModTime().Before(since) {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", filename, err)
		}

		docs = append(docs, SyncDoc{
			StagedName:   filename,
			Content:      string(content),
			OriginalPath: filePath,
		})
	}

	// 2. docs/ directory
	docsDir := filepath.Join(absProjectPath, "docs")
	docsDocs, err := scanMarkdownDir(docsDir, "docs", absProjectPath, since)
	if err != nil {
		return nil, err
	}
	docs = append(docs, docsDocs...)

	// 3. .agent/ directory
	agentDir := filepath.Join(absProjectPath, ".agent")
	agentDocs, err := scanMarkdownDir(agentDir, "agent", absProjectPath, since)
	if err != nil {
		return nil, err
	}
	docs = append(docs, agentDocs...)

	// 4. Dependencies file (go.mod, package.json, pyproject.toml)
	depFiles := []string{"go.mod", "package.json", "pyproject.toml"}
	for _, depFile := range depFiles {
		depPath := filepath.Join(absProjectPath, depFile)
		info, err := os.Stat(depPath)
		if err != nil {
			// File doesn't exist, try next
			continue
		}

		// Check mtime if since is specified
		if !since.IsZero() && info.ModTime().Before(since) {
			continue
		}

		content, err := os.ReadFile(depPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", depFile, err)
		}

		depDoc := SyncDoc{
			StagedName:   "dependencies.md",
			Content:      formatDependenciesMarkdown(depFile, string(content)),
			OriginalPath: depPath,
		}
		docs = append(docs, depDoc)
		break // Only use the first one found
	}

	// 5. Git history
	gitDoc, err := scanGitHistory(absProjectPath, since)
	if err != nil {
		// Git errors are silently skipped
		// Continue without git history
	} else if gitDoc != nil {
		docs = append(docs, *gitDoc)
	}

	return docs, nil
}

// scanMarkdownDir walks a directory and returns SyncDocs for all .md files
func scanMarkdownDir(dirPath, prefix, projectPath string, since time.Time) ([]SyncDoc, error) {
	var docs []SyncDoc

	_, err := os.Stat(dirPath)
	if err != nil {
		// Directory doesn't exist, skip silently
		return docs, nil
	}

	err = filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process .md files
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		// Check mtime if since is specified
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("failed to get file info for %s: %w", path, err)
		}

		if !since.IsZero() && info.ModTime().Before(since) {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Calculate staged name: prefix + relative path with / replaced by -
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Convert path separators to -
		stagedName := prefix + "-" + strings.ReplaceAll(relPath, string(filepath.Separator), "-")

		docs = append(docs, SyncDoc{
			StagedName:   stagedName,
			Content:      string(content),
			OriginalPath: path,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return docs, nil
}

// formatDependenciesMarkdown formats dependency file content as markdown
func formatDependenciesMarkdown(filename, content string) string {
	var buf bytes.Buffer
	buf.WriteString("# Dependencies\n\n")
	buf.WriteString(fmt.Sprintf("From `%s`:\n\n", filename))
	buf.WriteString("```\n")
	buf.WriteString(content)
	buf.WriteString("\n```\n")
	return buf.String()
}

// scanGitHistory runs git log and returns a SyncDoc with the history
func scanGitHistory(projectPath string, since time.Time) (*SyncDoc, error) {
	// Build git log command
	args := []string{"log", "--format=%h %s %an %ad", "--date=short"}
	if !since.IsZero() {
		args = append(args, fmt.Sprintf("--since=%s", since.Format("2006-01-02")))
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = projectPath

	output, err := cmd.Output()
	if err != nil {
		// Git failure is silently skipped
		return nil, nil
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return nil, nil
	}

	// Format as markdown
	var buf bytes.Buffer
	buf.WriteString("# Git History\n\n")
	buf.WriteString(fmt.Sprintf("Project: %s\n\n", projectPath))
	buf.WriteString("## Recent Commits\n\n")

	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			buf.WriteString("- ")
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}

	return &SyncDoc{
		StagedName:   "git-history.md",
		Content:      buf.String(),
		OriginalPath: filepath.Join(projectPath, ".git"),
	}, nil
}
