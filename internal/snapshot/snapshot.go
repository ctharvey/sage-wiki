package snapshot

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Snapshot creates a timestamped zip archive of the wiki/ and raw/ directories.
// Returns the timestamp string (format: 20060102-150405) on success.
func Snapshot(projectDir string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	snapshotsDir := filepath.Join(projectDir, ".sage", "snapshots")

	// Create snapshots directory if it doesn't exist
	if err := os.MkdirAll(snapshotsDir, 0755); err != nil {
		return "", fmt.Errorf("snapshot: failed to create snapshots directory: %w", err)
	}

	zipPath := filepath.Join(snapshotsDir, timestamp+".zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("snapshot: failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	// Add wiki directory if it exists
	wikiDir := filepath.Join(projectDir, "wiki")
	if _, err := os.Stat(wikiDir); err == nil {
		if err := addDirToZip(zw, wikiDir, "wiki"); err != nil {
			return "", fmt.Errorf("snapshot: failed to add wiki directory: %w", err)
		}
	}

	// Add raw directory if it exists
	rawDir := filepath.Join(projectDir, "raw")
	if _, err := os.Stat(rawDir); err == nil {
		if err := addDirToZip(zw, rawDir, "raw"); err != nil {
			return "", fmt.Errorf("snapshot: failed to add raw directory: %w", err)
		}
	}

	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("snapshot: failed to finalize zip: %w", err)
	}

	return timestamp, nil
}

// Restore extracts a timestamped snapshot zip back into the project directory.
// It removes the existing wiki/ and raw/ directories before extracting.
func Restore(projectDir, timestamp string) error {
	zipPath := filepath.Join(projectDir, ".sage", "snapshots", timestamp+".zip")

	// Check if zip file exists
	if _, err := os.Stat(zipPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("snapshot: snapshot file not found: %s", zipPath)
		}
		return fmt.Errorf("snapshot: failed to stat snapshot file: %w", err)
	}

	// Remove existing wiki and raw directories
	wikiDir := filepath.Join(projectDir, "wiki")
	rawDir := filepath.Join(projectDir, "raw")

	if err := os.RemoveAll(wikiDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("snapshot: failed to remove wiki directory: %w", err)
	}

	if err := os.RemoveAll(rawDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("snapshot: failed to remove raw directory: %w", err)
	}

	// Open and extract zip
	zipFile, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("snapshot: failed to open zip file: %w", err)
	}
	defer zipFile.Close()

	fileInfo, err := zipFile.Stat()
	if err != nil {
		return fmt.Errorf("snapshot: failed to stat zip file: %w", err)
	}

	zr, err := zip.NewReader(zipFile, fileInfo.Size())
	if err != nil {
		return fmt.Errorf("snapshot: failed to create zip reader: %w", err)
	}

	for _, f := range zr.File {
		fpath := filepath.Join(projectDir, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0755); err != nil {
				return fmt.Errorf("snapshot: failed to create directory: %w", err)
			}
		} else {
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
				return fmt.Errorf("snapshot: failed to create parent directory: %w", err)
			}

			// Extract file
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("snapshot: failed to open file in zip: %w", err)
			}

			outFile, err := os.Create(fpath)
			if err != nil {
				rc.Close()
				return fmt.Errorf("snapshot: failed to create output file: %w", err)
			}

			if _, err := io.Copy(outFile, rc); err != nil {
				outFile.Close()
				rc.Close()
				return fmt.Errorf("snapshot: failed to copy file: %w", err)
			}

			outFile.Close()
			rc.Close()
		}
	}

	return nil
}

// ListSnapshots returns a list of available snapshot timestamps,
// sorted in descending order (newest first).
// Returns an empty slice if the snapshots directory doesn't exist.
func ListSnapshots(projectDir string) ([]string, error) {
	snapshotsDir := filepath.Join(projectDir, ".sage", "snapshots")

	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("snapshot: failed to read snapshots directory: %w", err)
	}

	var timestamps []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".zip") {
			// Strip .zip suffix to get timestamp
			timestamp := strings.TrimSuffix(entry.Name(), ".zip")
			timestamps = append(timestamps, timestamp)
		}
	}

	// Sort in descending order (newest first)
	sort.Sort(sort.Reverse(sort.StringSlice(timestamps)))

	return timestamps, nil
}

// addDirToZip recursively adds all files from srcDir to the zip writer,
// preserving relative paths with the given prefix.
func addDirToZip(zw *zip.Writer, srcDir, prefix string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from srcDir
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Build zip path: prefix/relPath
		zipPath := filepath.Join(prefix, relPath)
		// Normalize to forward slashes for zip archive
		zipPath = filepath.ToSlash(zipPath)

		if info.IsDir() {
			// Add directory entry with trailing slash
			_, err := zw.Create(zipPath + "/")
			return err
		}

		// Add file
		f, err := zw.Create(zipPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(f, file)
		return err
	})
}
