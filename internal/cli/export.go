package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/exporter"
	"github.com/jflowers/get-out/pkg/gdrive"
	"github.com/spf13/cobra"
)

var (
	exportFolder   string
	exportFolderID string
	exportDryRun   bool
	exportResume   bool
)

var exportCmd = &cobra.Command{
	Use:   "export [conversation_id...]",
	Short: "Export Slack messages to Google Docs",
	Long: `Export Slack messages to Google Docs.

If no conversation IDs are provided, exports all conversations 
configured in conversations.json where export=true.

Prerequisites:
  - Chrome/Chromium running with remote debugging enabled on the configured port
  - An active Slack tab in the browser with an authenticated session
  - Google OAuth completed (run 'get-out auth' first)

Examples:
  # Export all configured conversations
  get-out export

  # Export specific conversations
  get-out export D123ABC456 C789DEF012

  # Resume a previous export
  get-out export --resume

  # Dry run to see what would be exported
  get-out export --dry-run

  # Use custom Chrome port
  get-out export --chrome-port 9223

  # Export to an existing Google Drive folder by ID
  get-out export --folder-id 1ABC123xyz`,
	RunE: runExport,
}

func init() {
	exportCmd.Flags().StringVar(&exportFolder, "folder", "Slack Exports", "Google Drive root folder name (ignored if --folder-id is set)")
	exportCmd.Flags().StringVar(&exportFolderID, "folder-id", "", "Google Drive folder ID to export into (uses existing folder)")
	exportCmd.Flags().BoolVar(&exportDryRun, "dry-run", false, "Show what would be exported without actually exporting")
	exportCmd.Flags().BoolVar(&exportResume, "resume", false, "Resume from last checkpoint")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	fmt.Println("Slack Message Export")
	fmt.Println("====================")
	fmt.Println()

	// Load settings
	settingsPath := filepath.Join(configDir, "settings.json")
	settings, err := config.LoadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	// Apply settings as defaults (CLI flags override)
	if exportFolderID == "" && settings.GoogleDriveFolderID != "" {
		exportFolderID = settings.GoogleDriveFolderID
	}

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
		if debugMode {
			fmt.Printf("Note: Could not load people.json: %v\n", err)
		}
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
		fmt.Println()
		fmt.Println("Make sure you have conversations configured in:")
		fmt.Printf("  %s\n", configPath)
		fmt.Println()
		fmt.Println("And that at least one has \"export\": true")
		return nil
	}

	fmt.Printf("Found %d conversations to export\n", len(toExport))
	if len(people.People) > 0 {
		fmt.Printf("People mapping: %d entries\n", len(people.People))
	}
	if exportFolderID != "" {
		fmt.Printf("Drive folder ID: %s\n", exportFolderID)
	} else {
		fmt.Printf("Drive folder: %s\n", exportFolder)
	}
	fmt.Printf("Chrome port: %d\n", chromePort)
	fmt.Println()

	// Dry run mode - just show what would be exported
	if exportDryRun {
		fmt.Println("DRY RUN - Would export:")
		fmt.Println()
		for _, c := range toExport {
			fmt.Printf("  - %s (%s)\n", c.Name, c.ID)
			fmt.Printf("    Type: %s, Mode: %s\n", c.Type, c.Mode)
			if c.Share {
				fmt.Printf("    Sharing: enabled")
				if len(c.ShareMembers) > 0 {
					fmt.Printf(" with %d members", len(c.ShareMembers))
				}
				fmt.Println()
			}
		}
		return nil
	}

	// Check prerequisites
	gdriveCfg := gdrive.DefaultConfig(configDir)
	if !gdrive.HasCredentials(gdriveCfg) {
		return fmt.Errorf("Google credentials not found at %s\n\nDownload credentials.json from Google Cloud Console", gdriveCfg.CredentialsPath)
	}

	if !gdrive.HasToken(gdriveCfg) {
		fmt.Println("Google authorization required. Run 'get-out auth' first.")
		return fmt.Errorf("no Google token found")
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupt received, saving progress...")
		cancel()
	}()

	// Create exporter
	exp := exporter.NewExporter(&exporter.ExporterConfig{
		ConfigDir:             configDir,
		RootFolderName:        exportFolder,
		RootFolderID:          exportFolderID,
		ChromePort:            chromePort,
		Debug:                 debugMode,
		GoogleCredentialsFile: settings.GoogleCredentialsFile,
		SlackBotToken:         settings.SlackBotToken,
		OnProgress: func(msg string) {
			if verbose || debugMode {
				fmt.Printf("  %s\n", msg)
			}
		},
	})

	// Initialize connections
	fmt.Println("Initializing...")
	if err := exp.Initialize(ctx, chromePort); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}
	fmt.Println()

	// Run export
	fmt.Println("Starting export...")
	fmt.Println()

	results, err := exp.ExportAll(ctx, toExport)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Print summary
	fmt.Println()
	fmt.Println("Export Summary")
	fmt.Println("==============")
	fmt.Println()

	totalMessages := 0
	totalDocs := 0
	totalThreads := 0
	errorCount := 0

	for _, r := range results {
		status := "OK"
		if r.Error != nil {
			status = "FAILED"
			errorCount++
		}

		fmt.Printf("%-30s %6d msgs  %3d docs  %3d threads  [%s]\n",
			truncateName(r.Name, 30),
			r.MessageCount,
			r.DocsCreated,
			r.ThreadsExported,
			status)

		if r.Error != nil && (verbose || debugMode) {
			fmt.Printf("  Error: %v\n", r.Error)
		}

		totalMessages += r.MessageCount
		totalDocs += r.DocsCreated
		totalThreads += r.ThreadsExported
	}

	fmt.Println()
	fmt.Printf("Total: %d messages, %d docs, %d threads\n", totalMessages, totalDocs, totalThreads)

	if errorCount > 0 {
		fmt.Printf("Errors: %d conversation(s) failed\n", errorCount)
	}

	// Print folder URL
	rootURL := exp.GetRootFolderURL()
	if rootURL != "" {
		fmt.Println()
		fmt.Println("Export folder:")
		fmt.Printf("  %s\n", rootURL)
	}

	if errorCount > 0 {
		return fmt.Errorf("%d export(s) failed", errorCount)
	}

	return nil
}

// truncateName shortens a name for display.
func truncateName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	if maxLen <= 3 {
		return name[:maxLen]
	}
	return name[:maxLen-3] + "..."
}
