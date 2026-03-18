package gdrive

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jflowers/get-out/pkg/secrets"
	"golang.org/x/oauth2"
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

	token, err := LoadTokenFromStore(store)
	if err == nil {
		t.Fatal("LoadTokenFromStore with empty store: expected error, got nil")
	}
	if token != nil {
		t.Error("expected nil token on error")
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

	token, err := LoadTokenFromStore(store)
	if err == nil {
		t.Fatal("LoadTokenFromStore with corrupt JSON: expected error, got nil")
	}
	if token != nil {
		t.Error("expected nil token on error")
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

	client, err := AuthenticateWithStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("AuthenticateWithStore with no credentials: expected error, got nil")
	}
	if client != nil {
		t.Error("expected nil client on error")
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

	client, err := ClientFromStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("ClientFromStore with no credentials: expected error, got nil")
	}
	if client != nil {
		t.Error("expected nil client on error")
	}
}

func TestClientFromStore_MissingToken(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeCredentials(t, store)
	// No token in store.
	cfg := &Config{}
	ctx := context.Background()

	client, err := ClientFromStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("ClientFromStore with no token: expected error, got nil")
	}
	if client != nil {
		t.Error("expected nil client on error")
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

	client, err := ClientFromStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("ClientFromStore with expired token and no refresh: expected error, got nil")
	}
	if client != nil {
		t.Error("expected nil client on error")
	}
}

// TestClientFromStore_ValidToken verifies the happy path: valid credentials +
// valid token → returns a non-nil *http.Client without network access.
func TestClientFromStore_ValidToken(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeCredentials(t, store)
	writeToken(t, store, map[string]any{
		"access_token": "ya29.valid",
		"expiry":       time.Now().Add(time.Hour).Format(time.RFC3339),
	})
	cfg := &Config{}
	ctx := context.Background()

	client, err := ClientFromStore(ctx, cfg, store)
	if err != nil {
		t.Fatalf("ClientFromStore with valid token: unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("ClientFromStore returned nil client")
	}
}

// ---------------------------------------------------------------------------
// saveTokenToStore tests
// ---------------------------------------------------------------------------

// TestSaveTokenToStore_RoundTrip verifies that saveTokenToStore serialises a
// token to the store and that LoadTokenFromStore can read it back correctly.
func TestSaveTokenToStore_RoundTrip(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	tok := &oauth2.Token{
		AccessToken:  "ya29.saved",
		RefreshToken: "1//saved",
		Expiry:       time.Now().Add(time.Hour).UTC().Truncate(time.Second),
	}
	if err := saveTokenToStore(store, tok); err != nil {
		t.Fatalf("saveTokenToStore: %v", err)
	}
	got, err := LoadTokenFromStore(store)
	if err != nil {
		t.Fatalf("LoadTokenFromStore after saveTokenToStore: %v", err)
	}
	if got.AccessToken != tok.AccessToken {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, tok.AccessToken)
	}
	if got.RefreshToken != tok.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, tok.RefreshToken)
	}
}

// ---------------------------------------------------------------------------
// EnsureTokenFreshWithStore — refresh branch tests (no network required)
// ---------------------------------------------------------------------------

// TestEnsureTokenFreshWithStore_ExpiredWithRefreshMissingCredentials verifies
// that the function returns an error about credentials when the token has a
// refresh token but no credentials are in the store.
func TestEnsureTokenFreshWithStore_ExpiredWithRefreshMissingCredentials(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	// Expired token WITH refresh token, but NO credentials in store.
	writeToken(t, store, map[string]any{
		"access_token":  "ya29.expired",
		"refresh_token": "1//refresh",
		"expiry":        time.Now().Add(-time.Hour).Format(time.RFC3339),
	})
	cfg := &Config{}
	ctx := context.Background()

	err := EnsureTokenFreshWithStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("expected error when credentials absent during refresh, got nil")
	}
	if !strings.Contains(err.Error(), "credentials") {
		t.Errorf("error %q should mention credentials", err.Error())
	}
}

