package cli

import (
	"fmt"
	"io"
	"os"
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
	configPath := filepath.Join(configDir, "conversations.json")
	cfg, err := config.LoadConversations(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	listCore(os.Stdout, cfg.FilterByExport(), listType, listMode)
	return nil
}

// listCore formats and writes the conversation list to w.
// It filters by typeFilter and modeFilter (empty string means no filter),
// groups results by conversation type, and writes formatted output.
func listCore(w io.Writer, conversations []config.ConversationConfig, typeFilter, modeFilter string) {
	// Apply filters
	if typeFilter != "" {
		var filtered []config.ConversationConfig
		for _, c := range conversations {
			if string(c.Type) == typeFilter {
				filtered = append(filtered, c)
			}
		}
		conversations = filtered
	}

	if modeFilter != "" {
		var filtered []config.ConversationConfig
		for _, c := range conversations {
			if string(c.Mode) == modeFilter {
				filtered = append(filtered, c)
			}
		}
		conversations = filtered
	}

	// Display results
	fmt.Fprintf(w, "Configured conversations (%d total):\n\n", len(conversations))

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

		fmt.Fprintf(w, "%s (%d):\n", typeNames[t], len(convs))
		for _, c := range convs {
			modeStr := "API"
			if c.Mode == models.ExportModeBrowser {
				modeStr = "Browser"
			}
			shareStr := ""
			if c.Share {
				shareStr = " [share]"
			}
			fmt.Fprintf(w, "  %-12s %-30s [%s]%s\n", c.ID, c.Name, modeStr, shareStr)
		}
		fmt.Fprintln(w)
	}
}
