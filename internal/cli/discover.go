package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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
	client := slackapi.NewBrowserClient(creds.Token, creds.Cookie)

	// Create spinner for interactive mode
	var spin *StatusSpinner
	if isTerminal() {
		spin = NewStatusSpinner()
	}

	// Phase 1: Collect channel members
	progress := makeSpinnerProgress(spin)
	fmt.Println()
	if spin != nil {
		spin.Start()
	}

	memberSet, skippedConvs, err := collectChannelMembers(ctx, client, cfg.Conversations, progress)
	if spin != nil {
		spin.Stop()
	}
	if err != nil {
		return err
	}

	fmt.Printf("Found %d unique members across all channels", len(memberSet))
	if skippedConvs > 0 {
		fmt.Printf(" (skipped %d DMs/MPDMs)", skippedConvs)
	}
	fmt.Println()

	// Load existing people.json if merging
	peoplePath := filepath.Join(configDir, "people.json")
	existingPeople := loadExistingPeople(peoplePath, merge)

	// Phase 2: Fetch user profiles
	fmt.Println("\nFetching user profiles...")
	if spin != nil {
		spin.Start()
	}

	fetchedUsers, skipped, err := fetchUserProfiles(ctx, client, memberSet, existingPeople, progress)
	if spin != nil {
		spin.Stop()
	}
	if err != nil {
		return err
	}

	fmt.Printf("  Fetched %d new user profiles", len(fetchedUsers))
	if skipped > 0 {
		fmt.Printf(" (skipped %d already in people.json)", skipped)
	}
	fmt.Println()

	// Phase 3: Write people.json
	count, err := writePeopleJSON(peoplePath, fetchedUsers, existingPeople, merge)
	if err != nil {
		return err
	}
	fmt.Printf("\nWrote %d people to %s\n", count, peoplePath)

	return nil
}

// ---------------------------------------------------------------------------
// Extracted functions
// ---------------------------------------------------------------------------

// makeSpinnerProgress returns a progress callback that updates the spinner if
// available, or is a no-op otherwise.
func makeSpinnerProgress(spin *StatusSpinner) func(string) {
	if spin != nil {
		return func(msg string) { spin.Update(msg) }
	}
	return func(msg string) {}
}

// collectChannelMembers fetches unique member IDs from channel conversations.
// It skips DM and MPIM conversations (which don't support member listing).
func collectChannelMembers(
	ctx context.Context,
	client *slackapi.Client,
	conversations []config.ConversationConfig,
	progress func(string),
) (memberSet map[string]bool, skippedConvs int, err error) {
	memberSet = make(map[string]bool)

	for _, conv := range conversations {
		if conv.Type != models.ConversationTypeChannel && conv.Type != models.ConversationTypePrivateChannel {
			skippedConvs++
			continue
		}

		progress(fmt.Sprintf("Fetching members for %s (%s)...", conv.Name, conv.ID))

		cursor := ""
		for {
			resp, err := client.GetConversationMembers(ctx, conv.ID, cursor)
			if err != nil {
				break // skip this conversation on error
			}

			for _, memberID := range resp.Members {
				memberSet[memberID] = true
			}

			if resp.ResponseMetadata.NextCursor == "" {
				break
			}
			cursor = resp.ResponseMetadata.NextCursor

			if ctx.Err() != nil {
				return memberSet, skippedConvs, ctx.Err()
			}
		}

		if ctx.Err() != nil {
			return memberSet, skippedConvs, ctx.Err()
		}
	}

	return memberSet, skippedConvs, nil
}

// fetchUserProfiles fetches user profiles for the given member IDs, skipping
// any IDs present in the skip map. Returns fetched users and skip count.
func fetchUserProfiles(
	ctx context.Context,
	client *slackapi.Client,
	memberIDs map[string]bool,
	skip map[string]config.PersonConfig,
	progress func(string),
) (users []*slackapi.User, skipped int, err error) {
	fetched := 0
	total := len(memberIDs)

	for memberID := range memberIDs {
		if _, exists := skip[memberID]; exists {
			skipped++
			continue
		}

		user, err := client.GetUserInfo(ctx, memberID)
		if err != nil {
			continue // skip individual user errors
		}

		users = append(users, user)
		fetched++

		progress(fmt.Sprintf("Fetching user profiles... %d/%d", fetched, total))

		if ctx.Err() != nil {
			return users, skipped, ctx.Err()
		}
	}

	return users, skipped, nil
}

// loadExistingPeople loads existing people.json into a map for merge lookups.
// Returns an empty map if merge is false or the file doesn't exist.
func loadExistingPeople(peoplePath string, merge bool) map[string]config.PersonConfig {
	result := make(map[string]config.PersonConfig)
	if !merge {
		return result
	}

	existing, err := config.LoadPeople(peoplePath)
	if err != nil {
		return result
	}

	for _, p := range existing.People {
		result[p.SlackID] = p
	}
	if len(result) > 0 {
		fmt.Printf("Loaded %d existing people (will merge)\n", len(result))
	}
	return result
}

// writePeopleJSON converts fetched users to PersonConfig, merges with existing
// people if needed, and writes the result to disk.
func writePeopleJSON(path string, fetchedUsers []*slackapi.User, existingPeople map[string]config.PersonConfig, merge bool) (int, error) {
	newPeople := buildPeopleFromUsers(fetchedUsers)

	var existingList []config.PersonConfig
	if merge {
		for _, p := range existingPeople {
			existingList = append(existingList, p)
		}
	}
	people := mergePeople(newPeople, existingList)

	peopleCfg := config.PeopleConfig{People: people}
	data, err := json.MarshalIndent(peopleCfg, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("failed to marshal people config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return 0, fmt.Errorf("failed to write people.json: %w", err)
	}

	return len(people), nil
}

// buildPeopleFromUsers converts Slack user info to PersonConfig entries,
// filtering out bots, app users, and deleted users.
func buildPeopleFromUsers(users []*slackapi.User) []config.PersonConfig {
	var people []config.PersonConfig
	for _, user := range users {
		if user.IsBot || user.IsAppUser || user.Deleted {
			continue
		}
		people = append(people, config.PersonConfig{
			SlackID:     user.ID,
			Email:       user.Profile.Email,
			DisplayName: user.GetDisplayName(),
		})
	}
	return people
}

// mergePeople merges newPeople with existingPeople. Existing entries are kept
// as-is (not overwritten).
func mergePeople(newPeople, existingPeople []config.PersonConfig) []config.PersonConfig {
	existing := make(map[string]config.PersonConfig)
	for _, p := range existingPeople {
		existing[p.SlackID] = p
	}
	var result []config.PersonConfig
	for _, p := range existingPeople {
		result = append(result, p)
	}
	for _, p := range newPeople {
		if _, exists := existing[p.SlackID]; !exists {
			result = append(result, p)
		}
	}
	return result
}
