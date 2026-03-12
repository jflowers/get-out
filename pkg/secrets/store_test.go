package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// resetMock resets the global keyring provider to a clean mock state.
// Call this via t.Cleanup in every test that mutates the global provider.
func resetMock() { keyring.MockInit() }

// ---------------------------------------------------------------------------
// KeychainStore tests (using go-keyring's mock backend)
// ---------------------------------------------------------------------------

// TestKeychainStore_SetGetDelete verifies round-trip operations via MockInit.
func TestKeychainStore_SetGetDelete(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(resetMock)

	tests := []struct {
		name string
		key  string
	}{
		{"oauth-token", KeyOAuthToken},
		{"credentials-json", KeyClientCredentials},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := &KeychainStore{}
			t.Cleanup(func() { store.Delete(tc.key) }) //nolint:errcheck

			// Get on missing key returns ErrNotFound
			_, err := store.Get(tc.key)
			if err != ErrNotFound {
				t.Fatalf("Get(%q) on empty store: got err=%v, want ErrNotFound", tc.key, err)
			}

			// Set + Get round-trip
			val := "test-value-for-" + tc.key
			if err := store.Set(tc.key, val); err != nil {
				t.Fatalf("Set(%q): %v", tc.key, err)
			}
			got, err := store.Get(tc.key)
			if err != nil {
				t.Fatalf("Get(%q) after Set: %v", tc.key, err)
			}
			if got != val {
				t.Fatalf("Get(%q) = %q, want %q", tc.key, got, val)
			}

			// Delete + Get returns ErrNotFound
			if err := store.Delete(tc.key); err != nil {
				t.Fatalf("Delete(%q): %v", tc.key, err)
			}
			_, err = store.Get(tc.key)
			if err != ErrNotFound {
				t.Fatalf("Get(%q) after Delete: got err=%v, want ErrNotFound", tc.key, err)
			}

			// Delete on already-deleted key is not an error
			if err := store.Delete(tc.key); err != nil {
				t.Fatalf("Delete(%q) on absent key: %v", tc.key, err)
			}
		})
	}
}

// TestKeychainStore_SetError verifies that KeychainStore.Set wraps errors
// from the underlying keyring.
func TestKeychainStore_SetError(t *testing.T) {
	keyring.MockInitWithError(keyring.ErrNotFound)
	t.Cleanup(resetMock)
	store := &KeychainStore{}

	err := store.Set(KeyOAuthToken, "test-value")
	if err == nil {
		t.Fatal("expected error from Set with failing keyring, got nil")
	}
	if err.Error() == "" {
		t.Error("error should have a message")
	}
}

// TestKeychainStore_GetErrNotFound verifies ErrNotFound mapping from keyring.
func TestKeychainStore_GetErrNotFound(t *testing.T) {
	keyring.MockInitWithError(keyring.ErrNotFound)
	t.Cleanup(resetMock)
	store := &KeychainStore{}

	_, err := store.Get(KeyOAuthToken)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FileStore tests
// ---------------------------------------------------------------------------

// TestFileStore_SetGetDelete verifies file-based round-trip operations.
func TestFileStore_SetGetDelete(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		filename string
	}{
		{
			name:     "oauth-token",
			key:      KeyOAuthToken,
			value:    `{"access_token":"ya29.abc","refresh_token":"1//xyz","expiry":"2026-01-01T00:00:00Z"}`,
			filename: "token.json",
		},
		{
			name:     "credentials-json",
			key:      KeyClientCredentials,
			value:    `{"installed":{"client_id":"123.apps.googleusercontent.com","client_secret":"GOCSPX-test"}}`,
			filename: "credentials.json",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			store := &FileStore{ConfigDir: dir}

			// Get on missing returns ErrNotFound
			_, err := store.Get(tc.key)
			if err != ErrNotFound {
				t.Fatalf("Get(%q) on empty dir: got err=%v, want ErrNotFound", tc.key, err)
			}

			// Set + Get round-trip
			if err := store.Set(tc.key, tc.value); err != nil {
				t.Fatalf("Set(%q): %v", tc.key, err)
			}

			got, err := store.Get(tc.key)
			if err != nil {
				t.Fatalf("Get(%q) after Set: %v", tc.key, err)
			}
			if got != tc.value {
				t.Fatalf("Get(%q) = %q, want %q", tc.key, got, tc.value)
			}

			// Verify file exists at expected path
			path := filepath.Join(dir, tc.filename)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Fatalf("expected file %s to exist after Set", tc.filename)
			}

			// Delete + Get returns ErrNotFound
			if err := store.Delete(tc.key); err != nil {
				t.Fatalf("Delete(%q): %v", tc.key, err)
			}
			_, err = store.Get(tc.key)
			if err != ErrNotFound {
				t.Fatalf("Get(%q) after Delete: got err=%v, want ErrNotFound", tc.key, err)
			}

			// Delete on already-deleted key is not an error
			if err := store.Delete(tc.key); err != nil {
				t.Fatalf("Delete(%q) on absent key: %v", tc.key, err)
			}
		})
	}
}

