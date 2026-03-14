package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jflowers/get-out/pkg/secrets"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Test helpers for auth tests
// ---------------------------------------------------------------------------

// withSecretStore runs fn with the package-level secretStore set to store,
// then restores the original value. Not safe for t.Parallel().
func withSecretStore(store secrets.SecretStore, fn func()) {
	orig := secretStore
	secretStore = store
	defer func() { secretStore = orig }()
	fn()
}

// withConfigDir runs fn with the package-level configDir set to dir.
func withConfigDir(dir string, fn func()) {
	orig := configDir
	configDir = dir
	defer func() { configDir = orig }()
	fn()
}

// minimalSettingsJSON returns a minimal settings.json file in dir.
func writeMinimalSettings(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}
}

// tokenJSON returns a JSON-encoded token with the given access token and expiry offset.
func tokenJSON(accessToken, refreshToken string, expiryOffset time.Duration) string {
	type tok struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		Expiry       time.Time `json:"expiry"`
	}
	data, _ := json.Marshal(tok{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       time.Now().Add(expiryOffset),
	})
	return string(data)
}

// fakeCmd returns a cobra.Command with a dummy RunE to satisfy argument requirements.
func fakeCmd() *cobra.Command {
	return &cobra.Command{Use: "test"}
}

// ---------------------------------------------------------------------------
// runAuthLogin tests — early-exit branches only (no live credentials)
// ---------------------------------------------------------------------------

func TestRunAuthLogin_MissingCredentials(t *testing.T) {
	dir := t.TempDir()
	writeMinimalSettings(t, dir)

	store := &secrets.FileStore{ConfigDir: dir} // empty store — no credentials

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthLogin(fakeCmd(), nil)
			if err == nil {
				t.Fatal("runAuthLogin with no credentials: expected error, got nil")
			}
		})
	})
}

func TestRunAuthLogin_AlreadyAuthenticated(t *testing.T) {
	dir := t.TempDir()
	writeMinimalSettings(t, dir)

	store := &secrets.FileStore{ConfigDir: dir}
	// Put credentials in the store so the credential check passes.
	creds := `{"installed":{"client_id":"test.apps.googleusercontent.com","client_secret":"test-secret","redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}`
	if err := store.Set(secrets.KeyClientCredentials, creds); err != nil {
		t.Fatalf("set credentials: %v", err)
	}
	// Put a valid (unexpired) token in the store so the "already authenticated" branch fires.
	if err := store.Set(secrets.KeyOAuthToken, tokenJSON("ya29.valid", "refresh", time.Hour)); err != nil {
		t.Fatalf("set token: %v", err)
	}

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthLogin(fakeCmd(), nil)
			// The "already authenticated" branch returns nil.
			if err != nil {
				t.Fatalf("runAuthLogin when already authenticated: unexpected error: %v", err)
			}
		})
	})
}

// ---------------------------------------------------------------------------
// runAuthStatus tests — early-exit branches only (no live credentials)
// ---------------------------------------------------------------------------

func TestRunAuthStatus_MissingToken(t *testing.T) {
	dir := t.TempDir()
	writeMinimalSettings(t, dir)

	store := &secrets.FileStore{ConfigDir: dir}
	// Credentials present, but no token.
	creds := `{"installed":{"client_id":"test.apps.googleusercontent.com","client_secret":"test-secret","redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}`
	if err := store.Set(secrets.KeyClientCredentials, creds); err != nil {
		t.Fatalf("set credentials: %v", err)
	}

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthStatus(fakeCmd(), nil)
			if err == nil {
				t.Fatal("runAuthStatus with no token: expected error, got nil")
			}
		})
	})
}

func TestRunAuthStatus_MissingCredentials(t *testing.T) {
	dir := t.TempDir()
	writeMinimalSettings(t, dir)

	store := &secrets.FileStore{ConfigDir: dir} // empty store

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			// Missing credentials prints a ✗ line but does NOT return an error
			// (the spec only returns an error when the token is absent or expired).
			// Missing token causes the error return.
			// Here both are missing, so the token check returns error.
			err := runAuthStatus(fakeCmd(), nil)
			if err == nil {
				t.Fatal("runAuthStatus with no credentials or token: expected error, got nil")
			}
		})
	})
}

