package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/jflowers/get-out/pkg/chrome"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test browser connection and token extraction",
	Long: `Test the connection to Chrome and extract Slack credentials.

This command verifies that:
  1. Chrome is running with remote debugging enabled
  2. A Slack tab is open and logged in
  3. The xoxc token and xoxd cookie can be extracted

Prerequisites:
  Start Chrome/Chromium with remote debugging:
    /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222
  
  Or for Zen Browser, enable remote debugging in settings.

Then open Slack in a tab and log in to your workspace.`,
	RunE: runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
	fmt.Println("Browser Connection Test")
	fmt.Println("=======================")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to browser
	fmt.Printf("Connecting to Chrome on port %d...\n", chromePort)
	cfg := &chrome.Config{
		DebugPort: chromePort,
		Timeout:   10 * time.Second,
	}

	session, err := chrome.Connect(ctx, cfg)
	if err != nil {
		fmt.Println()
		fmt.Println("ERROR: Could not connect to browser")
		fmt.Printf("  %v\n", err)
		fmt.Println()
		fmt.Println("Make sure Chrome is running with remote debugging:")
		fmt.Printf("  /Applications/Google\\ Chrome.app/Contents/MacOS/Google\\ Chrome --remote-debugging-port=%d\n", chromePort)
		return err
	}
	defer session.Close()

	fmt.Println("  Connected!")
	fmt.Println()

	// List tabs
	fmt.Println("Browser tabs:")
	targets, err := session.ListTargets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tabs: %w", err)
	}

	for _, t := range targets {
		if t.Type == "page" {
			indicator := "  "
			if isSlackURL(t.URL) {
				indicator = "* "
			}
			title := t.Title
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			fmt.Printf("  %s%-50s  %s\n", indicator, title, truncateURL(t.URL))
		}
	}
	fmt.Println()

	// Find Slack tab
	fmt.Println("Looking for Slack tab...")
	slackTarget, err := session.FindSlackTarget(ctx)
	if err != nil {
		fmt.Println()
		fmt.Println("ERROR: No Slack tab found")
		fmt.Println("  Please open Slack in the browser and log in")
		return err
	}
	fmt.Printf("  Found: %s\n", slackTarget.Title)
	fmt.Println()

	// Extract credentials
	fmt.Println("Extracting Slack credentials...")
	creds, err := session.ExtractCredentials(ctx)
	if err != nil {
		fmt.Println()
		fmt.Println("ERROR: Could not extract credentials")
		fmt.Printf("  %v\n", err)
		return err
	}

	fmt.Println("  Success!")
	fmt.Println()
	fmt.Println("Credentials:")
	fmt.Printf("  Team Domain: %s\n", creds.TeamDomain)
	fmt.Printf("  Team ID:     %s\n", creds.TeamID)
	fmt.Printf("  Token:       %s...%s\n", creds.Token[:15], creds.Token[len(creds.Token)-4:])
	fmt.Printf("  Cookie:      %s...%s\n", creds.Cookie[:15], creds.Cookie[len(creds.Cookie)-4:])
	fmt.Println()
	fmt.Println("Browser connection test PASSED!")

	return nil
}

func isSlackURL(url string) bool {
	return len(url) > 0 && (contains(url, "slack.com") || contains(url, "app.slack.com"))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func truncateURL(url string) string {
	if len(url) > 60 {
		return url[:57] + "..."
	}
	return url
}
