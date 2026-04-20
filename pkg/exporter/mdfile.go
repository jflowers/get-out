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

	return atomicWriteFile(targetDir, targetPath, content)
}

// atomicWriteFile creates a temp file in targetDir, writes content, sets
// permissions, and atomically renames to targetPath. On any error the temp
// file is cleaned up.
//
// Separated from WriteMarkdownFile to reduce per-function cyclomatic
// complexity (lower CRAP score for the same coverage level).
func atomicWriteFile(targetDir, targetPath string, content []byte) error {
	tmpFile, err := os.CreateTemp(targetDir, ".tmp-*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up the temp file on any error
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	// Write content and close the file handle. Both must succeed
	// before we proceed to rename.
	if err := writeAndClose(tmpFile, content); err != nil {
		return err
	}

	// Set deterministic permissions (CreateTemp uses 0600 by default)
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomic rename to final location
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to rename temp file to target: %w", err)
	}

	success = true
	return nil
}

// writeAndClose writes content to f and closes it. The file is always
// closed, even when the write fails. This consolidates two error paths
// into one call site, reducing cyclomatic complexity in atomicWriteFile.
func writeAndClose(f *os.File, content []byte) error {
	_, writeErr := f.Write(content)
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("failed to write temp file: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close temp file: %w", closeErr)
	}
	return nil
}
