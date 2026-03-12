package gdrive

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jflowers/get-out/pkg/secrets"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// testFileStore returns a FileStore rooted at a fresh temp dir.
func testFileStore(t *testing.T) *secrets.FileStore {
	t.Helper()
	return &secrets.FileStore{ConfigDir: t.TempDir()}
}

// writeToken serialises tok to the store under KeyOAuthToken.
func writeToken(t *testing.T, store secrets.SecretStore, tok any) {
	t.Helper()
	data, err := json.Marshal(tok)
	if err != nil {
		t.Fatalf("marshal token: %v", err)
	}
	if err := store.Set(secrets.KeyOAuthToken, string(data)); err != nil {
		t.Fatalf("store.Set token: %v", err)
	}
}

// writeCredentials writes a minimal (non-functional) credentials JSON to the store.
// The JSON structure matches what google.ConfigFromJSON expects so parsing succeeds.
func writeCredentials(t *testing.T, store secrets.SecretStore) {
	t.Helper()
	creds := `{"installed":{"client_id":"test.apps.googleusercontent.com","client_secret":"test-secret","redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}`
	if err := store.Set(secrets.KeyClientCredentials, creds); err != nil {
		t.Fatalf("store.Set credentials: %v", err)
	}
}

// ---------------------------------------------------------------------------
// LoadTokenFromStore tests
// ---------------------------------------------------------------------------

func TestLoadTokenFromStore_ValidJSON(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)

	expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	writeToken(t, store, map[string]any{
		"access_token":  "ya29.test",
		"refresh_token": "1//test",
		"expiry":        expiry.Format(time.RFC3339),
	})

	token, err := LoadTokenFromStore(store)
	if err != nil {
		t.Fatalf("LoadTokenFromStore: %v", err)
	}
	if token.AccessToken != "ya29.test" {
		t.Errorf("AccessToken = %q, want %q", token.AccessToken, "ya29.test")
	}
	if token.RefreshToken != "1//test" {
		t.Errorf("RefreshToken = %q, want %q", token.RefreshToken, "1//test")
	}
}

func TestLoadTokenFromStore_MissingToken(t *testing.T) {
	t.Parallel()
	store := testFileStore(t) // empty store — no token

	_, err := LoadTokenFromStore(store)
	if err == nil {
		t.Fatal("LoadTokenFromStore with empty store: expected error, got nil")
	}
}

func TestLoadTokenFromStore_CorruptJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Write corrupt JSON directly to token.json so FileStore.Get succeeds.
	if err := os.WriteFile(filepath.Join(dir, "token.json"), []byte("not-valid-json{{{"), 0600); err != nil {
		t.Fatal(err)
	}
	store := &secrets.FileStore{ConfigDir: dir}

	_, err := LoadTokenFromStore(store)
	if err == nil {
		t.Fatal("LoadTokenFromStore with corrupt JSON: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// EnsureTokenFreshWithStore tests (pure-logic branches — no network)
// ---------------------------------------------------------------------------

func TestEnsureTokenFreshWithStore_MissingToken(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	cfg := &Config{CredentialsPath: filepath.Join(t.TempDir(), "credentials.json")}
	ctx := context.Background()

	err := EnsureTokenFreshWithStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("EnsureTokenFreshWithStore with no token: expected error, got nil")
	}
}

func TestEnsureTokenFreshWithStore_ValidToken(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeToken(t, store, map[string]any{
		"access_token": "ya29.valid",
		"expiry":       time.Now().Add(time.Hour).Format(time.RFC3339),
	})
	cfg := &Config{}
	ctx := context.Background()

	// A valid token should return nil without touching the network.
	err := EnsureTokenFreshWithStore(ctx, cfg, store)
	if err != nil {
		t.Fatalf("EnsureTokenFreshWithStore with valid token: unexpected error: %v", err)
	}
}

func TestEnsureTokenFreshWithStore_NoRefreshToken(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	// Expired token with no refresh token
	writeToken(t, store, map[string]any{
		"access_token": "ya29.expired",
		"expiry":       time.Now().Add(-time.Hour).Format(time.RFC3339),
	})
	cfg := &Config{}
	ctx := context.Background()

	err := EnsureTokenFreshWithStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("EnsureTokenFreshWithStore with expired token and no refresh: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// AuthenticateWithStore tests (pure-logic branches — no network)
// ---------------------------------------------------------------------------

func TestAuthenticateWithStore_MissingCredentials(t *testing.T) {
	t.Parallel()
	store := testFileStore(t) // empty — no credentials
	cfg := &Config{}
	ctx := context.Background()

	_, err := AuthenticateWithStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("AuthenticateWithStore with no credentials: expected error, got nil")
	}
}

func TestAuthenticateWithStore_ValidTokenShortCircuits(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeCredentials(t, store)
	// Write a valid token — AuthenticateWithStore should return without browser flow.
	writeToken(t, store, map[string]any{
		"access_token": "ya29.valid",
		"expiry":       time.Now().Add(time.Hour).Format(time.RFC3339),
	})

	cfg := &Config{}
	ctx := context.Background()

	// Should return an http.Client without opening a browser.
	client, err := AuthenticateWithStore(ctx, cfg, store)
	if err != nil {
		t.Fatalf("AuthenticateWithStore with valid token: unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("AuthenticateWithStore returned nil client")
	}
}

func TestAuthenticateWithStore_WithRefreshToken(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeCredentials(t, store)
	// Expired token with a refresh token — AuthenticateWithStore should use it
	// without a browser flow (token.Valid() is false but RefreshToken != "").
	writeToken(t, store, map[string]any{
		"access_token":  "ya29.expired",
		"refresh_token": "1//refresh",
		"expiry":        time.Now().Add(-time.Hour).Format(time.RFC3339),
	})

	cfg := &Config{}
	ctx := context.Background()

	client, err := AuthenticateWithStore(ctx, cfg, store)
	if err != nil {
		t.Fatalf("AuthenticateWithStore with refresh token: unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("AuthenticateWithStore returned nil client")
	}
}

// ---------------------------------------------------------------------------
// ClientFromStore tests (pure-logic branches — no network)
// ---------------------------------------------------------------------------

func TestClientFromStore_MissingCredentials(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	cfg := &Config{}
	ctx := context.Background()

	_, err := ClientFromStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("ClientFromStore with no credentials: expected error, got nil")
	}
}

func TestClientFromStore_MissingToken(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeCredentials(t, store)
	// No token in store.
	cfg := &Config{}
	ctx := context.Background()

	_, err := ClientFromStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("ClientFromStore with no token: expected error, got nil")
	}
}

func TestClientFromStore_ExpiredNoRefresh(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeCredentials(t, store)
	writeToken(t, store, map[string]any{
		"access_token": "ya29.expired",
		"expiry":       time.Now().Add(-time.Hour).Format(time.RFC3339),
	})
	cfg := &Config{}
	ctx := context.Background()

	_, err := ClientFromStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("ClientFromStore with expired token and no refresh: expected error, got nil")
	}
}
