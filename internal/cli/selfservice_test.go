package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jflowers/get-out/pkg/secrets"
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

// ---------------------------------------------------------------------------
// T036: oauthToken.Valid() tests
// ---------------------------------------------------------------------------

func TestOAuthToken_Valid(t *testing.T) {
	t.Parallel()
	now := time.Now()

	tests := []struct {
		name  string
		token oauthToken
		want  bool
	}{
		{
			name:  "valid: future expiry + non-empty access token",
			token: oauthToken{AccessToken: "tok", Expiry: now.Add(time.Hour)},
			want:  true,
		},
		{
			name:  "invalid: past expiry",
			token: oauthToken{AccessToken: "tok", Expiry: now.Add(-time.Hour)},
			want:  false,
		},
		{
			name:  "invalid: expiry within 10-second buffer",
			token: oauthToken{AccessToken: "tok", Expiry: now.Add(5 * time.Second)},
			want:  false,
		},
		{
			name:  "invalid: empty access token even with future expiry",
			token: oauthToken{AccessToken: "", Expiry: now.Add(time.Hour)},
			want:  false,
		},
		{
			name:  "invalid: zero expiry",
			token: oauthToken{AccessToken: "tok"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.token.Valid()
			if got != tt.want {
				t.Errorf("oauthToken.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// T036: loadTokenForDoctor tests
// ---------------------------------------------------------------------------

func TestLoadTokenForDoctor(t *testing.T) {
	t.Parallel()

	t.Run("valid JSON token file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
		tok := oauthToken{AccessToken: "abc", RefreshToken: "ref", Expiry: expiry}
		data, _ := json.Marshal(tok)
		path := filepath.Join(dir, "token.json")
		os.WriteFile(path, data, 0600) //nolint:errcheck

		got, err := loadTokenForDoctor(path)
		if err != nil {
			t.Fatalf("loadTokenForDoctor() error: %v", err)
		}
		if got.AccessToken != "abc" {
			t.Errorf("AccessToken = %q, want %q", got.AccessToken, "abc")
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		t.Parallel()
		_, err := loadTokenForDoctor(filepath.Join(t.TempDir(), "nonexistent.json"))
		if err == nil {
			t.Error("loadTokenForDoctor() = nil, want error for missing file")
		}
	})

	t.Run("corrupt JSON returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "token.json")
		os.WriteFile(path, []byte("not-json{{{"), 0600) //nolint:errcheck
		_, err := loadTokenForDoctor(path)
		if err == nil {
			t.Error("loadTokenForDoctor() = nil, want error for corrupt JSON")
		}
	})
}

// ---------------------------------------------------------------------------
// T036: checkConfigDir tests
// ---------------------------------------------------------------------------

func TestCheckConfigDir(t *testing.T) {
	t.Parallel()

	t.Run("absent dir increments failCount", func(t *testing.T) {
		t.Parallel()
		var p, w, f int
		checkConfigDir(filepath.Join(t.TempDir(), "nonexistent"), secrets.BackendFile, &p, &w, &f)
		if f != 1 || p != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want pass=0 fail=1", p, w, f)
		}
	})

	t.Run("path is a file increments failCount", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		filePath := filepath.Join(dir, "notadir")
		os.WriteFile(filePath, []byte("x"), 0644) //nolint:errcheck
		var p, w, f int
		checkConfigDir(filePath, secrets.BackendFile, &p, &w, &f)
		if f != 1 || p != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want pass=0 fail=1", p, w, f)
		}
	})

	t.Run("valid dir with mode 0700 increments passCount only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		os.Chmod(dir, 0700) //nolint:errcheck
		var p, w, f int
		checkConfigDir(dir, secrets.BackendFile, &p, &w, &f)
		if p != 1 || w != 0 || f != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want pass=1 warn=0 fail=0", p, w, f)
		}
	})

	t.Run("valid dir with broad permissions increments pass and warn", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		os.Chmod(dir, 0755) //nolint:errcheck
		var p, w, f int
		checkConfigDir(dir, secrets.BackendFile, &p, &w, &f)
		if p != 1 || w != 1 || f != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want pass=1 warn=1 fail=0", p, w, f)
		}
	})
}

// ---------------------------------------------------------------------------
// T036: checkFile tests
// ---------------------------------------------------------------------------

