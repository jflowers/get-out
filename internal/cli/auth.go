package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/gdrive"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Google APIs",
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
	RunE: runAuth,
}

func init() {
	rootCmd.AddCommand(authCmd)
}

func runAuth(cmd *cobra.Command, args []string) error {
	fmt.Println("Google Authentication")
	fmt.Println("=====================")
	fmt.Println()

	// Load settings to check for custom credentials path
	settingsPath := filepath.Join(configDir, "settings.json")
	settings, err := config.LoadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	cfg := gdrive.DefaultConfig(configDir)
	if settings.GoogleCredentialsFile != "" {
		cfg.CredentialsPath = settings.GoogleCredentialsFile
	}

	// Check for credentials
	if !gdrive.HasCredentials(cfg) {
		fmt.Println("ERROR: credentials.json not found")
		fmt.Println()
		fmt.Printf("Please download OAuth credentials from Google Cloud Console\n")
		fmt.Printf("and save them to: %s\n", cfg.CredentialsPath)
		fmt.Println()
		fmt.Println("Steps:")
		fmt.Println("  1. Go to https://console.cloud.google.com/apis/credentials")
		fmt.Println("  2. Create OAuth 2.0 Client ID (Desktop application)")
		fmt.Println("  3. Download JSON and save as credentials.json")
		fmt.Println()
		fmt.Println("Also ensure these APIs are enabled:")
		fmt.Println("  - Google Drive API")
		fmt.Println("  - Google Docs API")
		return fmt.Errorf("credentials.json not found at %s", cfg.CredentialsPath)
	}

	// Check if already authenticated
	if gdrive.IsTokenValid(cfg) {
		fmt.Println("Already authenticated!")
		fmt.Printf("Token file: %s\n", cfg.TokenPath)
		fmt.Println()
		fmt.Println("To re-authenticate, delete the token file and run this command again.")
		return nil
	}

	fmt.Printf("Credentials: %s\n", cfg.CredentialsPath)
	fmt.Printf("Token will be saved to: %s\n", cfg.TokenPath)
	fmt.Println()

	// Start OAuth flow
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Println("Starting OAuth flow...")
	client, err := gdrive.Authenticate(ctx, cfg)
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
