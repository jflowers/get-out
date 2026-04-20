package exporter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandAndValidatePath expands ~ to the current user's home directory and
// validates that the resulting path is not dangerously shallow.
// Returns the expanded absolute path or an error.
func ExpandAndValidatePath(path string) (string, error) {
	// Handle tilde expansion
	if strings.HasPrefix(path, "~") {
		if len(path) == 1 || path[1] == '/' {
			// ~/some/path or just ~
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to resolve home directory: %w", err)
			}
			path = filepath.Join(home, path[1:])
		} else {
			// ~otheruser/path — not supported
			return "", fmt.Errorf("local export path must use ~/ for current user home, not ~username/")
		}
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Clean the path
	absPath = filepath.Clean(absPath)

	// Reject paths that are too shallow (fewer than 2 components).
	// Split by separator and count non-empty parts. On Unix, "/" splits to
	// ["", ""], giving 0 real parts; "/foo" gives 1 real part.
	parts := strings.Split(absPath, string(filepath.Separator))
	realParts := 0
	for _, p := range parts {
		if p != "" {
			realParts++
		}
	}
	if realParts < 2 {
		return "", fmt.Errorf("local export path is too shallow")
	}

	return absPath, nil
}

// WriteMarkdownFile writes content to {dir}/{typeName}/{date}.md atomically.
// typeName is the sanitized directory name (e.g., from SanitizeDirectoryName).
// Returns nil if the target file already exists (skip).
// Returns nil if content is written successfully.
// Uses atomic write: .tmp- prefixed temp file + os.Rename.
func WriteMarkdownFile(dir string, typeName string, date string, content []byte) error {
	targetDir := filepath.Join(dir, typeName)
	targetPath := filepath.Join(targetDir, date+".md")

	// Check if the target file already exists — skip if so
	if _, err := os.Stat(targetPath); err == nil {
		return nil
	}

	// Create the directory hierarchy
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	// Create temp file in the target directory
	tmpFile, err := os.CreateTemp(targetDir, ".tmp-*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// From here on, clean up the temp file on any error
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	// Write content
	if _, err := tmpFile.Write(content); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Close before chmod/rename
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set file permissions
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to rename temp file to target: %w", err)
	}

	success = true
	return nil
}