func TestCheckFile(t *testing.T) {
	t.Parallel()

	t.Run("absent file with mustExist=true increments failCount", func(t *testing.T) {
		t.Parallel()
		var p, w, f int
		present := checkFile("missing.json", filepath.Join(t.TempDir(), "missing.json"), true, "fix msg", &p, &w, &f)
		if present || f != 1 || p != 0 {
			t.Errorf("got present=%v pass=%d warn=%d fail=%d, want present=false fail=1", present, p, w, f)
		}
	})

	t.Run("absent file with mustExist=false increments warnCount", func(t *testing.T) {
		t.Parallel()
		var p, w, f int
		present := checkFile("optional.json", filepath.Join(t.TempDir(), "optional.json"), false, "fix msg", &p, &w, &f)
		if present || w != 1 || f != 0 {
			t.Errorf("got present=%v pass=%d warn=%d fail=%d, want present=false warn=1", present, p, w, f)
		}
	})

	t.Run("present non-sensitive file increments passCount", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		os.WriteFile(path, []byte("{}"), 0644) //nolint:errcheck
		var p, w, f int
		present := checkFile("settings.json", path, true, "", &p, &w, &f)
		if !present || p != 1 || w != 0 || f != 0 {
			t.Errorf("got present=%v pass=%d warn=%d fail=%d, want present=true pass=1", present, p, w, f)
		}
	})

	t.Run("present sensitive file with broad perms increments pass and warn", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "token.json")
		os.WriteFile(path, []byte("{}"), 0644) //nolint:errcheck
		var p, w, f int
		present := checkFile("token.json", path, true, "", &p, &w, &f)
		if !present || p != 1 || w != 1 || f != 0 {
			t.Errorf("got present=%v pass=%d warn=%d fail=%d, want present=true pass=1 warn=1", present, p, w, f)
		}
	})

	t.Run("present sensitive file with 0600 perms increments passCount only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "credentials.json")
		os.WriteFile(path, []byte("{}"), 0600) //nolint:errcheck
		var p, w, f int
		present := checkFile("credentials.json", path, true, "", &p, &w, &f)
		if !present || p != 1 || w != 0 || f != 0 {
			t.Errorf("got present=%v pass=%d warn=%d fail=%d, want present=true pass=1 warn=0", present, p, w, f)
		}
	})
}

// ---------------------------------------------------------------------------
// T036: checkTokenValidity tests
// ---------------------------------------------------------------------------

func TestCheckTokenValidity(t *testing.T) {
	t.Parallel()

	// writeTokenToStore writes a token as JSON into a FileStore for the given dir.
	writeTokenToStore := func(dir string, tok oauthToken) secrets.SecretStore {
		data, _ := json.Marshal(tok)
		store := &secrets.FileStore{ConfigDir: dir}
		store.Set(secrets.KeyOAuthToken, string(data)) //nolint:errcheck
		return store
	}

	t.Run("valid token: passCount++ returns true", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		store := writeTokenToStore(dir, oauthToken{
			AccessToken: "tok",
			Expiry:      time.Now().Add(time.Hour),
		})
		var p, w, f int
		ok := checkTokenValidity(dir, store, &p, &w, &f)
		if !ok || p != 1 || w != 0 || f != 0 {
			t.Errorf("got ok=%v pass=%d warn=%d fail=%d, want ok=true pass=1", ok, p, w, f)
		}
	})

	t.Run("expired token with refresh: warnCount++ returns true", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		store := writeTokenToStore(dir, oauthToken{
			AccessToken:  "tok",
			RefreshToken: "ref",
			Expiry:       time.Now().Add(-time.Hour),
		})
		var p, w, f int
		ok := checkTokenValidity(dir, store, &p, &w, &f)
		if !ok || w != 1 || p != 0 || f != 0 {
			t.Errorf("got ok=%v pass=%d warn=%d fail=%d, want ok=true warn=1", ok, p, w, f)
		}
	})

	t.Run("expired token without refresh: failCount++ returns false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		store := writeTokenToStore(dir, oauthToken{
			AccessToken: "tok",
			Expiry:      time.Now().Add(-time.Hour),
		})
		var p, w, f int
		ok := checkTokenValidity(dir, store, &p, &w, &f)
		if ok || f != 1 || p != 0 {
			t.Errorf("got ok=%v pass=%d warn=%d fail=%d, want ok=false fail=1", ok, p, w, f)
		}
	})

	t.Run("corrupt token: failCount++ returns false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Write corrupt JSON directly to token.json so FileStore.Get succeeds but parse fails
		os.WriteFile(filepath.Join(dir, "token.json"), []byte("bad-json"), 0600) //nolint:errcheck
		store := &secrets.FileStore{ConfigDir: dir}
		var p, w, f int
		ok := checkTokenValidity(dir, store, &p, &w, &f)
		if ok || f != 1 {
			t.Errorf("got ok=%v pass=%d warn=%d fail=%d, want ok=false fail=1", ok, p, w, f)
		}
	})
}