func TestRunAuthStatus_ExpiredTokenNoRefresh(t *testing.T) {
	dir := t.TempDir()
	writeMinimalSettings(t, dir)

	store := &secrets.FileStore{ConfigDir: dir}
	creds := `{"installed":{"client_id":"test.apps.googleusercontent.com","client_secret":"test-secret","redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}`
	if err := store.Set(secrets.KeyClientCredentials, creds); err != nil {
		t.Fatalf("set credentials: %v", err)
	}
	// Expired token with no refresh token — EnsureTokenFreshWithStore will fail.
	if err := store.Set(secrets.KeyOAuthToken, tokenJSON("ya29.expired", "", -time.Hour)); err != nil {
		t.Fatalf("set token: %v", err)
	}

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthStatus(fakeCmd(), nil)
			if err == nil {
				t.Fatal("runAuthStatus with expired token and no refresh: expected error, got nil")
			}
		})
	})
}

// ---------------------------------------------------------------------------
// Additional auth tests for coverage improvement
// ---------------------------------------------------------------------------

func TestRunAuthLogin_CustomCredentialsPath(t *testing.T) {
	dir := t.TempDir()
	// Settings with custom credentials file path
	settings := `{"googleCredentialsFile": "/custom/path/creds.json"}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settings), 0644); err != nil {
		t.Fatal(err)
	}

	store := &secrets.FileStore{ConfigDir: dir} // empty store — no credentials

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthLogin(fakeCmd(), nil)
			if err == nil {
				t.Fatal("expected error for missing credentials")
			}
			// Error message should reference the custom credentials path
		})
	})
}

func TestRunAuthLogin_SettingsLoadError(t *testing.T) {
	dir := t.TempDir()
	// Write invalid JSON settings
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("invalid json{{{"), 0644); err != nil {
		t.Fatal(err)
	}

	store := &secrets.FileStore{ConfigDir: dir}

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthLogin(fakeCmd(), nil)
			if err == nil {
				t.Fatal("expected error for invalid settings")
			}
		})
	})
}

func TestRunAuthStatus_SettingsLoadError(t *testing.T) {
	dir := t.TempDir()
	// Write invalid JSON settings
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("not-json{"), 0644); err != nil {
		t.Fatal(err)
	}

	store := &secrets.FileStore{ConfigDir: dir}

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthStatus(fakeCmd(), nil)
			if err == nil {
				t.Fatal("expected error for invalid settings")
			}
		})
	})
}

func TestRunAuthStatus_NoCredentialsButTokenPresent(t *testing.T) {
	// Credentials missing prints ✗ line but continues to token check.
	// Token present but expired without refresh → returns error from token check.
	dir := t.TempDir()
	writeMinimalSettings(t, dir)

	store := &secrets.FileStore{ConfigDir: dir}
	// No credentials, but add a token
	if err := store.Set(secrets.KeyOAuthToken, tokenJSON("ya29.expired", "", -time.Hour)); err != nil {
		t.Fatal(err)
	}

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthStatus(fakeCmd(), nil)
			// Should fail at EnsureTokenFreshWithStore since there are no credentials
			if err == nil {
				t.Fatal("expected error for expired token with no credentials")
			}
		})
	})
}

func TestRunAuthStatus_MissingSettingsFile(t *testing.T) {
	// No settings.json at all → should return "failed to load settings" error
	dir := t.TempDir()
	// Don't write settings.json
	store := &secrets.FileStore{ConfigDir: dir}

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthStatus(fakeCmd(), nil)
			if err == nil {
				t.Fatal("expected error for missing settings")
			}
		})
	})
}

func TestRunAuthStatus_CustomCredentialsPathSetsConfig(t *testing.T) {
	// Settings with a custom GoogleCredentialsFile — verify the credentials path
	// is picked up even though the actual flow fails (no real credentials).
	dir := t.TempDir()
	settings := `{"googleCredentialsFile": "/custom/path/creds.json"}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settings), 0644); err != nil {
		t.Fatal(err)
	}

	store := &secrets.FileStore{ConfigDir: dir}
	// No credentials or token — should fail at token-not-found.
	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthStatus(fakeCmd(), nil)
			if err == nil {
				t.Fatal("expected error for missing token")
			}
		})
	})
}

func TestRunAuthStatus_CredentialsPresentTokenAbsent(t *testing.T) {
	// Credentials present (so first check prints ✓) but no token → error
	dir := t.TempDir()
	writeMinimalSettings(t, dir)

	store := &secrets.FileStore{ConfigDir: dir}
	creds := `{"installed":{"client_id":"test.apps.googleusercontent.com","client_secret":"test-secret","redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}`
	if err := store.Set(secrets.KeyClientCredentials, creds); err != nil {
		t.Fatal(err)
	}
	// No token at all

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthStatus(fakeCmd(), nil)
			if err == nil {
				t.Fatal("expected error for missing token with credentials present")
			}
		})
	})
}

func TestRunAuthLogin_MissingSettingsFile(t *testing.T) {
	// No settings.json at all → should return "failed to load settings" error
	dir := t.TempDir()
	// Don't write settings.json
	store := &secrets.FileStore{ConfigDir: dir}

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthLogin(fakeCmd(), nil)
			if err == nil {
				t.Fatal("expected error for missing settings")
			}
		})
	})
}

