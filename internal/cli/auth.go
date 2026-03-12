package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/gdrive"
	"github.com/jflowers/get-out/pkg/secrets"
	"github.com/spf13/cobra"
)

// authCmd is the parent command group for Google authentication sub-commands.
var authCmd = &cobra.Command{
	Use:          "auth",
	Short:        "Manage Google authentication",
	SilenceUsage: true,
	Long: `Manage Google OAuth authentication for Drive and Docs API access.

Sub-commands:
  login   Authenticate with Google (opens browser)
  status  Show current authentication status`,
}

// authLoginCmd performs the OAuth browser flow.
var authLoginCmd = &cobra.Command{
	Use:          "login",
	Short:        "Authenticate with Google APIs (opens browser)",
	SilenceUsage: true,
	Long: `Authenticate with Google Drive and Docs APIs using OAuth 2.0.

This command will:
  1. Load credentials.json from the config directory
  2. Open a browser for Google OAuth consent
  3. Save the refresh token to token.json

Prerequisites:
  - credentials.json from Google Cloud Console (OAuth 2.0 Client ID)
  - APIs enabled: Google Drive API, Google Docs API

To get credentials.json:
  1. Go to https://console.cloud.google.com/apis/credentials
  2. Create a new OAuth 2.0 Client ID (Desktop application)
  3. Download the JSON and save as credentials.json in your config directory`,
	RunE: runAuthLogin,
}

// authStatusCmd shows the current authentication status without triggering a browser flow.
var authStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Show current Google authentication status",
	SilenceUsage: true,
	Long:         `Show whether the saved Google OAuth token is valid, and which account is connected.`,
	RunE:         runAuthStatus,
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	rootCmd.AddCommand(authCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	fmt.Println("Google Authentication")
	fmt.Println("=====================")
	fmt.Println()

	// Load settings to check for custom credentials path (for fallback path display)
	settingsPath := filepath.Join(configDir, "settings.json")
	settings, err := config.LoadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	cfg := gdrive.DefaultConfig(configDir)
	if settings.GoogleCredentialsFile != "" {
		cfg.CredentialsPath = settings.GoogleCredentialsFile
	}

	// Check for credentials via store
	if _, err := secretStore.Get(secrets.KeyClientCredentials); err != nil {
		fmt.Println("ERROR: credentials not found")
		fmt.Println()
		fmt.Printf("Please download OAuth credentials from Google Cloud Console\n")
		fmt.Printf("and save them to: %s\n", cfg.CredentialsPath)
		fmt.Println()
		fmt.Println("Steps:")
		fmt.Println("  1. Go to https://console.cloud.google.com/apis/credentials")
		fmt.Println("  2. Create OAuth 2.0 Client ID (Desktop application)")
		fmt.Println("  3. Download JSON and save as credentials.json")
		fmt.Println("  4. Run: get-out init  (to migrate credentials to keychain)")
		fmt.Println()
		fmt.Println("Also ensure these APIs are enabled:")
		fmt.Println("  - Google Drive API")
		fmt.Println("  - Google Docs API")
		return fmt.Errorf("credentials not found in store or at %s", cfg.CredentialsPath)
	}

	// Check if already authenticated
	if token, err := gdrive.LoadTokenFromStore(secretStore); err == nil && token.Valid() {
		fmt.Println("Already authenticated!")
		fmt.Println()
		fmt.Println("To re-authenticate, delete the stored token and run this command again:")
		fmt.Println("  get-out auth login  (will re-run the OAuth flow)")
		return nil
	}

	fmt.Printf("Secret storage: %s\n", secretBackend)
	fmt.Println()

	// Start OAuth flow
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Println("Starting OAuth flow...")
	client, err := gdrive.AuthenticateWithStore(ctx, cfg, secretStore)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Verify by creating a Drive client
	fmt.Println()
	fmt.Println("Verifying connection...")
	driveClient, err := gdrive.NewClient(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to create Drive client: %w", err)
	}

	// Test by getting about info
	about, err := driveClient.Drive.About.Get().Fields("user").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to verify Drive access: %w", err)
	}

	fmt.Println()
	fmt.Println("Authentication successful!")
	fmt.Printf("Connected as: %s (%s)\n", about.User.DisplayName, about.User.EmailAddress)
	fmt.Println()
	fmt.Println("You can now use 'get-out export' to export Slack messages to Google Docs.")

	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	settingsPath := filepath.Join(configDir, "settings.json")
	settings, err := config.LoadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	cfg := gdrive.DefaultConfig(configDir)
	if settings.GoogleCredentialsFile != "" {
		cfg.CredentialsPath = settings.GoogleCredentialsFile
	}

	// Check 1: credentials in store
	if _, err := secretStore.Get(secrets.KeyClientCredentials); err == nil {
		fmt.Printf("Credentials: ✓ found (%s)\n", secretBackend)
	} else {
		fmt.Printf("Credentials: ✗ not found\n")
		fmt.Printf("             → Place credentials.json at %s and run: get-out init\n", cfg.CredentialsPath)
	}

	// Check 2: token in store
	if _, err := secretStore.Get(secrets.KeyOAuthToken); err != nil {
		fmt.Printf("Token:       ✗ not found\n")
		fmt.Println("             → Run: get-out auth login")
		return fmt.Errorf("not authenticated")
	}
	fmt.Printf("Token:       ✓ found (%s)\n", secretBackend)

	// Check 3: token validity + silent refresh
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := gdrive.EnsureTokenFreshWithStore(ctx, cfg, secretStore); err != nil {
		fmt.Println("Token:       ✗ expired and could not be refreshed")
		fmt.Println("             → Run: get-out auth login")
		return fmt.Errorf("token expired: %w", err)
	}

	// Read expiry for display (re-read after potential refresh)
	token, err := gdrive.LoadTokenFromStore(secretStore)
	if err == nil {
		fmt.Printf("Expiry:      %s\n", token.Expiry.Local().Format("2006-01-02 15:04:05 MST"))
	}

	// Check 4: Drive API call to get email (use ClientFromStore — no browser flow)
	httpClient, err := gdrive.ClientFromStore(ctx, cfg, secretStore)
	if err != nil {
		fmt.Println("Drive API:   ✗ could not authenticate")
		return fmt.Errorf("drive authentication failed: %w", err)
	}
	driveClient, err := gdrive.NewClient(ctx, httpClient)
	if err != nil {
		fmt.Println("Drive API:   ✗ could not create client")
		return fmt.Errorf("drive client error: %w", err)
	}
	about, err := driveClient.Drive.About.Get().Fields("user").Context(ctx).Do()
	if err != nil {
		fmt.Println("Drive API:   ✗ request failed")
		return fmt.Errorf("drive API error: %w", err)
	}
	fmt.Printf("Account:     %s (%s)\n", about.User.DisplayName, about.User.EmailAddress)
	fmt.Println("Status:      ✓ authenticated")

	return nil
}