// ---------------------------------------------------------------------------
// T036: checkConversations tests
// ---------------------------------------------------------------------------

func TestCheckConversations(t *testing.T) {
	t.Parallel()

	t.Run("absent file: failCount++", func(t *testing.T) {
		t.Parallel()
		var p, w, f int
		checkConversations(t.TempDir(), &p, &w, &f)
		if f != 1 || p != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want fail=1", p, w, f)
		}
	})

	t.Run("corrupt JSON: failCount++", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "conversations.json"), []byte("bad{json"), 0644) //nolint:errcheck
		var p, w, f int
		checkConversations(dir, &p, &w, &f)
		if f != 1 || p != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want fail=1", p, w, f)
		}
	})

	t.Run("empty conversations array: warnCount++", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "conversations.json"), []byte(`{"conversations":[]}`), 0644) //nolint:errcheck
		var p, w, f int
		checkConversations(dir, &p, &w, &f)
		if w != 1 || f != 0 || p != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want warn=1", p, w, f)
		}
	})

	t.Run("valid conversations: passCount++", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		content := `{"conversations":[{"id":"C123ABCDEFG","name":"general","type":"channel","mode":"browser","export":true}]}`
		os.WriteFile(filepath.Join(dir, "conversations.json"), []byte(content), 0644) //nolint:errcheck
		var p, w, f int
		checkConversations(dir, &p, &w, &f)
		if p != 1 || w != 0 || f != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want pass=1", p, w, f)
		}
	})
}

// ---------------------------------------------------------------------------
// T036: checkPeople tests
// ---------------------------------------------------------------------------

func TestCheckPeople(t *testing.T) {
	t.Parallel()

	t.Run("absent file: warnCount++", func(t *testing.T) {
		t.Parallel()
		var p, w, f int
		checkPeople(t.TempDir(), &p, &w, &f)
		if w != 1 || f != 0 || p != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want warn=1", p, w, f)
		}
	})

	t.Run("present file: passCount++", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "people.json"), []byte(`{}`), 0644) //nolint:errcheck
		var p, w, f int
		checkPeople(dir, &p, &w, &f)
		if p != 1 || w != 0 || f != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want pass=1", p, w, f)
		}
	})
}

// ---------------------------------------------------------------------------
// T036: checkExportIndex tests
// ---------------------------------------------------------------------------

func TestCheckExportIndex(t *testing.T) {
	t.Parallel()

	t.Run("absent file: warnCount++", func(t *testing.T) {
		t.Parallel()
		var p, w, f int
		checkExportIndex(t.TempDir(), &p, &w, &f)
		if w != 1 || f != 0 || p != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want warn=1", p, w, f)
		}
	})

	t.Run("corrupt JSON: failCount++", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "export-index.json"), []byte("bad{json"), 0644) //nolint:errcheck
		var p, w, f int
		checkExportIndex(dir, &p, &w, &f)
		if f != 1 || p != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want fail=1", p, w, f)
		}
	})

	t.Run("valid index: passCount++", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		content := `{"root_folder_id":"abc","conversations":{},"users":{},"updated_at":"2026-01-01T00:00:00Z"}`
		os.WriteFile(filepath.Join(dir, "export-index.json"), []byte(content), 0644) //nolint:errcheck
		var p, w, f int
		checkExportIndex(dir, &p, &w, &f)
		if p != 1 || w != 0 || f != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want pass=1", p, w, f)
		}
	})
}

// ---------------------------------------------------------------------------
// T037: chromeLaunchCmd tests
// ---------------------------------------------------------------------------

func TestChromeLaunchCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos     string
		port     int
		wantSubs []string
	}{
		{
			goos:     "darwin",
			port:     9222,
			wantSubs: []string{`open -a "Google Chrome"`, "--remote-debugging-port=9222"},
		},
		{
			goos:     "linux",
			port:     9222,
			wantSubs: []string{"google-chrome", "--remote-debugging-port=9222"},
		},
		{
			goos:     "windows",
			port:     9333,
			wantSubs: []string{"google-chrome", "--remote-debugging-port=9333"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			t.Parallel()
			got := chromeLaunchCmd(tt.goos, tt.port)
			for _, sub := range tt.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("chromeLaunchCmd(%q, %d) = %q, missing substring %q", tt.goos, tt.port, got, sub)
				}
			}
		})
	}
}
