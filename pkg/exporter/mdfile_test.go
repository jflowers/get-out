package exporter

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExpandAndValidatePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	t.Run("tilde expands to home dir", func(t *testing.T) {
		result, err := ExpandAndValidatePath("~/some/path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(home, "some", "path")
		if result != want {
			t.Errorf("got %q, want %q", result, want)
		}
	})

	t.Run("tilde-only expands to home dir", func(t *testing.T) {
		// ~ alone should expand but may fail the "too shallow" check
		// depending on the home directory depth. On most systems,
		// /Users/name or /home/name has 2+ components, so it passes.
		result, err := ExpandAndValidatePath("~")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != home {
			t.Errorf("got %q, want %q", result, home)
		}
	})

	t.Run("tilde-username returns error", func(t *testing.T) {
		_, err := ExpandAndValidatePath("~otheruser/path")
		if err == nil {
			t.Fatal("expected error for ~otheruser path")
		}
		if !strings.Contains(err.Error(), "~username") {
			t.Errorf("error should mention ~username, got: %v", err)
		}
	})

	t.Run("relative path resolves to absolute", func(t *testing.T) {
		result, err := ExpandAndValidatePath("relative/path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !filepath.IsAbs(result) {
			t.Errorf("expected absolute path, got %q", result)
		}
		if !strings.HasSuffix(result, filepath.Join("relative", "path")) {
			t.Errorf("expected path to end with relative/path, got %q", result)
		}
	})

	t.Run("root path returns error (too shallow)", func(t *testing.T) {
		_, err := ExpandAndValidatePath("/")
		if err == nil {
			t.Fatal("expected error for root path")
		}
		if !strings.Contains(err.Error(), "too shallow") {
			t.Errorf("error should mention too shallow, got: %v", err)
		}
	})

	t.Run("single-component path returns error (too shallow)", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("path semantics differ on Windows")
		}
		_, err := ExpandAndValidatePath("/tmp")
		if err == nil {
			t.Fatal("expected error for single-component path")
		}
		if !strings.Contains(err.Error(), "too shallow") {
			t.Errorf("error should mention too shallow, got: %v", err)
		}
	})

	t.Run("normal absolute path passes through", func(t *testing.T) {
		input := "/usr/local/share"
		result, err := ExpandAndValidatePath(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != input {
			t.Errorf("got %q, want %q", result, input)
		}
	})

	t.Run("absolute path with two components passes", func(t *testing.T) {
		input := "/tmp/exports"
		result, err := ExpandAndValidatePath(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != input {
			t.Errorf("got %q, want %q", result, input)
		}
	})
}

func TestWriteMarkdownFile(t *testing.T) {
	t.Run("creates directory and writes file", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte("# Test\nHello, world!")

		err := WriteMarkdownFile(dir, "channel-general", "2026-04-20", content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify file exists at expected path
		targetPath := filepath.Join(dir, "channel-general", "2026-04-20.md")
		data, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}

		// Verify content matches
		if string(data) != string(content) {
			t.Errorf("content mismatch: got %q, want %q", string(data), string(content))
		}
	})

	t.Run("file has 0644 permissions", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte("# Permissions Test")

		err := WriteMarkdownFile(dir, "perms-test", "2026-01-01", content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		targetPath := filepath.Join(dir, "perms-test", "2026-01-01.md")
		info, err := os.Stat(targetPath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}

		perm := info.Mode().Perm()
		if perm != 0644 {
			t.Errorf("expected permissions 0644, got %04o", perm)
		}
	})

	t.Run("existing file is not overwritten", func(t *testing.T) {
		dir := t.TempDir()
		typeName := "no-overwrite"
		date := "2026-03-15"
		originalContent := []byte("original content")
		newContent := []byte("new content that should not appear")

		// Write the first time
		err := WriteMarkdownFile(dir, typeName, date, originalContent)
		if err != nil {
			t.Fatalf("first write failed: %v", err)
		}

		// Write again — should skip silently
		err = WriteMarkdownFile(dir, typeName, date, newContent)
		if err != nil {
			t.Fatalf("second write returned error: %v", err)
		}

		// Verify original content is preserved
		targetPath := filepath.Join(dir, typeName, date+".md")
		data, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(data) != string(originalContent) {
			t.Errorf("file was overwritten: got %q, want %q", string(data), string(originalContent))
		}
	})

	t.Run("parent directory is created if missing", func(t *testing.T) {
		dir := t.TempDir()
		typeName := "deeply/nested/channel"
		date := "2026-06-01"
		content := []byte("# Nested")

		err := WriteMarkdownFile(dir, typeName, date, content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		targetPath := filepath.Join(dir, typeName, date+".md")
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			t.Errorf("expected file at %s but it does not exist", targetPath)
		}
	})

	t.Run("empty content writes empty file", func(t *testing.T) {
		dir := t.TempDir()

		err := WriteMarkdownFile(dir, "empty", "2026-01-01", []byte{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		targetPath := filepath.Join(dir, "empty", "2026-01-01.md")
		data, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if len(data) != 0 {
			t.Errorf("expected empty file, got %d bytes", len(data))
		}
	})
}

func TestWriteMarkdownFile_AtomicCleanup(t *testing.T) {
	t.Run("no orphaned temp files after successful write", func(t *testing.T) {
		dir := t.TempDir()
		typeName := "atomic-test"
		content := []byte("# Atomic Write Test\nContent here.")

		err := WriteMarkdownFile(dir, typeName, "2026-07-04", content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check the target directory for any .tmp- files
		targetDir := filepath.Join(dir, typeName)
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			t.Fatalf("failed to read directory: %v", err)
		}

		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".tmp-") {
				t.Errorf("found orphaned temp file: %s", entry.Name())
			}
		}

		// Should have exactly one file: the target
		mdFiles := 0
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".md") && !strings.HasPrefix(entry.Name(), ".tmp-") {
				mdFiles++
			}
		}
		if mdFiles != 1 {
			t.Errorf("expected 1 .md file, found %d", mdFiles)
		}
	})

	t.Run("no orphaned temp files after multiple writes", func(t *testing.T) {
		dir := t.TempDir()
		typeName := "multi-write"

		// Write several files
		dates := []string{"2026-01-01", "2026-01-02", "2026-01-03"}
		for _, date := range dates {
			content := []byte("# " + date)
			err := WriteMarkdownFile(dir, typeName, date, content)
			if err != nil {
				t.Fatalf("failed to write %s: %v", date, err)
			}
		}

		// Verify no temp files remain
		targetDir := filepath.Join(dir, typeName)
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			t.Fatalf("failed to read directory: %v", err)
		}

		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".tmp-") {
				t.Errorf("found orphaned temp file: %s", entry.Name())
			}
		}

		// Should have exactly 3 files
		if len(entries) != len(dates) {
			t.Errorf("expected %d files, found %d", len(dates), len(entries))
		}
	})
}
