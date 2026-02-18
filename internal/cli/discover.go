package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jflowers/get-out/pkg/chrome"
	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/models"
	"github.com/jflowers/get-out/pkg/slackapi"
	"github.com/spf13/cobra"
)

var (
	discoverMerge bool
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover people from configured Slack conversations",
	Long: `Discover fetches the member list for each conversation in conversations.json
and generates a people.json file with user details (name, email, display name).

By default, it merges with any existing people.json, skipping users already present.
Use --no-merge to overwrite the existing file.

Prerequisites:
  - Chrome/Chromium running with remote debugging enabled
  - An active Slack tab in the browser with an authenticated session
  - A valid conversations.json in the config directory

Examples:
  # Discover people and merge with existing people.json
  get-out discover

  # Discover and overwrite existing people.json
  get-out discover --no-merge`,
	RunE: runDiscover,
}

func init() {
	discoverCmd.Flags().BoolVar(&discoverMerge, "no-merge", false, "Overwrite existing people.json instead of merging")
	rootCmd.AddCommand(discoverCmd)
}

func runDiscover(cmd *cobra.Command, args []string) error {
	merge := !discoverMerge // --no-merge flag inverts the default

	fmt.Println("Discover People")
	fmt.Println("===============")
	fmt.Println()

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupt received, stopping...")
		cancel()
	}()

	// Load conversations config
	configPath := filepath.Join(configDir, "conversations.json")
	cfg, err := config.LoadConversations(configPath)
	if err != nil {
		return fmt.Errorf("failed to load conversations config: %w", err)
	}

	if len(cfg.Conversations) == 0 {
		return fmt.Errorf("no conversations configured in %s", configPath)
	}

	fmt.Printf("Found %d conversations in config\n", len(cfg.Conversations))

	// Load settings for optional bot token
	settingsPath := filepath.Join(configDir, "settings.json")
	settings, _ := config.LoadSettings(settingsPath) // Ignore error - settings are optional

	// Connect to Chrome and extract Slack credentials
	fmt.Println("Connecting to Chrome...")
	chromeCfg := chrome.DefaultConfig()
	chromeCfg.DebugPort = chromePort
	session, err := chrome.Connect(ctx, chromeCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to Chrome: %w", err)
	}
	defer session.Close()

	fmt.Println("Extracting Slack credentials...")
	creds, err := session.ExtractCredentials(ctx)
	if err != nil {
		return fmt.Errorf("failed to extract credentials: %w", err)
	}
	fmt.Printf("Found Slack team: %s\n\n", creds.TeamDomain)

	// Create Slack client
	var client *slackapi.Client
	if settings != nil && settings.SlackBotToken != "" {
		fmt.Println("Using API mode (bot token)")
		client = slackapi.NewAPIClient(settings.SlackBotToken)
	} else {
		fmt.Println("Using browser mode (xoxc token)")
		client = slackapi.NewBrowserClient(creds.Token, creds.Cookie)
	}

	// Collect unique member IDs across channel conversations only.
	// DMs and MPDMs are not accessible via bot token for member listing.
	fmt.Println()
	memberSet := make(map[string]bool)
	skippedConvs := 0
	for _, conv := range cfg.Conversations {
		// Only fetch members from channels — bot tokens can't list DM/MPIM members
		if conv.Type != models.ConversationTypeChannel && conv.Type != models.ConversationTypePrivateChannel {
			skippedConvs++
			continue
		}

		fmt.Printf("  Fetching members for %s (%s)...", conv.Name, conv.ID)

		count := 0
		cursor := ""
		for {
			resp, err := client.GetConversationMembers(ctx, conv.ID, cursor)
			if err != nil {
				fmt.Printf(" error: %v\n", err)
				break
			}

			for _, memberID := range resp.Members {
				if !memberSet[memberID] {
					memberSet[memberID] = true
					count++
				}
			}

			if resp.ResponseMetadata.NextCursor == "" {
				break
			}
			cursor = resp.ResponseMetadata.NextCursor

			// Throttle
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}
		fmt.Printf(" %d members\n", count)

		// Throttle between conversations
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	fmt.Printf("\nFound %d unique members across all channels", len(memberSet))
	if skippedConvs > 0 {
		fmt.Printf(" (skipped %d DMs/MPDMs)", skippedConvs)
	}
	fmt.Println()

	// Load existing people.json if merging
	peoplePath := filepath.Join(configDir, "people.json")
	var existingPeople map[string]config.PersonConfig
	if merge {
		existingPeople = make(map[string]config.PersonConfig)
		existing, err := config.LoadPeople(peoplePath)
		if err == nil {
			for _, p := range existing.People {
				existingPeople[p.SlackID] = p
			}
			if len(existingPeople) > 0 {
				fmt.Printf("Loaded %d existing people (will merge)\n", len(existingPeople))
			}
		}
	}

	// Fetch user info for each member
	fmt.Println("\nFetching user profiles...")
	var people []config.PersonConfig
	fetched := 0
	skipped := 0

	for memberID := range memberSet {
		// Skip if already in existing people.json
		if merge {
			if _, exists := existingPeople[memberID]; exists {
				skipped++
				continue
			}
		}

		user, err := client.GetUserInfo(ctx, memberID)
		if err != nil {
			if verbose {
				fmt.Printf("  Warning: could not fetch user %s: %v\n", memberID, err)
			}
			continue
		}

		// Skip bots and app users
		if user.IsBot || user.IsAppUser {
			continue
		}

		// Skip deleted users
		if user.Deleted {
			continue
		}

		person := config.PersonConfig{
			SlackID:     user.ID,
			Email:       user.Profile.Email,
			DisplayName: user.GetDisplayName(),
		}
		people = append(people, person)
		fetched++

		if fetched%25 == 0 {
			fmt.Printf("  Fetched %d user profiles...\n", fetched)
		}

		// Throttle
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}

	fmt.Printf("  Fetched %d new user profiles", fetched)
	if skipped > 0 {
		fmt.Printf(" (skipped %d already in people.json)", skipped)
	}
	fmt.Println()

	// Merge with existing if needed
	if merge && len(existingPeople) > 0 {
		for _, p := range existingPeople {
			people = append(people, p)
		}
	}

	// Write people.json
	peopleCfg := config.PeopleConfig{People: people}
	data, err := json.MarshalIndent(peopleCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal people config: %w", err)
	}

	if err := os.WriteFile(peoplePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write people.json: %w", err)
	}

	fmt.Printf("\nWrote %d people to %s\n", len(people), peoplePath)

	return nil
}
