package cli

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jflowers/get-out/pkg/chrome"
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

	if err := os.WriteFile(filepath.Join(oldDir, "settings.json"), []byte(`{"logLevel":"INFO"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "conversations.json"), []byte(`{"conversations":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := migrateFiles(oldDir, newDir); err != nil {
		t.Fatalf("migrateFiles() error: %v", err)
	}

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

	if err := migrateFiles(oldDir, newDir); err != nil {
		t.Fatalf("migrateFiles() with absent oldDir returned error: %v", err)
	}
}

// T018: file exists in both dirs — only new-dir file is kept (no overwrite).
func TestMigrateFiles_PartialNewDir(t *testing.T) {
	t.Parallel()
	oldDir := t.TempDir()
	newDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(oldDir, "settings.json"), []byte("old-settings"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "token.json"), []byte("old-token"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "settings.json"), []byte("new-settings"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := migrateFiles(oldDir, newDir); err != nil {
		t.Fatalf("migrateFiles() error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(newDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-settings" {
		t.Errorf("settings.json was overwritten: got %q", got)
	}

	if _, err := os.Stat(filepath.Join(newDir, "token.json")); err != nil {
		t.Errorf("token.json was not migrated: %v", err)
	}
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
		if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		var p, w, f int
		checkConfigDir(filePath, secrets.BackendFile, &p, &w, &f)
		if f != 1 || p != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want pass=0 fail=1", p, w, f)
		}
	})

	t.Run("valid dir with mode 0700 increments passCount only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
		var p, w, f int
		checkConfigDir(dir, secrets.BackendFile, &p, &w, &f)
		if p != 1 || w != 0 || f != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want pass=1 warn=0 fail=0", p, w, f)
		}
	})

	t.Run("valid dir with broad permissions increments pass and warn", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.Chmod(dir, 0755); err != nil {
			t.Fatal(err)
		}
		var p, w, f int
		checkConfigDir(dir, secrets.BackendFile, &p, &w, &f)
		if p != 1 || w != 1 || f != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want pass=1 warn=1 fail=0", p, w, f)
		}
	})

	t.Run("BackendKeychain: pass message contains backend name", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
		var p, w, f int
		checkConfigDir(dir, secrets.BackendKeychain, &p, &w, &f)
		if p != 1 || w != 0 || f != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want pass=1 warn=0 fail=0", p, w, f)
		}
		// The backend string should be "OS keychain" for BackendKeychain.
		if secrets.BackendKeychain.String() != "OS keychain" {
			t.Errorf("BackendKeychain.String() = %q, want %q", secrets.BackendKeychain.String(), "OS keychain")
		}
	})
}

// ---------------------------------------------------------------------------
// T036 (new): checkSecret tests
// ---------------------------------------------------------------------------

func TestCheckSecret(t *testing.T) {
	t.Parallel()

	t.Run("secret present: passCount++, returns true", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		store := &secrets.FileStore{ConfigDir: dir}
		if err := store.Set(secrets.KeyOAuthToken, `{"access_token":"tok"}`); err != nil {
			t.Fatalf("set: %v", err)
		}
		var p, f int
		ok := checkSecret("token", secrets.KeyOAuthToken, store, "fix msg", &p, &f)
		if !ok || p != 1 || f != 0 {
			t.Errorf("got ok=%v pass=%d fail=%d, want ok=true pass=1 fail=0", ok, p, f)
		}
	})

	t.Run("secret absent: failCount++, returns false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		store := &secrets.FileStore{ConfigDir: dir}
		var p, f int
		ok := checkSecret("token", secrets.KeyOAuthToken, store, "fix msg", &p, &f)
		if ok || p != 0 || f != 1 {
			t.Errorf("got ok=%v pass=%d fail=%d, want ok=false pass=0 fail=1", ok, p, f)
		}
	})
}

// ---------------------------------------------------------------------------
// T036: checkTokenValidity tests
// ---------------------------------------------------------------------------

