// get-out: A Slack message exporter using session-based authentication.
// Designed for enterprise workspaces where traditional API access is restricted.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jflowers/get-out/pkg/chrome"
	"github.com/jflowers/get-out/pkg/parser"
	"github.com/jflowers/get-out/pkg/slackapi"
	"github.com/jflowers/get-out/pkg/util"
)

var (
	// Connection flags
	debugURL  = flag.String("debug-url", "", "Chrome DevTools Protocol URL (e.g., ws://127.0.0.1:9222)")
	workspace = flag.String("workspace", "", "Slack workspace (e.g., mycompany or mycompany.slack.com)")

	// Export flags
	outputDir     = flag.String("output", "./slack-export", "Output directory for exported messages")
	exportTypes   = flag.String("types", "im,mpim", "Conversation types to export: im,mpim,private_channel,channel")
	resume        = flag.Bool("resume", true, "Resume from last checkpoint")
	resetProgress = flag.Bool("reset", false, "Reset checkpoint and start fresh")

	// Operation flags
	testOnly = flag.Bool("test", false, "Test API access without exporting")
	listOnly = flag.Bool("list", false, "List available conversations without exporting")
	verbose  = flag.Bool("verbose", false, "Enable verbose output")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `get-out: Slack Message Exporter

A tool to export Slack messages using browser session authentication.
Designed for enterprise workspaces where traditional API access is restricted.

PREREQUISITES:
  1. Start Chrome/Zen with remote debugging enabled:
     Chrome:  /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222
     Zen:     Similar flag or via browser settings
  
  2. Navigate to your Slack workspace and log in

USAGE:
  get-out [flags]

FLAGS:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
EXAMPLES:
  # Test API access
  get-out -test

  # List all DMs and group messages
  get-out -list

  # Export all DMs to ./slack-export
  get-out -types=im

  # Export everything including private channels
  get-out -types=im,mpim,private_channel

  # Resume a previous export
  get-out -resume

  # Connect to specific debug port
  get-out -debug-url=ws://127.0.0.1:9222 -test

`)
	}
	flag.Parse()

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInterrupted, saving progress...")
		cancel()
	}()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	fmt.Println("get-out: Slack Message Exporter")
	fmt.Println("================================")
	fmt.Println()

	// Connect to browser
	fmt.Println("[1/4] Connecting to browser...")
	session, err := connectToBrowser(ctx)
	if err != nil {
		return fmt.Errorf("browser connection failed: %w", err)
	}
	defer session.Close()

	// Extract credentials
	fmt.Println("[2/4] Extracting Slack credentials...")
	creds, err := session.ExtractCredentials(ctx)
	if err != nil {
		return fmt.Errorf("credential extraction failed: %w", err)
	}

	printCredentialSummary(creds)

	// Create API client
	client := slackapi.NewClient(&slackapi.ClientConfig{
		Token:        creds.Token,
		Cookie:       creds.Cookie,
		TeamID:       creds.TeamID,
		IsEnterprise: creds.Enterprise,
		RateLimit:    150 * time.Millisecond,
	})

	// Test mode
	if *testOnly {
		return runTestMode(ctx, client)
	}

	// List mode
	if *listOnly {
		return runListMode(ctx, client)
	}

	// Export mode
	return runExportMode(ctx, client, creds)
}

