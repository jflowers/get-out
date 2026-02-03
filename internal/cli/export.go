package cli

import (
	"fmt"
	"path/filepath"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/spf13/cobra"
)

var (
	exportFolder string
	exportDryRun bool
	exportResume bool
)

var exportCmd = &cobra.Command{
	Use:   "export [conversation_id...]",
	Short: "Export Slack messages to Google Docs",
	Long: `Export Slack messages to Google Docs.

If no conversation IDs are provided, exports all conversations 
configured in conversations.json where export=true.

Examples:
  # Export all configured conversations
  get-out export

  # Export specific conversations
  get-out export D123ABC456 C789DEF012

  # Resume a previous export
  get-out export --resume

  # Dry run to see what would be exported
  get-out export --dry-run`,
	RunE: runExport,
}

func init() {
	exportCmd.Flags().StringVar(&exportFolder, "folder", "Slack Exports", "Google Drive root folder name")
	exportCmd.Flags().BoolVar(&exportDryRun, "dry-run", false, "Show what would be exported without actually exporting")
	exportCmd.Flags().BoolVar(&exportResume, "resume", false, "Resume from last checkpoint")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	fmt.Println("Slack Message Export")
	fmt.Println("====================")
	fmt.Println()

	// Load conversations config
	configPath := filepath.Join(configDir, "conversations.json")
	cfg, err := config.LoadConversations(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Load people config (optional)
	peoplePath := filepath.Join(configDir, "people.json")
	people, err := config.LoadPeople(peoplePath)
	if err != nil {
		fmt.Printf("Warning: Could not load people.json: %v\n", err)
		people = &config.PeopleConfig{}
	}

	// Determine which conversations to export
	var toExport []config.ConversationConfig
	if len(args) > 0 {
		// Export specific conversations
		for _, id := range args {
			conv := cfg.GetByID(id)
			if conv == nil {
				return fmt.Errorf("conversation not found in config: %s", id)
			}
			toExport = append(toExport, *conv)
		}
	} else {
		// Export all with export=true
		toExport = cfg.FilterByExport()
	}

	if len(toExport) == 0 {
		fmt.Println("No conversations to export.")
		return nil
	}

	fmt.Printf("Found %d conversations to export\n", len(toExport))
	fmt.Printf("People mapping: %d entries\n", len(people.People))
	fmt.Printf("Drive folder: %s\n", exportFolder)
	fmt.Println()

	if exportDryRun {
		fmt.Println("DRY RUN - Would export:")
		for _, c := range toExport {
			fmt.Printf("  - %s (%s) [%s mode]\n", c.Name, c.ID, c.Mode)
		}
		return nil
	}

	// TODO: Implement actual export
	// 1. Authenticate with Google (check token.json)
	// 2. For browser mode conversations:
	//    a. Connect to Chrome via CDP
	//    b. Extract xoxc token
	//    c. Fetch messages via Slack API
	// 3. For API mode conversations:
	//    a. Use xoxb bot token from env/config
	//    b. Fetch messages via Slack API
	// 4. Create folder structure in Google Drive
	// 5. Convert messages to Google Docs format
	// 6. Write daily docs with proper formatting
	// 7. Handle threads in subfolders
	// 8. Update export index for link resolution

	fmt.Println("Export not yet implemented.")
	fmt.Println()
	fmt.Println("Conversations to export:")
	for _, c := range toExport {
		fmt.Printf("  - %s (%s) [%s mode]\n", c.Name, c.ID, c.Mode)
	}

	return nil
}