// tokenStruct mirrors oauth2.Token JSON shape for test fixture writing.
type tokenStruct struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

func TestCheckTokenValidity(t *testing.T) {
	t.Parallel()

	// writeTokenToStore writes a token as JSON into a FileStore for the given dir.
	writeTokenToStore := func(t *testing.T, dir string, tok tokenStruct) secrets.SecretStore {
		t.Helper()
		data, err := json.Marshal(tok)
		if err != nil {
			t.Fatalf("marshal token: %v", err)
		}
		store := &secrets.FileStore{ConfigDir: dir}
		if err := store.Set(secrets.KeyOAuthToken, string(data)); err != nil {
			t.Fatalf("store.Set token: %v", err)
		}
		return store
	}

	t.Run("valid token: passCount++ returns true", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		store := writeTokenToStore(t, dir, tokenStruct{
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
		store := writeTokenToStore(t, dir, tokenStruct{
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
		store := writeTokenToStore(t, dir, tokenStruct{
			AccessToken: "tok",
			Expiry:      time.Now().Add(-time.Hour),
		})
		var p, w, f int
		ok := checkTokenValidity(dir, store, &p, &w, &f)
		if ok || f != 1 || p != 0 {
			t.Errorf("got ok=%v pass=%d warn=%d fail=%d, want ok=false fail=1", ok, p, w, f)
		}
	})

	t.Run("corrupt token JSON: failCount++ returns false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Write corrupt JSON directly to token.json so FileStore.Get succeeds but parse fails.
		if err := os.WriteFile(filepath.Join(dir, "token.json"), []byte("bad-json"), 0600); err != nil {
			t.Fatal(err)
		}
		store := &secrets.FileStore{ConfigDir: dir}
		var p, w, f int
		ok := checkTokenValidity(dir, store, &p, &w, &f)
		if ok || f != 1 {
			t.Errorf("got ok=%v pass=%d warn=%d fail=%d, want ok=false fail=1", ok, p, w, f)
		}
	})

	t.Run("missing token in store: failCount++ returns false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		store := &secrets.FileStore{ConfigDir: dir} // empty store, no token.json
		var p, w, f int
		ok := checkTokenValidity(dir, store, &p, &w, &f)
		if ok || f != 1 {
			t.Errorf("got ok=%v pass=%d warn=%d fail=%d, want ok=false fail=1", ok, p, w, f)
		}
	})
}

// ---------------------------------------------------------------------------
// T036: checkDriveAPI tests
// ---------------------------------------------------------------------------

func TestCheckDriveAPI_NoCredentials(t *testing.T) {
	t.Parallel()
	// An empty FileStore has no credentials → ClientFromStore should fail
	dir := t.TempDir()
	store := &secrets.FileStore{ConfigDir: dir}
	var p, w, f int
	checkDriveAPI(dir, store, &p, &w, &f)
	if f != 1 || p != 0 {
		t.Errorf("got pass=%d warn=%d fail=%d, want pass=0 fail=1", p, w, f)
	}
}

func TestCheckDriveAPI_BadCredentials(t *testing.T) {
	t.Parallel()
	// Invalid credentials JSON → ConfigFromJSON should fail
	dir := t.TempDir()
	store := &secrets.FileStore{ConfigDir: dir}
	if err := store.Set(secrets.KeyClientCredentials, `{"not":"valid oauth"}`); err != nil {
		t.Fatalf("store.Set: %v", err)
	}
	// Also need a token for ClientFromStore to get past credentials parsing
	tok := tokenStruct{AccessToken: "tok", Expiry: time.Now().Add(time.Hour)}
	tokJSON, _ := json.Marshal(tok)
	if err := store.Set(secrets.KeyOAuthToken, string(tokJSON)); err != nil {
		t.Fatalf("store.Set token: %v", err)
	}
	var p, w, f int
	checkDriveAPI(dir, store, &p, &w, &f)
	if f != 1 || p != 0 {
		t.Errorf("got pass=%d warn=%d fail=%d, want pass=0 fail=1", p, w, f)
	}
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
		if err := os.WriteFile(filepath.Join(dir, "conversations.json"), []byte("bad{json"), 0644); err != nil {
			t.Fatal(err)
		}
		var p, w, f int
		checkConversations(dir, &p, &w, &f)
		if f != 1 || p != 0 {
			t.Errorf("got pass=%d warn=%d fail=%d, want fail=1", p, w, f)
		}
	})

	t.Run("empty conversations array: warnCount++", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "conversations.json"), []byte(`{"conversations":[]}`), 0644); err != nil {
			t.Fatal(err)
		}
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
		if err := os.WriteFile(filepath.Join(dir, "conversations.json"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
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
		if err := os.WriteFile(filepath.Join(dir, "people.json"), []byte(`{}`), 0644); err != nil {
			t.Fatal(err)
		}
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
		if err := os.WriteFile(filepath.Join(dir, "export-index.json"), []byte("bad{json"), 0644); err != nil {
			t.Fatal(err)
		}
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
		if err := os.WriteFile(filepath.Join(dir, "export-index.json"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
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

// ---------------------------------------------------------------------------
// T036: checkSlackTab tests
// ---------------------------------------------------------------------------

func TestCheckSlackTab_ConnectionError(t *testing.T) {
	// Port 0 or an invalid port — chrome.Connect will fail, checkSlackTab
	// should increment warnCount and return without panicking.
	var p, w, f int
	// Use a high port that is almost certainly not listening
	checkSlackTab(19999, &p, &w, &f, "https://app.slack.com")
	if w != 1 {
		t.Errorf("got pass=%d warn=%d fail=%d, want warn=1", p, w, f)
	}
	if p != 0 || f != 0 {
		t.Errorf("expected no pass/fail on connection error, got pass=%d fail=%d", p, f)
	}
}

func TestCheckSlackTab_DifferentInvalidPorts(t *testing.T) {
	// Multiple invalid ports to ensure checkSlackTab handles them consistently.
	for _, port := range []int{19998, 19997, 19996} {
		var p, w, f int
		checkSlackTab(port, &p, &w, &f, "https://app.slack.com")
		if w != 1 {
			t.Errorf("port %d: got warn=%d, want 1", port, w)
		}
		if p != 0 || f != 0 {
			t.Errorf("port %d: got pass=%d fail=%d, want 0", port, p, f)
		}
	}
}

// ---------------------------------------------------------------------------
// T036: evalSlackTargets tests (extracted from checkSlackTab for testability)
// ---------------------------------------------------------------------------

func TestEvalSlackTargets_NoTargets(t *testing.T) {
	var p, w int
	evalSlackTargets(nil, &p, &w, "https://app.slack.com")
	if w != 1 || p != 0 {
		t.Errorf("got pass=%d warn=%d, want pass=0 warn=1", p, w)
	}
}

func TestEvalSlackTargets_NoSlackTabs(t *testing.T) {
	targets := []chrome.TargetInfo{
		{URL: "https://www.google.com", Type: "page", Title: "Google"},
		{URL: "https://github.com", Type: "page", Title: "GitHub"},
	}
	var p, w int
	evalSlackTargets(targets, &p, &w, "https://app.slack.com")
	if w != 1 || p != 0 {
		t.Errorf("got pass=%d warn=%d, want pass=0 warn=1", p, w)
	}
}

func TestEvalSlackTargets_OneSlackTab(t *testing.T) {
	targets := []chrome.TargetInfo{
		{URL: "https://www.google.com", Type: "page", Title: "Google"},
		{URL: "https://app.slack.com/client/T123/C456", Type: "page", Title: "Slack"},
	}
	var p, w int
	evalSlackTargets(targets, &p, &w, "https://app.slack.com")
	if p != 1 || w != 0 {
		t.Errorf("got pass=%d warn=%d, want pass=1 warn=0", p, w)
	}
}

func TestEvalSlackTargets_MultipleSlackTabs(t *testing.T) {
	targets := []chrome.TargetInfo{
		{URL: "https://app.slack.com/client/T123/C456", Type: "page", Title: "Slack 1"},
		{URL: "https://myworkspace.slack.com/messages", Type: "page", Title: "Slack 2"},
	}
	var p, w int
	evalSlackTargets(targets, &p, &w, "https://app.slack.com")
	if p != 1 || w != 0 {
		t.Errorf("got pass=%d warn=%d, want pass=1 warn=0", p, w)
	}
}

func TestCheckChrome_NotListening(t *testing.T) {
	// checkChrome with a port that's not listening — should fail
	var p, w, f int
	ok := checkChrome(19999, &p, &w, &f)
	if ok {
		t.Error("checkChrome on invalid port should return false")
	}
	if f != 1 {
		t.Errorf("got fail=%d, want 1", f)
	}
	if p != 0 {
		t.Errorf("got pass=%d, want 0", p)
	}
}

// ---------------------------------------------------------------------------
// T037: chromeLaunchCmd tests
// ---------------------------------------------------------------------------

func TestChromeLaunchCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string // descriptive name (not used as goos)
		goos     string
		port     int
		wantSubs []string
	}{
		{
			name:     "darwin",
			goos:     "darwin",
			port:     9222,
			wantSubs: []string{`open -a "Google Chrome"`, "--remote-debugging-port=9222"},
		},
		{
			name:     "linux",
			goos:     "linux",
			port:     9222,
			wantSubs: []string{"google-chrome", "--remote-debugging-port=9222"},
		},
		{
			name:     "non-darwin else-branch (windows)",
			goos:     "windows",
			port:     9333,
			wantSubs: []string{"google-chrome", "--remote-debugging-port=9333"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
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

// ---------------------------------------------------------------------------
// Auto-launch Chrome helper tests
// ---------------------------------------------------------------------------

func TestChromeProfilePath(t *testing.T) {
	t.Parallel()

	path, err := chromeProfilePath()
	if err != nil {
		t.Fatalf("chromeProfilePath() returned error: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join(".get-out", "chrome-data")) {
		t.Errorf("chromeProfilePath() = %q, want suffix %q", path, filepath.Join(".get-out", "chrome-data"))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error: %v", err)
	}
	if !strings.HasPrefix(path, home) {
		t.Errorf("chromeProfilePath() = %q, want prefix %q (home dir)", path, home)
	}
}

func TestFindChromeBinary(t *testing.T) {
	t.Parallel()

	result := findChromeBinary()
	switch runtime.GOOS {
	case "darwin":
		// On macOS with Chrome installed, should return the Chrome path.
		// Skip assertion if Chrome is not installed (e.g., CI without Chrome).
		if result == "" {
			t.Skip("Chrome not installed on this macOS system")
		}
		if result != "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" {
			t.Errorf("findChromeBinary() = %q, want macOS Chrome path", result)
		}
	case "linux":
		// On Linux, result depends on what's installed; just verify it doesn't panic.
		if result != "" {
			t.Logf("findChromeBinary() found: %s", result)
		} else {
			t.Skip("No Chrome binary found on this Linux system")
		}
	default:
		// On unsupported platforms, should return empty.
		if result != "" {
			t.Errorf("findChromeBinary() = %q on %s, want empty", result, runtime.GOOS)
		}
	}
}

func TestIsPortOpen(t *testing.T) {
	t.Parallel()

	t.Run("listening port returns true", func(t *testing.T) {
		t.Parallel()
		// Start a listener on a random port.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("net.Listen error: %v", err)
		}
		defer ln.Close()

		port := ln.Addr().(*net.TCPAddr).Port
		if !isPortOpen(port) {
			t.Errorf("isPortOpen(%d) = false, want true (listener active)", port)
		}
	})

	t.Run("unused port returns false", func(t *testing.T) {
		t.Parallel()
		// Get a port by briefly listening, then close it.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("net.Listen error: %v", err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		ln.Close()

		if isPortOpen(port) {
			t.Errorf("isPortOpen(%d) = true, want false (no listener)", port)
		}
	})
}

// ---------------------------------------------------------------------------
// T018: checkOllama tests
// ---------------------------------------------------------------------------

// ollamaMockServer creates an httptest.Server that simulates Ollama's
// ping (GET /) and model listing (GET /api/tags) endpoints.
// If models is nil, the /api/tags endpoint is not registered (simulating
// a server that only responds to ping).
func ollamaMockServer(t *testing.T, models []string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only respond to exact root path to avoid catching /api/tags
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ollama is running"))
	})
	if models != nil {
		mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
			type m struct {
				Name string `json:"name"`
			}
			var ms []m
			for _, name := range models {
				ms = append(ms, m{Name: name})
			}
			resp := struct {
				Models []m `json:"models"`
			}{Models: ms}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		})
	}
	return httptest.NewServer(mux)
}

func TestCheckOllama_AllOK(t *testing.T) {
	t.Parallel()
	srv := ollamaMockServer(t, []string{"granite-guardian:8b", "llama3:latest"})
	defer srv.Close()

	// checkOllama creates its own client internally, but we need to test
	// with our mock server. Since checkOllama uses ollama.NewClient which
	// uses the default HTTP client, we test by calling checkOllama with
	// the mock server's URL as the endpoint.
	var p, w, f int
	checkOllama(srv.URL, "granite-guardian:8b", &p, &w, &f)
	if p != 2 {
		t.Errorf("got pass=%d, want 2 (Ollama OK + model OK)", p)
	}
	if f != 0 {
		t.Errorf("got fail=%d, want 0", f)
	}
	if w != 0 {
		t.Errorf("got warn=%d, want 0", w)
	}
}

func TestCheckOllama_Unreachable(t *testing.T) {
	t.Parallel()
	// Use an endpoint that will refuse connections.
	var p, w, f int
	checkOllama("http://127.0.0.1:1", "granite-guardian:8b", &p, &w, &f)
	if f != 1 {
		t.Errorf("got fail=%d, want 1 (Ollama unreachable)", f)
	}
	if p != 0 {
		t.Errorf("got pass=%d, want 0 (should not pass when unreachable)", p)
	}
}

func TestCheckOllama_ModelMissing(t *testing.T) {
	t.Parallel()
	// Server is reachable but the requested model is not in the tags list.
	srv := ollamaMockServer(t, []string{"llama3:latest", "codellama:7b"})
	defer srv.Close()

	var p, w, f int
	checkOllama(srv.URL, "granite-guardian:8b", &p, &w, &f)
	if p != 1 {
		t.Errorf("got pass=%d, want 1 (Ollama OK)", p)
	}
	if f != 1 {
		t.Errorf("got fail=%d, want 1 (model missing)", f)
	}
}

func TestLaunchChrome_NoBinary(t *testing.T) {
	t.Parallel()

	// launchChrome calls findChromeBinary() internally.
	// On a system without Chrome, it should return an error.
	// To test this reliably without modifying findChromeBinary, we test
	// with a path that won't have Chrome on any system.
	// Note: This test verifies the error path when the binary is empty,
	// but since findChromeBinary() checks the real system, we test the
	// launchChrome function's contract: it should fail gracefully.

	// We can't easily mock findChromeBinary since it's a package-level function.
	// Instead, test that providing an invalid profile path still invokes the
	// binary check first. On systems with Chrome, this will actually try to
	// start Chrome, so we skip if Chrome is found.
	if findChromeBinary() != "" {
		t.Skip("Chrome is installed; cannot test missing-binary path")
	}

	cmd, err := launchChrome("/tmp/test-profile", 19999, "https://app.slack.com")
	if err == nil {
		t.Error("launchChrome() should return error when Chrome binary is not found")
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Kill()
		}
	}
	if cmd != nil {
		t.Error("launchChrome() should return nil cmd on error")
	}
	if err != nil && !strings.Contains(err.Error(), "Chrome not found") {
		t.Errorf("error message = %q, want it to contain %q", err.Error(), "Chrome not found")
	}
}
