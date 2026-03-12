package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
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