func connectToBrowser(ctx context.Context) (*chrome.Session, error) {
	cfg := &chrome.Config{
		DebugURL:       *debugURL,
		SlackWorkspace: *workspace,
		Timeout:        30 * time.Second,
	}

	session, err := chrome.Connect(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Find existing Slack page
	if err := session.FindSlackPage(ctx, *workspace); err != nil {
		fmt.Printf("  Note: %v\n", err)
		if *workspace != "" {
			fmt.Printf("  Attempting to navigate to %s...\n", *workspace)
			if err := session.NavigateToSlack(ctx, *workspace); err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("no Slack page found - please open Slack in your browser or specify -workspace")
		}
	}

	fmt.Println("  Connected to browser successfully")
	return session, nil
}

func printCredentialSummary(creds *chrome.SlackCredentials) {
	fmt.Printf("  Token: %s...%s\n", creds.Token[:10], creds.Token[len(creds.Token)-4:])
	fmt.Printf("  Team ID: %s\n", creds.TeamID)
	fmt.Printf("  User ID: %s\n", creds.UserID)
	if creds.Enterprise {
		fmt.Println("  Workspace: ENTERPRISE (some APIs may be restricted)")
	} else {
		fmt.Println("  Workspace: Standard")
	}
	fmt.Println()
}

func runTestMode(ctx context.Context, client *slackapi.Client) error {
	fmt.Println("[3/4] Testing API access...")

	report, err := client.TestAccess(ctx)
	if err != nil {
		return fmt.Errorf("API test failed: %w", err)
	}

	fmt.Println()
	fmt.Println("API Access Report")
	fmt.Println("-----------------")
	fmt.Printf("Team: %s (%s)\n", report.Team, report.TeamID)
	fmt.Printf("User: %s (%s)\n", report.User, report.UserID)
	fmt.Printf("Enterprise: %v\n", report.IsEnterprise)
	fmt.Println()
	fmt.Println("API Availability:")

	for api, status := range report.APIs {
		statusStr := "OK"
		if !status.Available {
			statusStr = "BLOCKED"
			if status.Restricted {
				statusStr = "RESTRICTED (enterprise)"
			}
			if status.Error != "" {
				statusStr += fmt.Sprintf(" (%s)", status.Error)
			}
		}
		fmt.Printf("  %-25s %s\n", api, statusStr)
	}

	fmt.Println()
	if report.APIs["conversations.list"].Available {
		fmt.Println("Export via API: AVAILABLE")
		fmt.Println("You can run 'get-out' to export your messages.")
	} else {
		fmt.Println("Export via API: BLOCKED")
		fmt.Println("DOM scraping fallback will be required (not yet implemented).")
	}

	return nil
}

func runListMode(ctx context.Context, client *slackapi.Client) error {
	fmt.Println("[3/4] Fetching conversation list...")

	convs, err := client.ListConversations(ctx, *exportTypes)
	if err != nil {
		return fmt.Errorf("failed to list conversations: %w", err)
	}

	fmt.Println()
	fmt.Printf("Found %d conversations:\n", len(convs))
	fmt.Println()

	// Group by type
	var ims, mpims, privates, channels []slackapi.Conversation
	for _, c := range convs {
		switch {
		case c.IsIM:
			ims = append(ims, c)
		case c.IsMPIM:
			mpims = append(mpims, c)
		case c.IsPrivate:
			privates = append(privates, c)
		default:
			channels = append(channels, c)
		}
	}

	if len(ims) > 0 {
		fmt.Printf("Direct Messages (%d):\n", len(ims))
		for _, c := range ims {
			fmt.Printf("  - %s (user: %s)\n", c.ID, c.User)
		}
		fmt.Println()
	}

	if len(mpims) > 0 {
		fmt.Printf("Group Messages (%d):\n", len(mpims))
		for _, c := range mpims {
			fmt.Printf("  - %s (%s)\n", c.Name, c.ID)
		}
		fmt.Println()
	}

	if len(privates) > 0 {
		fmt.Printf("Private Channels (%d):\n", len(privates))
		for _, c := range privates {
			fmt.Printf("  - #%s (%s)\n", c.Name, c.ID)
		}
		fmt.Println()
	}

	if len(channels) > 0 {
		fmt.Printf("Channels (%d):\n", len(channels))
		for _, c := range channels {
			fmt.Printf("  - #%s (%s)\n", c.Name, c.ID)
		}
		fmt.Println()
	}

	return nil
}

func runExportMode(ctx context.Context, client *slackapi.Client, creds *chrome.SlackCredentials) error {
	// Initialize checkpoint
	checkpoint, err := util.NewCheckpoint(*outputDir)
	if err != nil {
		return fmt.Errorf("failed to initialize checkpoint: %w", err)
	}

	if *resetProgress {
		fmt.Println("Resetting checkpoint...")
		if err := checkpoint.Reset(); err != nil {
			return err
		}
	}

	// Initialize file writer
	writer, err := util.NewFileWriter(*outputDir)
	if err != nil {
		return fmt.Errorf("failed to initialize file writer: %w", err)
	}

	fmt.Printf("[3/4] Fetching conversations and user data...\n")

	// Fetch user map for name resolution
	userMap, err := client.BuildUserMap(ctx)
	if err != nil {
		fmt.Printf("  Warning: Could not fetch user list: %v\n", err)
		fmt.Println("  User mentions will show IDs instead of names")
		userMap = make(map[string]string)
	} else {
		fmt.Printf("  Loaded %d users\n", len(userMap))
	}

	// Fetch conversations
	convs, err := client.ListConversations(ctx, *exportTypes)
	if err != nil {
		return fmt.Errorf("failed to list conversations: %w", err)
	}
	fmt.Printf("  Found %d conversations to export\n", len(convs))

	// Create parser
	mdParser := parser.NewParser(userMap, nil)

	fmt.Println()
	fmt.Println("[4/4] Exporting messages...")
	fmt.Println()

	exported := 0
	skipped := 0

	for i, conv := range convs {
		select {
		case <-ctx.Done():
			fmt.Println("\nExport interrupted, saving checkpoint...")
			return checkpoint.Save()
		default:
		}

		// Check if already completed
		if *resume && checkpoint.IsConversationCompleted(conv.ID) {
			skipped++
			continue
		}

		// Get conversation name for display
		name := getConversationName(conv, userMap)
		fmt.Printf("  [%d/%d] %s...", i+1, len(convs), name)

		// Get checkpoint for resume
		var oldestTS string
		if *resume {
			if cp := checkpoint.GetConversationCheckpoint(conv.ID); cp != nil {
				oldestTS = cp.LastTimestamp
			}
		}

		// Fetch messages
		messages, lastTS, err := client.GetAllMessages(ctx, conv.ID, oldestTS)
		if err != nil {
			fmt.Printf(" ERROR: %v\n", err)
			// Save checkpoint and continue
			checkpoint.UpdateConversation(conv.ID, name, lastTS, len(messages), false)
			_ = checkpoint.Save()
			continue
		}

		if len(messages) == 0 {
			fmt.Println(" (no new messages)")
			checkpoint.UpdateConversation(conv.ID, name, lastTS, 0, true)
			continue
		}

		// Convert to Markdown
		markdown := mdParser.ConversationToMarkdown(&conv, messages)

		// Write to file
		filename := util.ConversationFilename(getConvType(conv), name, conv.ID)
		if err := writer.WriteConversation(filename, markdown); err != nil {
			fmt.Printf(" WRITE ERROR: %v\n", err)
			checkpoint.UpdateConversation(conv.ID, name, lastTS, len(messages), false)
			_ = checkpoint.Save()
			continue
		}

		fmt.Printf(" %d messages\n", len(messages))
		checkpoint.UpdateConversation(conv.ID, name, lastTS, len(messages), true)
		exported++

		// Save checkpoint periodically
		if exported%5 == 0 {
			_ = checkpoint.Save()
		}
	}

	// Final save
	if err := checkpoint.Save(); err != nil {
		fmt.Printf("Warning: Failed to save final checkpoint: %v\n", err)
	}

	fmt.Println()
	fmt.Println("Export complete!")
	fmt.Printf("  Exported: %d conversations\n", exported)
	fmt.Printf("  Skipped:  %d conversations (already done)\n", skipped)
	fmt.Printf("  Output:   %s\n", writer.GetOutputDir())

	return nil
}

func getConversationName(conv slackapi.Conversation, userMap map[string]string) string {
	if conv.IsIM {
		if name, ok := userMap[conv.User]; ok {
			return name
		}
		return conv.User
	}
	if conv.Name != "" {
		return conv.Name
	}
	return conv.ID
}

func getConvType(conv slackapi.Conversation) string {
	switch {
	case conv.IsIM:
		return "im"
	case conv.IsMPIM:
		return "mpim"
	case conv.IsPrivate:
		return "private_channel"
	default:
		return "channel"
	}
}
