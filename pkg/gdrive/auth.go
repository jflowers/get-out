// Package gdrive provides Google Drive and Docs API integration.
package gdrive

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jflowers/get-out/pkg/secrets"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
)

// Scopes required for Drive and Docs access.
var Scopes = []string{
	drive.DriveFileScope, // Full access to files created by this app
	docs.DocumentsScope,  // Read/write access to Docs
}

// Config holds configuration for Google API authentication.
type Config struct {
	// CredentialsPath is the path to credentials.json from Google Cloud Console
	CredentialsPath string

	// TokenPath is where to save/load the OAuth token
	TokenPath string
}

// DefaultConfig returns default paths for credentials and token.
func DefaultConfig(configDir string) *Config {
	return &Config{
		CredentialsPath: filepath.Join(configDir, "credentials.json"),
		TokenPath:       filepath.Join(configDir, "token.json"),
	}
}

// getTokenFromWeb starts a local server and initiates browser-based OAuth flow.
func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	// Use the explicit IPv4 loopback address to match the server bind address.
	// Using "localhost" risks sending callbacks to [::1] on IPv6-preferring systems
	// while the server only listens on 127.0.0.1, silently breaking the auth flow.
	config.RedirectURL = "http://127.0.0.1:8085/callback"

	// Generate a cryptographically random state token to protect against CSRF.
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("failed to generate OAuth state: %w", err)
	}
	expectedState := hex.EncodeToString(stateBytes)

	// Channel to receive the auth code.
	// errChan capacity 2: the callback handler and the ListenAndServe goroutine
	// can both send concurrently; non-blocking sends below prevent goroutine leaks.
	codeChan := make(chan string, 1)
	errChan := make(chan error, 2)

	// Use a dedicated mux (not the global http.DefaultServeMux) so repeated
	// calls don't panic with "multiple registrations for /callback".
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:         "127.0.0.1:8085", // bind to loopback only
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Validate CSRF state parameter
		if r.URL.Query().Get("state") != expectedState {
			http.Error(w, "invalid OAuth state parameter", http.StatusBadRequest)
			select {
			case errChan <- fmt.Errorf("invalid OAuth state: possible CSRF attack"):
			default:
			}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			select {
			case errChan <- fmt.Errorf("no code in callback"):
			default:
			}
			fmt.Fprintf(w, "Error: no authorization code received")
			return
		}
		codeChan <- code
		fmt.Fprintf(w, `
			<html><body>
			<h1>Authorization successful!</h1>
			<p>You can close this window and return to the terminal.</p>
			</body></html>
		`)
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			select {
			case errChan <- err:
			default:
			}
		}
	}()

	// Generate auth URL with the random state
	authURL := config.AuthCodeURL(expectedState, oauth2.AccessTypeOffline)

	fmt.Println()
	fmt.Println("To authorize this application, visit this URL in your browser:")
	fmt.Println()
	fmt.Println("  ", authURL)
	fmt.Println()
	fmt.Println("Waiting for authorization...")

	// Wait for code or error
	var code string
	select {
	case code = <-codeChan:
		// Success
	case err := <-errChan:
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		server.Shutdown(shutdownCtx) //nolint:errcheck
		return nil, err
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		server.Shutdown(shutdownCtx) //nolint:errcheck
		return nil, ctx.Err()
	}

	// Shutdown server using a background context so it completes even if caller's ctx is done.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx) //nolint:errcheck

	// Exchange code for token
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange code for token: %w", err)
	}

	return token, nil
}

// LoadTokenFromStore retrieves and parses a token from the SecretStore.
func LoadTokenFromStore(store secrets.SecretStore) (*oauth2.Token, error) {
	data, err := store.Get(secrets.KeyOAuthToken)
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, fmt.Errorf("parse token from store: %w", err)
	}
	return &token, nil
}

// saveTokenToStore serializes a token and writes it to the SecretStore.
func saveTokenToStore(store secrets.SecretStore, token *oauth2.Token) error {
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	return store.Set(secrets.KeyOAuthToken, string(data))
}

// ClientFromStore builds an authenticated HTTP client from a token already in
// the store plus credentials from the store. Unlike AuthenticateWithStore, this
// function never initiates a browser OAuth flow — it fails if no valid token
// exists. Use this for read-only status checks after EnsureTokenFreshWithStore
// has already been called.
func ClientFromStore(ctx context.Context, cfg *Config, store secrets.SecretStore) (*http.Client, error) {
	credData, err := store.Get(secrets.KeyClientCredentials)
	if err != nil {
		return nil, fmt.Errorf("credentials not found in store: %w", err)
	}
	oauthConfig, err := google.ConfigFromJSON([]byte(credData), Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}
	token, err := LoadTokenFromStore(store)
	if err != nil {
		return nil, fmt.Errorf("token not found in store: %w", err)
	}
	if !token.Valid() && token.RefreshToken == "" {
		return nil, fmt.Errorf("token is expired and has no refresh token; run 'get-out auth login'")
	}
	return oauthConfig.Client(ctx, token), nil
}

// AuthenticateWithStore performs the OAuth 2.0 flow using a SecretStore for
// credential and token I/O. This is the preferred function for CLI commands.
// If a saved token exists and is valid (or has a refresh token), it is reused.
// Otherwise, a browser-based consent flow is initiated.
func AuthenticateWithStore(ctx context.Context, cfg *Config, store secrets.SecretStore) (*http.Client, error) {
	// Read credentials from store
	credData, err := store.Get(secrets.KeyClientCredentials)
	if err != nil {
		return nil, fmt.Errorf("credentials not found in store (run 'get-out auth login' after placing credentials.json): %w", err)
	}

	oauthConfig, err := google.ConfigFromJSON([]byte(credData), Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	// Try to load existing token from store
	token, err := LoadTokenFromStore(store)
	if err == nil {
		if token.Valid() || token.RefreshToken != "" {
			return oauthConfig.Client(ctx, token), nil
		}
	}

	// Need to get new token via browser flow
	token, err = getTokenFromWeb(ctx, oauthConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to get token: %w", err)
	}

	// Save token to store
	if err := saveTokenToStore(store, token); err != nil {
		fmt.Printf("Warning: could not save token to store: %v\n", err)
	}

	return oauthConfig.Client(ctx, token), nil
}

// EnsureTokenFreshWithStore checks if the saved Google OAuth token is still
// valid and refreshes it if needed, using a SecretStore for I/O.
func EnsureTokenFreshWithStore(ctx context.Context, cfg *Config, store secrets.SecretStore) error {
	token, err := LoadTokenFromStore(store)
	if err != nil {
		return fmt.Errorf("no saved Google token found, run 'get-out auth login' first: %w", err)
	}

	if token.Valid() {
		return nil
	}

	if token.RefreshToken == "" {
		return fmt.Errorf("Google token expired and no refresh token available, run 'get-out auth login' to re-authenticate")
	}

	// Read credentials to build oauth config
	credData, err := store.Get(secrets.KeyClientCredentials)
	if err != nil {
		return fmt.Errorf("unable to read credentials for token refresh: %w", err)
	}

	oauthConfig, err := google.ConfigFromJSON([]byte(credData), Scopes...)
	if err != nil {
		return fmt.Errorf("unable to parse credentials: %w", err)
	}

	tokenSource := oauthConfig.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("Google token refresh failed, run 'get-out auth login' to re-authenticate: %w", err)
	}

	if err := saveTokenToStore(store, newToken); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save refreshed token to store: %v\n", err)
	}

	return nil
}