func TestRunAuthStatus_CustomCredentialsWithExpiredToken(t *testing.T) {
	dir := t.TempDir()
	settings := `{"googleCredentialsFile": "/custom/creds.json"}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settings), 0644); err != nil {
		t.Fatal(err)
	}

	store := &secrets.FileStore{ConfigDir: dir}
	// Put credentials and an expired token with no refresh
	creds := `{"installed":{"client_id":"test.apps.googleusercontent.com","client_secret":"test-secret","redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}`
	if err := store.Set(secrets.KeyClientCredentials, creds); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(secrets.KeyOAuthToken, tokenJSON("ya29.expired", "", -time.Hour)); err != nil {
		t.Fatal(err)
	}

	withConfigDir(dir, func() {
		withSecretStore(store, func() {
			err := runAuthStatus(fakeCmd(), nil)
			if err == nil {
				t.Fatal("expected error for expired token")
			}
		})
	})
}

// ---------------------------------------------------------------------------
// authStatusCore direct tests — exercises paths without runAuthStatus wrapper
// ---------------------------------------------------------------------------

func TestAuthStatusCore_MissingSettings(t *testing.T) {
	dir := t.TempDir()
	// No settings.json
	store := &secrets.FileStore{ConfigDir: dir}
	err := authStatusCore(dir, store, secrets.BackendFile)
	if err == nil {
		t.Fatal("expected error for missing settings")
	}
}

func TestAuthStatusCore_CredentialsPresentTokenMissing(t *testing.T) {
	dir := t.TempDir()
	writeMinimalSettings(t, dir)
	store := &secrets.FileStore{ConfigDir: dir}
	creds := `{"installed":{"client_id":"test.apps.googleusercontent.com","client_secret":"test-secret","redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}`
	if err := store.Set(secrets.KeyClientCredentials, creds); err != nil {
		t.Fatal(err)
	}
	err := authStatusCore(dir, store, secrets.BackendFile)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestAuthStatusCore_ExpiredTokenNoRefresh(t *testing.T) {
	dir := t.TempDir()
	writeMinimalSettings(t, dir)
	store := &secrets.FileStore{ConfigDir: dir}
	creds := `{"installed":{"client_id":"test.apps.googleusercontent.com","client_secret":"test-secret","redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}`
	if err := store.Set(secrets.KeyClientCredentials, creds); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(secrets.KeyOAuthToken, tokenJSON("ya29.expired", "", -time.Hour)); err != nil {
		t.Fatal(err)
	}
	err := authStatusCore(dir, store, secrets.BackendFile)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestAuthStatusCore_NoCredentialsButTokenPresent(t *testing.T) {
	dir := t.TempDir()
	writeMinimalSettings(t, dir)
	store := &secrets.FileStore{ConfigDir: dir}
	// No credentials, but add a token (expired, no refresh)
	if err := store.Set(secrets.KeyOAuthToken, tokenJSON("ya29.expired", "", -time.Hour)); err != nil {
		t.Fatal(err)
	}
	err := authStatusCore(dir, store, secrets.BackendFile)
	// Should fail at EnsureTokenFreshWithStore
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthStatusCore_ValidTokenDriveAPIFails(t *testing.T) {
	// Token is valid (not expired) so EnsureTokenFreshWithStore returns nil.
	// ClientFromStore succeeds (returns an HTTP client with fake token).
	// NewClient succeeds (creates Drive wrapper).
	// Drive.About.Get() fails (invalid token → network/auth error).
	// This covers the LoadTokenFromStore → expiry display and Drive API error branches.
	dir := t.TempDir()
	writeMinimalSettings(t, dir)
	store := &secrets.FileStore{ConfigDir: dir}

	creds := `{"installed":{"client_id":"test.apps.googleusercontent.com","client_secret":"test-secret","redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}`
	if err := store.Set(secrets.KeyClientCredentials, creds); err != nil {
		t.Fatal(err)
	}
	// Valid (not-yet-expired) token with a refresh token
	if err := store.Set(secrets.KeyOAuthToken, tokenJSON("ya29.fake-valid", "refresh-tok", time.Hour)); err != nil {
		t.Fatal(err)
	}

	err := authStatusCore(dir, store, secrets.BackendFile)
	// Should fail at Drive.About.Get() because the token is fake
	if err == nil {
		t.Fatal("expected Drive API error with fake token")
	}
	// The error should be from the Drive API call
	if !strings.Contains(err.Error(), "drive") {
		t.Errorf("expected drive-related error, got: %v", err)
	}
}