// TestEnsureTokenFreshWithStore_ExpiredWithRefreshBadCredentials verifies
// that the function returns an error when credentials JSON cannot be parsed.
func TestEnsureTokenFreshWithStore_ExpiredWithRefreshBadCredentials(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeToken(t, store, map[string]any{
		"access_token":  "ya29.expired",
		"refresh_token": "1//refresh",
		"expiry":        time.Now().Add(-time.Hour).Format(time.RFC3339),
	})
	// Write malformed credentials JSON.
	if err := store.Set(secrets.KeyClientCredentials, "not-valid-json{{{"); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{}
	ctx := context.Background()

	err := EnsureTokenFreshWithStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("expected error with bad credentials JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// AuthenticateWithStore — additional branch tests
// ---------------------------------------------------------------------------

// TestAuthenticateWithStore_BadCredentialsJSON verifies that
// AuthenticateWithStore returns an error when credentials JSON cannot be parsed.
func TestAuthenticateWithStore_BadCredentialsJSON(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	if err := store.Set(secrets.KeyClientCredentials, "not-valid-json{{{"); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{}
	ctx := context.Background()

	client, err := AuthenticateWithStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("AuthenticateWithStore with bad credentials JSON: expected error, got nil")
	}
	if client != nil {
		t.Error("expected nil client on error")
	}
	if !strings.Contains(err.Error(), "parse credentials") {
		t.Errorf("error %q should mention 'parse credentials'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Error message content assertions (regression lock-down)
// ---------------------------------------------------------------------------

// TestEnsureTokenFreshWithStore_MissingToken_ErrorMessage verifies that the
// error message for a missing token contains the actionable "auth login" hint.
func TestEnsureTokenFreshWithStore_MissingToken_ErrorMessage(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	cfg := &Config{}
	ctx := context.Background()

	err := EnsureTokenFreshWithStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
	if !strings.Contains(err.Error(), "auth login") {
		t.Errorf("error %q should contain 'auth login'", err.Error())
	}
}

// TestEnsureTokenFreshWithStore_NoRefreshToken_ErrorMessage verifies that the
// error message for an expired token with no refresh token contains the
// actionable "auth login" hint.
func TestEnsureTokenFreshWithStore_NoRefreshToken_ErrorMessage(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeToken(t, store, map[string]any{
		"access_token": "ya29.expired",
		"expiry":       time.Now().Add(-time.Hour).Format(time.RFC3339),
	})
	cfg := &Config{}
	ctx := context.Background()

	err := EnsureTokenFreshWithStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("expected error for expired token with no refresh, got nil")
	}
	if !strings.Contains(err.Error(), "auth login") {
		t.Errorf("error %q should contain 'auth login'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Contract assertion tests (Group 8.2-8.4)
// ---------------------------------------------------------------------------

// TestClientFromStore_ValidToken_ContractAssertions verifies the returned
// http.Client has a transport configured (not the default zero-value).
func TestClientFromStore_ValidToken_ContractAssertions(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeCredentials(t, store)
	writeToken(t, store, map[string]any{
		"access_token": "ya29.contract-test",
		"expiry":       time.Now().Add(time.Hour).Format(time.RFC3339),
	})
	cfg := &Config{}
	ctx := context.Background()

	client, err := ClientFromStore(ctx, cfg, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("returned nil client")
	}
	// The oauth2 transport wraps http.DefaultTransport; verify it's set.
	if client.Transport == nil {
		t.Error("client.Transport is nil; expected oauth2 transport")
	}
}

// TestAuthenticateWithStore_ValidToken_ContractAssertions verifies the returned
// http.Client has a configured transport.
func TestAuthenticateWithStore_ValidToken_ContractAssertions(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeCredentials(t, store)
	writeToken(t, store, map[string]any{
		"access_token": "ya29.contract-test",
		"expiry":       time.Now().Add(time.Hour).Format(time.RFC3339),
	})
	cfg := &Config{}
	ctx := context.Background()

	client, err := AuthenticateWithStore(ctx, cfg, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("returned nil client")
	}
	if client.Transport == nil {
		t.Error("client.Transport is nil; expected oauth2 transport")
	}
}

// TestEnsureTokenFreshWithStore_ValidToken_ContractAssertions verifies that
// a valid token returns nil error (the contract: no error = token is fresh)
// and that re-reading the token confirms it is still valid.
func TestEnsureTokenFreshWithStore_ValidToken_ContractAssertions(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeToken(t, store, map[string]any{
		"access_token": "ya29.fresh",
		"expiry":       time.Now().Add(time.Hour).Format(time.RFC3339),
	})
	cfg := &Config{}
	ctx := context.Background()

	err := EnsureTokenFreshWithStore(ctx, cfg, store)
	if err != nil {
		t.Errorf("EnsureTokenFreshWithStore with fresh token returned error: %v", err)
	}

	// Re-read the token from the store to verify it remains valid (freshness contract).
	token, err := LoadTokenFromStore(store)
	if err != nil {
		t.Fatalf("LoadTokenFromStore after EnsureTokenFresh: %v", err)
	}
	if !token.Valid() {
		t.Error("token.Valid() = false after EnsureTokenFreshWithStore; expected true")
	}
	if token.AccessToken != "ya29.fresh" {
		t.Errorf("AccessToken = %q, want ya29.fresh", token.AccessToken)
	}
}

// TestLoadTokenFromStore_ContractAssertions verifies returned token fields.
func TestLoadTokenFromStore_ContractAssertions(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)

	expiry := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	writeToken(t, store, map[string]any{
		"access_token":  "ya29.contract",
		"refresh_token": "1//contract",
		"token_type":    "Bearer",
		"expiry":        expiry.Format(time.RFC3339),
	})

	token, err := LoadTokenFromStore(store)
	if err != nil {
		t.Fatalf("LoadTokenFromStore: %v", err)
	}

	if token.AccessToken != "ya29.contract" {
		t.Errorf("AccessToken = %q, want ya29.contract", token.AccessToken)
	}
	if token.RefreshToken != "1//contract" {
		t.Errorf("RefreshToken = %q, want 1//contract", token.RefreshToken)
	}
	if token.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want Bearer", token.TokenType)
	}
	if !token.Valid() {
		t.Error("token.Valid() = false, expected true for future expiry")
	}
}

// TestClientFromStore_ExpiredNoRefresh_ErrorMessage verifies the actionable message.
func TestClientFromStore_ExpiredNoRefresh_ErrorMessage(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeCredentials(t, store)
	writeToken(t, store, map[string]any{
		"access_token": "ya29.expired",
		"expiry":       time.Now().Add(-time.Hour).Format(time.RFC3339),
	})
	cfg := &Config{}
	ctx := context.Background()

	client, err := ClientFromStore(ctx, cfg, store)
	if err == nil {
		t.Fatal("expected error")
	}
	if client != nil {
		t.Error("expected nil client on error")
	}
	if !strings.Contains(err.Error(), "auth login") {
		t.Errorf("error %q should contain 'auth login' hint", err.Error())
	}
}

// ---------------------------------------------------------------------------
// ClientFromStore contract tests
// ---------------------------------------------------------------------------

func TestClientFromStore_BadCredentialsJSON(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)

	// Write invalid JSON as credentials
	if err := store.Set(secrets.KeyClientCredentials, "not valid json!!!"); err != nil {
		t.Fatalf("store.Set: %v", err)
	}

	// Write a valid token so we get past the token check
	writeToken(t, store, map[string]any{
		"access_token":  "ya29.test",
		"refresh_token": "1//test",
		"expiry":        time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	})

	cfg := &Config{}
	ctx := context.Background()

	client, err := ClientFromStore(ctx, cfg, store)
	// Contract assertion: error returned for malformed credentials
	if err == nil {
		t.Fatal("expected error for bad credentials JSON, got nil")
	}
	if client != nil {
		t.Error("expected nil client on error")
	}
	// Contract assertion: error mentions credential parsing
	if !strings.Contains(err.Error(), "credentials") {
		t.Errorf("expected error to mention 'credentials', got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Phase 2: Auth edge case contract tests
// ---------------------------------------------------------------------------

func TestClientFromStore_ExpiredWithRefreshToken(t *testing.T) {
	t.Parallel()
	store := testFileStore(t)
	writeCredentials(t, store)
	writeToken(t, store, map[string]any{
		"access_token":  "ya29.expired",
		"refresh_token": "1//refresh-token",
		"expiry":        time.Now().Add(-time.Hour).Format(time.RFC3339),
	})

	cfg := &Config{}
	ctx := context.Background()

	client, err := ClientFromStore(ctx, cfg, store)
	// Contract assertion: expired token with refresh token returns client (not error)
	if err != nil {
		t.Fatalf("expected success with refresh token, got error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	// Contract assertion: client has configured transport
	if client.Transport == nil {
		t.Error("expected configured transport")
	}
}
