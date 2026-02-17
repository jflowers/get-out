package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/exporter"
	"github.com/jflowers/get-out/pkg/gdrive"
	"github.com/jflowers/get-out/pkg/models"
	"github.com/spf13/cobra"
)

var (
	exportFolder      string
	exportFolderID    string
	exportDryRun      bool
	exportResume      bool
	exportFrom        string
	exportTo          string
	exportSync        bool
	exportUserMapping string
	exportAllDMs      bool
	exportAllGroups   bool
	exportParallel    int
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
  get-out export --folder-id 1ABC123xyz

  # Export only messages from a date range
  get-out export --from 2025-01-01 --to 2025-06-30

  # Incremental sync - only new messages since last export
  get-out export --sync

  # Export all DMs or group messages
  get-out export --all-dms
  get-out export --all-groups

  # Export in parallel (max 5 concurrent)
  get-out export --parallel 5`,
	RunE: runExport,
}

func init() {
	exportCmd.Flags().StringVar(&exportFolder, "folder", "Slack Exports", "Google Drive root folder name (ignored if --folder-id is set)")
	exportCmd.Flags().StringVar(&exportFolderID, "folder-id", "", "Google Drive folder ID to export into (uses existing folder)")
	exportCmd.Flags().BoolVar(&exportDryRun, "dry-run", false, "Show what would be exported without actually exporting")
	exportCmd.Flags().BoolVar(&exportResume, "resume", false, "Resume from last checkpoint")
	exportCmd.Flags().StringVar(&exportFrom, "from", "", "Export messages from this date (YYYY-MM-DD)")
	exportCmd.Flags().StringVar(&exportTo, "to", "", "Export messages up to this date (YYYY-MM-DD)")
	exportCmd.Flags().BoolVar(&exportSync, "sync", false, "Only export messages since last successful export")
	exportCmd.Flags().StringVar(&exportUserMapping, "user-mapping", "", "Path to people.json for @mention linking (default: <config-dir>/people.json)")
	exportCmd.Flags().BoolVar(&exportAllDMs, "all-dms", false, "Export all DM conversations")
	exportCmd.Flags().BoolVar(&exportAllGroups, "all-groups", false, "Export all group (MPIM) conversations")
	exportCmd.Flags().IntVar(&exportParallel, "parallel", 1, "Number of conversations to export concurrently (max 5)")
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
	if exportUserMapping != "" {
		peoplePath = exportUserMapping
	}
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
	} else if exportAllDMs {
		// Export all DM conversations
		toExport = cfg.FilterByType(models.ConversationTypeDM)
	} else if exportAllGroups {
		// Export all group (MPIM) conversations
		toExport = cfg.FilterByType(models.ConversationTypeMPIM)
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
	if settings.GoogleCredentialsFile != "" {
		gdriveCfg.CredentialsPath = settings.GoogleCredentialsFile
		// Derive token path from the same directory as credentials
		gdriveCfg.TokenPath = filepath.Join(filepath.Dir(settings.GoogleCredentialsFile), "token.json")
	}
	if !gdrive.HasCredentials(gdriveCfg) {
		return fmt.Errorf("Google credentials not found at %s\n\nDownload credentials.json from Google Cloud Console", gdriveCfg.CredentialsPath)
	}

	if !gdrive.HasToken(gdriveCfg) {
		fmt.Println("Google authorization required. Run 'get-out auth' first.")
		return fmt.Errorf("no Google token found at %s", gdriveCfg.TokenPath)
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
	// Validate flag combinations
	if exportSync && (exportFrom != "" || exportTo != "") {
		return fmt.Errorf("--sync cannot be combined with --from or --to")
	}
	if exportResume && (exportFrom != "" || exportTo != "") {
		return fmt.Errorf("--resume cannot be combined with --from or --to")
	}

	// Parse date range flags into Slack timestamps
	dateFrom, err := parseDateFlag(exportFrom)
	if err != nil {
		return fmt.Errorf("invalid --from date: %w", err)
	}
	dateTo, err := parseDateFlag(exportTo)
	if err != nil {
		return fmt.Errorf("invalid --to date: %w", err)
	}
	// --to should be end of day (23:59:59)
	if dateTo != "" {
		ts, _ := strconv.ParseInt(dateTo[:len(dateTo)-7], 10, 64) // strip .000000
		ts += 86400 - 1                                           // end of day
		dateTo = fmt.Sprintf("%d.000000", ts)
	}

	exp := exporter.NewExporter(&exporter.ExporterConfig{
		ConfigDir:             configDir,
		RootFolderName:        exportFolder,
		RootFolderID:          exportFolderID,
		ChromePort:            chromePort,
		Debug:                 debugMode,
		GoogleCredentialsFile: settings.GoogleCredentialsFile,
		SlackBotToken:         settings.SlackBotToken,
		DateFrom:              dateFrom,
		DateTo:                dateTo,
		SyncMode:              exportSync,
		ResumeMode:            exportResume,
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

	results, err := exp.ExportAllParallel(ctx, toExport, exportParallel)
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

// parseDateFlag converts a YYYY-MM-DD date string to a Slack timestamp.
// Returns empty string if input is empty.
func parseDateFlag(dateStr string) (string, error) {
	if dateStr == "" {
		return "", nil
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return "", fmt.Errorf("expected YYYY-MM-DD format, got %q", dateStr)
	}
	return fmt.Sprintf("%d.000000", t.Unix()), nil
}