// TestFileStore_Permissions verifies that Set creates files with mode 0600.
func TestFileStore_Permissions(t *testing.T) {
	dir := t.TempDir()
	store := &FileStore{ConfigDir: dir}

	if err := store.Set(KeyOAuthToken, `{"access_token":"ya29.test"}`); err != nil {
		t.Fatalf("Set: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "token.json"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0600 {
		t.Errorf("token.json mode: got %04o, want 0600", mode)
	}

	if err := store.Set(KeyClientCredentials, `{"installed":{}}`); err != nil {
		t.Fatalf("Set credentials: %v", err)
	}

	info, err = os.Stat(filepath.Join(dir, "credentials.json"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0600 {
		t.Errorf("credentials.json mode: got %04o, want 0600", mode)
	}
}

// TestFileStore_UnknownKey verifies that unknown keys return an error containing the key name.
func TestFileStore_UnknownKey(t *testing.T) {
	dir := t.TempDir()
	store := &FileStore{ConfigDir: dir}
	const unknownKey = "unknown-key"

	if _, err := store.Get(unknownKey); err == nil {
		t.Error("Get with unknown key: expected error, got nil")
	} else if !strings.Contains(err.Error(), unknownKey) {
		t.Errorf("Get error %q does not contain key name %q", err.Error(), unknownKey)
	}

	if err := store.Set(unknownKey, "val"); err == nil {
		t.Error("Set with unknown key: expected error, got nil")
	} else if !strings.Contains(err.Error(), unknownKey) {
		t.Errorf("Set error %q does not contain key name %q", err.Error(), unknownKey)
	}

	if err := store.Delete(unknownKey); err == nil {
		t.Error("Delete with unknown key: expected error, got nil")
	} else if !strings.Contains(err.Error(), unknownKey) {
		t.Errorf("Delete error %q does not contain key name %q", err.Error(), unknownKey)
	}
}

// ---------------------------------------------------------------------------
// NewStore / Backend tests
// ---------------------------------------------------------------------------

// TestBackendString verifies human-readable backend names.
func TestBackendString(t *testing.T) {
	tests := []struct {
		backend Backend
		want    string
	}{
		{BackendKeychain, "OS keychain"},
		{BackendFile, "plaintext files"},
		{Backend(99), "unknown"},
	}
	for _, tt := range tests {
		got := tt.backend.String()
		if got != tt.want {
			t.Errorf("Backend(%d).String() = %q, want %q", tt.backend, got, tt.want)
		}
	}
}

// TestNewStore_NoKeyring verifies that --no-keyring forces FileStore.
func TestNewStore_NoKeyring(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(resetMock)
	dir := t.TempDir()

	store, backend := NewStore(true, dir)
	if backend != BackendFile {
		t.Fatalf("NewStore(noKeyring=true): backend=%v, want BackendFile", backend)
	}
	if _, ok := store.(*FileStore); !ok {
		t.Fatalf("NewStore(noKeyring=true): store type=%T, want *FileStore", store)
	}
}

// TestNewStore_KeychainAvailable verifies that a working keychain returns KeychainStore.
func TestNewStore_KeychainAvailable(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(resetMock)
	dir := t.TempDir()

	store, backend := NewStore(false, dir)
	if backend != BackendKeychain {
		t.Fatalf("NewStore(noKeyring=false) with mock keyring: backend=%v, want BackendKeychain", backend)
	}
	if _, ok := store.(*KeychainStore); !ok {
		t.Fatalf("NewStore(noKeyring=false): store type=%T, want *KeychainStore", store)
	}
}

// TestNewStore_KeychainUnavailable verifies fallback to FileStore when the OS
// keychain returns a non-ErrNotFound error (e.g., keychain locked/daemon absent).
// The read-only probe in NewStore treats ErrNotFound as "keychain available"
// (key simply absent), but any other error signals a genuinely unavailable
// keychain and triggers the FileStore fallback.
func TestNewStore_KeychainUnavailable(t *testing.T) {
	keyring.MockInitWithError(errors.New("keychain locked"))
	t.Cleanup(resetMock)
	dir := t.TempDir()

	store, backend := NewStore(false, dir) // noKeyring=false — probe must decide
	if backend != BackendFile {
		t.Fatalf("NewStore with unavailable keychain: backend=%v, want BackendFile", backend)
	}
	if _, ok := store.(*FileStore); !ok {
		t.Fatalf("NewStore with unavailable keychain: store type=%T, want *FileStore", store)
	}
}

// ---------------------------------------------------------------------------
// Migrate tests
// ---------------------------------------------------------------------------

// TestMigrate_TokenFromDisk verifies that token.json is migrated to the store
// and deleted from disk.
func TestMigrate_TokenFromDisk(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(resetMock)
	dir := t.TempDir()

	tokenJSON := `{"access_token":"ya29.test","refresh_token":"1//test","expiry":"2026-06-01T00:00:00Z"}`
	tokenPath := filepath.Join(dir, "token.json")
	if err := os.WriteFile(tokenPath, []byte(tokenJSON), 0600); err != nil {
		t.Fatalf("write token.json: %v", err)
	}

	store := &KeychainStore{}
	if err := Migrate(store, dir, false, nil); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Token should be in store
	val, err := store.Get(KeyOAuthToken)
	if err != nil {
		t.Fatalf("token not in store: %v", err)
	}
	if val != tokenJSON {
		t.Errorf("stored token: got %q, want %q", val, tokenJSON)
	}

	// token.json should be deleted
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Error("token.json was not deleted after migration")
	}
}

// TestMigrate_CredentialsNonInteractive verifies that in non-interactive mode,
// credentials.json is migrated to the store but NOT deleted from disk.
func TestMigrate_CredentialsNonInteractive(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(resetMock)
	dir := t.TempDir()

	credsJSON := `{"installed":{"client_id":"123.apps.googleusercontent.com","client_secret":"GOCSPX-test"}}`
	credsPath := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(credsPath, []byte(credsJSON), 0600); err != nil {
		t.Fatalf("write credentials.json: %v", err)
	}

	store := &KeychainStore{}
	if err := Migrate(store, dir, false, nil); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Credentials should be in store
	val, err := store.Get(KeyClientCredentials)
	if err != nil {
		t.Fatalf("credentials not in store: %v", err)
	}
	if val != credsJSON {
		t.Errorf("stored credentials: got %q, want %q", val, credsJSON)
	}

	// credentials.json should still be on disk (non-interactive = no deletion)
	if _, err := os.Stat(credsPath); os.IsNotExist(err) {
		t.Error("credentials.json was deleted in non-interactive mode — should be preserved")
	}
}

// TestMigrate_CredentialsInteractiveAccept verifies that in interactive mode,
// when the user accepts deletion, credentials.json is deleted from disk.
func TestMigrate_CredentialsInteractiveAccept(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(resetMock)
	dir := t.TempDir()

	credsJSON := `{"installed":{"client_id":"123.apps.googleusercontent.com"}}`
	credsPath := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(credsPath, []byte(credsJSON), 0600); err != nil {
		t.Fatalf("write credentials.json: %v", err)
	}

	store := &KeychainStore{}
	mockPrompt := func(message string) (bool, error) { return true, nil }

	if err := Migrate(store, dir, true, mockPrompt); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Credentials should be in store
	if _, err := store.Get(KeyClientCredentials); err != nil {
		t.Fatalf("credentials not in store: %v", err)
	}

	// credentials.json should be deleted (user accepted)
	if _, err := os.Stat(credsPath); !os.IsNotExist(err) {
		t.Error("credentials.json was NOT deleted after user accepted — expected deletion")
	}
}

// TestMigrate_CredentialsInteractiveDecline verifies that in interactive mode,
// when the user declines deletion, credentials.json is preserved.
func TestMigrate_CredentialsInteractiveDecline(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(resetMock)
	dir := t.TempDir()

	credsJSON := `{"installed":{"client_id":"456.apps.googleusercontent.com"}}`
	credsPath := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(credsPath, []byte(credsJSON), 0600); err != nil {
		t.Fatalf("write credentials.json: %v", err)
	}

	store := &KeychainStore{}
	mockPrompt := func(message string) (bool, error) { return false, nil }

	if err := Migrate(store, dir, true, mockPrompt); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Credentials should be in store
	if _, err := store.Get(KeyClientCredentials); err != nil {
		t.Fatalf("credentials not in store: %v", err)
	}

	// credentials.json should still be on disk (user declined)
	if _, err := os.Stat(credsPath); os.IsNotExist(err) {
		t.Error("credentials.json was deleted despite user declining")
	}
}

// TestMigrate_Idempotent verifies that running Migrate twice produces no errors
// and secrets remain in store.
func TestMigrate_Idempotent(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(resetMock)
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "token.json"), []byte(`{"access_token":"ya29.test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "credentials.json"), []byte(`{"installed":{}}`), 0600); err != nil {
		t.Fatal(err)
	}

	store := &KeychainStore{}

	if err := Migrate(store, dir, false, nil); err != nil {
		t.Fatalf("Migrate #1: %v", err)
	}
	if err := Migrate(store, dir, false, nil); err != nil {
		t.Fatalf("Migrate #2 (idempotent): %v", err)
	}

	if _, err := store.Get(KeyOAuthToken); err != nil {
		t.Error("token missing from store after second migration")
	}
	if _, err := store.Get(KeyClientCredentials); err != nil {
		t.Error("credentials missing from store after second migration")
	}
}

// TestMigrate_PartialState verifies crash recovery: when a secret exists in
// both the store AND on disk, Migrate cleans up the file.
func TestMigrate_PartialState(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(resetMock)
	dir := t.TempDir()

	tokenJSON := `{"access_token":"ya29.partial"}`
	tokenPath := filepath.Join(dir, "token.json")
	if err := os.WriteFile(tokenPath, []byte(tokenJSON), 0600); err != nil {
		t.Fatalf("write token.json: %v", err)
	}

	store := &KeychainStore{}
	if err := store.Set(KeyOAuthToken, tokenJSON); err != nil {
		t.Fatalf("pre-populate store: %v", err)
	}

	if err := Migrate(store, dir, false, nil); err != nil {
		t.Fatalf("Migrate (partial state): %v", err)
	}

	val, err := store.Get(KeyOAuthToken)
	if err != nil {
		t.Fatalf("token missing from store: %v", err)
	}
	if val != tokenJSON {
		t.Errorf("store value changed: got %q, want %q", val, tokenJSON)
	}

	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Error("token.json should be deleted after crash-recovery migration")
	}
}

// TestMigrate_NothingToMigrate verifies that Migrate is a no-op when no
// plaintext secrets exist on disk.
func TestMigrate_NothingToMigrate(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(resetMock)
	dir := t.TempDir()

	store := &KeychainStore{}
	if err := Migrate(store, dir, false, nil); err != nil {
		t.Fatalf("Migrate with no files: %v", err)
	}
}
