// Package gdrive provides Google Drive and Docs API integration.
package gdrive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

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

// Authenticate performs the OAuth 2.0 flow and returns an authenticated HTTP client.
// If a saved token exists and is valid, it will be reused.
// Otherwise, a browser-based consent flow will be initiated.
func Authenticate(ctx context.Context, cfg *Config) (*http.Client, error) {
	// Read credentials
	credBytes, err := os.ReadFile(cfg.CredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file %s: %w", cfg.CredentialsPath, err)
	}

	// Parse OAuth config
	oauthConfig, err := google.ConfigFromJSON(credBytes, Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	// Try to load existing token
	token, err := loadToken(cfg.TokenPath)
	if err == nil && token.Valid() {
		return oauthConfig.Client(ctx, token), nil
	}

	// Need to get new token via browser flow
	token, err = getTokenFromWeb(ctx, oauthConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to get token: %w", err)
	}

	// Save token for future use
	if err := saveToken(cfg.TokenPath, token); err != nil {
		// Log warning but don't fail
		fmt.Printf("Warning: could not save token: %v\n", err)
	}

	return oauthConfig.Client(ctx, token), nil
}

// getTokenFromWeb starts a local server and initiates browser-based OAuth flow.
func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	// Use a local redirect for desktop apps
	config.RedirectURL = "http://localhost:8085/callback"

	// Channel to receive the auth code
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Start local server to receive callback
	server := &http.Server{Addr: ":8085"}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback")
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
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Generate auth URL
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

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
		server.Shutdown(ctx)
		return nil, err
	case <-ctx.Done():
		server.Shutdown(ctx)
		return nil, ctx.Err()
	}

	// Shutdown server
	server.Shutdown(ctx)

	// Exchange code for token
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange code for token: %w", err)
	}

	return token, nil
}

// loadToken reads a token from a file.
func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var token oauth2.Token
	if err := json.NewDecoder(f).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

// saveToken saves a token to a file.
func saveToken(path string, token *oauth2.Token) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}

// HasCredentials checks if credentials.json exists.
func HasCredentials(cfg *Config) bool {
	_, err := os.Stat(cfg.CredentialsPath)
	return err == nil
}

// HasToken checks if a saved token exists.
func HasToken(cfg *Config) bool {
	_, err := os.Stat(cfg.TokenPath)
	return err == nil
}

// IsTokenValid checks if the saved token is still valid.
func IsTokenValid(cfg *Config) bool {
	token, err := loadToken(cfg.TokenPath)
	if err != nil {
		return false
	}
	return token.Valid()
}
