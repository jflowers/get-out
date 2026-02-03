package cli

import (
	"fmt"

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
  - APIs enabled: Google Drive API, Google Docs API`,
	RunE: runAuth,
}

func init() {
	rootCmd.AddCommand(authCmd)
}

func runAuth(cmd *cobra.Command, args []string) error {
	fmt.Println("Google Authentication")
	fmt.Println("=====================")
	fmt.Println()

	// TODO: Implement Google OAuth flow
	// 1. Load credentials.json from configDir
	// 2. Start local HTTP server for OAuth callback
	// 3. Open browser to Google consent screen
	// 4. Exchange code for tokens
	// 5. Save refresh token to token.json

	fmt.Printf("Config directory: %s\n", configDir)
	fmt.Println()
	fmt.Println("Google OAuth flow not yet implemented.")
	fmt.Println("Please ensure you have:")
	fmt.Printf("  1. %s/credentials.json (OAuth client credentials)\n", configDir)
	fmt.Println("  2. Google Drive API enabled in your GCP project")
	fmt.Println("  3. Google Docs API enabled in your GCP project")

	return nil
}
