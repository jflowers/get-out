package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// T017: validateDriveID tests
// ---------------------------------------------------------------------------

func TestValidateDriveID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{name: "valid 28-char ID", id: "1a2b3c4d5e6f7g8h9i0j1k2l3m4n", wantErr: false},
		{name: "valid 33-char ID with underscores/dashes", id: "1BcD_eF-gHiJkLmNoPqRsTuVwXyZ012", wantErr: false},
		{name: "too short (27 chars)", id: "1a2b3c4d5e6f7g8h9i0j1k2l3m", wantErr: true},
		{name: "contains invalid char (space)", id: "1a2b3c4d5e6f7g8h9i0j1k2l3m4 ", wantErr: true},
		{name: "contains invalid char (!)", id: "1a2b3c4d5e6f7g8h9i0j1k2l3m4!", wantErr: true},
		{name: "empty string", id: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDriveID(tt.id)
			if tt.wantErr && err == nil {
				t.Errorf("validateDriveID(%q) = nil, want error", tt.id)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateDriveID(%q) = %v, want nil", tt.id, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// T017/T018: migrateFiles tests
// ---------------------------------------------------------------------------

func TestMigrateFiles_CopiesMissingFiles(t *testing.T) {
	t.Parallel()
	oldDir := t.TempDir()
	newDir := t.TempDir()

	// Create source files.
	if err := os.WriteFile(filepath.Join(oldDir, "settings.json"), []byte(`{"logLevel":"INFO"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "conversations.json"), []byte(`{"conversations":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := migrateFiles(oldDir, newDir); err != nil {
		t.Fatalf("migrateFiles() error: %v", err)
	}

	// Both files should now exist in newDir.
	for _, name := range []string{"settings.json", "conversations.json"} {
		if _, err := os.Stat(filepath.Join(newDir, name)); err != nil {
			t.Errorf("expected %s to be migrated, but got error: %v", name, err)
		}
	}
}

func TestMigrateFiles_DoesNotOverwriteExisting(t *testing.T) {
	t.Parallel()
	oldDir := t.TempDir()
	newDir := t.TempDir()

	oldContent := []byte("old content")
	newContent := []byte("existing content")

	if err := os.WriteFile(filepath.Join(oldDir, "settings.json"), oldContent, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "settings.json"), newContent, 0644); err != nil {
		t.Fatal(err)
	}

	if err := migrateFiles(oldDir, newDir); err != nil {
		t.Fatalf("migrateFiles() error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(newDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(newContent) {
		t.Errorf("migrateFiles overwrote existing file: got %q, want %q", got, newContent)
	}
}

func TestMigrateFiles_SensitiveFileMode(t *testing.T) {
	t.Parallel()
	oldDir := t.TempDir()
	newDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(oldDir, "credentials.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "token.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := migrateFiles(oldDir, newDir); err != nil {
		t.Fatalf("migrateFiles() error: %v", err)
	}

	for _, name := range []string{"credentials.json", "token.json"} {
		info, err := os.Stat(filepath.Join(newDir, name))
		if err != nil {
			t.Fatalf("expected %s to be migrated: %v", name, err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("%s permissions = %04o, want 0600", name, info.Mode().Perm())
		}
	}
}

// T018: old dir absent — no error.
func TestMigrateFiles_OldDirAbsent(t *testing.T) {
	t.Parallel()
	newDir := t.TempDir()
	oldDir := filepath.Join(t.TempDir(), "nonexistent")

	// oldDir doesn't exist; migrateFiles should not be called, but let's test defensively.
	// The actual runInit checks os.Stat(oldDir) before calling migrateFiles,
	// so migrateFiles won't be called in practice. But if it is, it should handle gracefully.
	// In our implementation managedFiles are iterated; os.IsNotExist skips each file.
	if err := migrateFiles(oldDir, newDir); err != nil {
		t.Fatalf("migrateFiles() with absent oldDir returned error: %v", err)
	}
}

// T018: file exists in both dirs — only new-dir file is kept (no overwrite).
func TestMigrateFiles_PartialNewDir(t *testing.T) {
	t.Parallel()
	oldDir := t.TempDir()
	newDir := t.TempDir()

	// Old dir has settings.json and token.json.
	os.WriteFile(filepath.Join(oldDir, "settings.json"), []byte("old-settings"), 0644) //nolint:errcheck
	os.WriteFile(filepath.Join(oldDir, "token.json"), []byte("old-token"), 0600)       //nolint:errcheck

	// New dir already has settings.json.
	os.WriteFile(filepath.Join(newDir, "settings.json"), []byte("new-settings"), 0644) //nolint:errcheck

	if err := migrateFiles(oldDir, newDir); err != nil {
		t.Fatalf("migrateFiles() error: %v", err)
	}

	// settings.json untouched.
	got, _ := os.ReadFile(filepath.Join(newDir, "settings.json"))
	if string(got) != "new-settings" {
		t.Errorf("settings.json was overwritten: got %q", got)
	}

	// token.json should have been migrated.
	if _, err := os.Stat(filepath.Join(newDir, "token.json")); err != nil {
		t.Errorf("token.json was not migrated: %v", err)
	}
}
