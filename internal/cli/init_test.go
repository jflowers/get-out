package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// scaffoldConfigDir tests
// ---------------------------------------------------------------------------

func TestScaffoldConfigDir_FreshDirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "newdir")

	created, err := scaffoldConfigDir(dir)
	if err != nil {
		t.Fatalf("scaffoldConfigDir() error: %v", err)
	}

	// Directory must exist.
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("config dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("config path is not a directory")
	}

	// Both starter files should be reported as created.
	if len(created) != 2 {
		t.Fatalf("created = %v, want 2 entries", created)
	}
	wantFiles := map[string]bool{"conversations.json": false, "settings.json": false}
	for _, name := range created {
		if _, ok := wantFiles[name]; !ok {
			t.Errorf("unexpected created file: %s", name)
		}
		wantFiles[name] = true
	}
	for name, found := range wantFiles {
		if !found {
			t.Errorf("expected %s in created list", name)
		}
	}

	// Both files must exist on disk.
	for _, name := range []string{"conversations.json", "settings.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not found on disk: %v", name, err)
		}
	}
}

func TestScaffoldConfigDir_ExistingDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir() // already exists

	created, err := scaffoldConfigDir(dir)
	if err != nil {
		t.Fatalf("scaffoldConfigDir() error: %v", err)
	}

	// Both starter files should still be created since the dir is empty.
	if len(created) != 2 {
		t.Fatalf("created = %v, want 2 entries", created)
	}
	for _, name := range []string{"conversations.json", "settings.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not found on disk: %v", name, err)
		}
	}
}

func TestScaffoldConfigDir_ExistingFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Pre-create both starter files with custom content.
	if err := os.WriteFile(filepath.Join(dir, "conversations.json"), []byte(`{"custom":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"existing":true}`), 0600); err != nil {
		t.Fatal(err)
	}

	created, err := scaffoldConfigDir(dir)
	if err != nil {
		t.Fatalf("scaffoldConfigDir() error: %v", err)
	}

	// Nothing should be created since both files already exist.
	if len(created) != 0 {
		t.Fatalf("created = %v, want empty (files already exist)", created)
	}

	// Original content must be preserved.
	got, err := os.ReadFile(filepath.Join(dir, "conversations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"custom":true}` {
		t.Errorf("conversations.json was overwritten: got %q", got)
	}

	got, err = os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"existing":true}` {
		t.Errorf("settings.json was overwritten: got %q", got)
	}
}

func TestScaffoldConfigDir_NotADirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "blockerfile")

	// Create a regular file where the config dir would go.
	if err := os.WriteFile(filePath, []byte("I am a file"), 0644); err != nil {
		t.Fatal(err)
	}

	created, err := scaffoldConfigDir(filePath)
	if err == nil {
		t.Fatal("scaffoldConfigDir() = nil error, want error for non-directory path")
	}
	if created != nil {
		t.Error("expected nil created list on error")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error = %q, want it to mention 'not a directory'", err)
	}
}

func TestScaffoldConfigDir_StarterConversationsContent(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "fresh")

	_, err := scaffoldConfigDir(dir)
	if err != nil {
		t.Fatalf("scaffoldConfigDir() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "conversations.json"))
	if err != nil {
		t.Fatalf("reading conversations.json: %v", err)
	}

	// Must be valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("conversations.json is not valid JSON: %v\ncontent: %s", err, data)
	}

	// Must have a "conversations" key with an empty array.
	convs, ok := parsed["conversations"]
	if !ok {
		t.Fatal("conversations.json missing 'conversations' key")
	}
	arr, ok := convs.([]interface{})
	if !ok {
		t.Fatalf("conversations field is %T, want []interface{}", convs)
	}
	if len(arr) != 0 {
		t.Errorf("conversations array has %d elements, want 0", len(arr))
	}
}

func TestScaffoldConfigDir_SettingsPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission checks not reliable on Windows")
	}
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "permcheck")

	_, err := scaffoldConfigDir(dir)
	if err != nil {
		t.Fatalf("scaffoldConfigDir() error: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("stat settings.json: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("settings.json permissions = %04o, want 0600", perm)
	}
}

// ---------------------------------------------------------------------------
// formatDoctorSummary tests
// ---------------------------------------------------------------------------

func TestFormatDoctorSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		pass         int
		warn         int
		fail         int
		wantContains []string
	}{
		{
			name:         "all passing",
			pass:         10,
			warn:         0,
			fail:         0,
			wantContains: []string{"10 passed", "0 warnings", "0 failures"},
		},
		{
			name:         "with warnings",
			pass:         7,
			warn:         3,
			fail:         0,
			wantContains: []string{"7 passed", "3 warnings", "0 failures"},
		},
		{
			name:         "with failures",
			pass:         5,
			warn:         2,
			fail:         3,
			wantContains: []string{"5 passed", "2 warnings", "3 failures"},
		},
		{
			name:         "all zeros",
			pass:         0,
			warn:         0,
			fail:         0,
			wantContains: []string{"0 passed", "0 warnings", "0 failures"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			formatDoctorSummary(&buf, tt.pass, tt.warn, tt.fail)
			got := buf.String()

			for _, sub := range tt.wantContains {
				if !strings.Contains(got, sub) {
					t.Errorf("formatDoctorSummary(%d, %d, %d) output missing %q\ngot: %s",
						tt.pass, tt.warn, tt.fail, sub, got)
				}
			}

			// Should contain the separator lines.
			if strings.Count(got, "─────────────────────────────────────────") < 2 {
				t.Errorf("expected at least 2 separator lines in output\ngot: %s", got)
			}
		})
	}
}
