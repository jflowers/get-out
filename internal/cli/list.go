package cli

import (
	"fmt"
	"path/filepath"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/models"
	"github.com/spf13/cobra"
)

var (
	listType string
	listMode string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured conversations",
	Long: `List all conversations configured for export in conversations.json.

Optionally filter by type (dm, mpim, channel, private_channel) or
mode (api, browser).`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVar(&listType, "type", "", "Filter by type: dm, mpim, channel, private_channel")
	listCmd.Flags().StringVar(&listMode, "mode", "", "Filter by mode: api, browser")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	// Load conversations config
	configPath := filepath.Join(configDir, "conversations.json")
	cfg, err := config.LoadConversations(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply filters
	conversations := cfg.FilterByExport()

	if listType != "" {
		var filtered []config.ConversationConfig
		for _, c := range conversations {
			if string(c.Type) == listType {
				filtered = append(filtered, c)
			}
		}
		conversations = filtered
	}

	if listMode != "" {
		var filtered []config.ConversationConfig
		for _, c := range conversations {
			if string(c.Mode) == listMode {
				filtered = append(filtered, c)
			}
		}
		conversations = filtered
	}

	// Display results
	fmt.Printf("Configured conversations (%d total):\n\n", len(conversations))

	// Group by type
	byType := make(map[models.ConversationType][]config.ConversationConfig)
	for _, c := range conversations {
		byType[c.Type] = append(byType[c.Type], c)
	}

	typeOrder := []models.ConversationType{
		models.ConversationTypeDM,
		models.ConversationTypeMPIM,
		models.ConversationTypeChannel,
		models.ConversationTypePrivateChannel,
	}

	typeNames := map[models.ConversationType]string{
		models.ConversationTypeDM:             "Direct Messages",
		models.ConversationTypeMPIM:           "Group Messages",
		models.ConversationTypeChannel:        "Channels",
		models.ConversationTypePrivateChannel: "Private Channels",
	}

	for _, t := range typeOrder {
		convs := byType[t]
		if len(convs) == 0 {
			continue
		}

		fmt.Printf("%s (%d):\n", typeNames[t], len(convs))
		for _, c := range convs {
			modeStr := "API"
			if c.Mode == models.ExportModeBrowser {
				modeStr = "Browser"
			}
			shareStr := ""
			if c.Share {
				shareStr = " [share]"
			}
			fmt.Printf("  %-12s %-30s [%s]%s\n", c.ID, c.Name, modeStr, shareStr)
		}
		fmt.Println()
	}

	return nil
}
