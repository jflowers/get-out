package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/exporter"
	"github.com/jflowers/get-out/pkg/models"
	"github.com/jflowers/get-out/pkg/ollama"
	"github.com/jflowers/get-out/pkg/secrets"
	"github.com/spf13/cobra"
)

var (
	exportFolder               string
	exportFolderID             string
	exportDryRun               bool
	exportResume               bool
	exportFrom                 string
	exportTo                   string
	exportSync                 bool
	exportUserMapping          string
	exportAllDMs               bool
	exportAllGroups            bool
	exportParallel             int
	exportLocalExportDir       string
	exportNoSensitivityFilter  bool
	exportOllamaEndpoint       string
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
	exportCmd.Flags().StringVar(&exportLocalExportDir, "local-export-dir", "", "Directory for local markdown export (overrides settings)")
	exportCmd.Flags().BoolVar(&exportNoSensitivityFilter, "no-sensitivity-filter", false, "Disable sensitivity filtering for this run")
	exportCmd.Flags().StringVar(&exportOllamaEndpoint, "ollama-endpoint", "", "Override Ollama endpoint URL")
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

	// Resolve folder ID from flags and settings
	exportFolderID = resolveExportFolderID(exportFolderID, settings)

	// Resolve local export directory from flag and settings
	localExportDir := resolveLocalExportDir(exportLocalExportDir, settings)
	if localExportDir != "" {
		var pathErr error
		localExportDir, pathErr = exporter.ExpandAndValidatePath(localExportDir)
		if pathErr != nil {
			return fmt.Errorf("invalid local export directory: %w", pathErr)
		}
	}

	// Sensitivity filter initialization: validate Ollama prerequisites and
	// create the MessageFilter before building ExporterConfig (US2: fail fast).
	var messageFilter exporter.MessageFilter
	if settings.Ollama != nil && settings.Ollama.Enabled && !exportNoSensitivityFilter {
		ollamaEndpoint := resolveOllamaEndpoint(exportOllamaEndpoint, settings)
		ollamaModel := settings.Ollama.Model
		if ollamaModel == "" {
			ollamaModel = config.DefaultOllamaModel
		}

		ollamaClient := ollama.NewClient(ollamaEndpoint, ollamaModel)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := validateOllamaPrerequisites(ctx, ollamaClient); err != nil {
			cancel()
			return err
		}
		cancel()

		guardian := ollama.NewGuardian(ollamaClient)
		messageFilter = exporter.NewOllamaFilter(guardian)
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
	toExport, err := selectConversations(cfg, args, exportAllDMs, exportAllGroups)
	if err != nil {
		return err
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
	if localExportDir != "" {
		fmt.Printf("Local export: %s\n", localExportDir)
	}
	fmt.Println()

	// Dry run mode - just show what would be exported
	if exportDryRun {
		formatExportDryRun(os.Stdout, toExport)
		if localExportDir != "" {
			formatLocalExportDryRun(os.Stdout, toExport, localExportDir)
		}
		return nil
	}

	// Check prerequisites
	if err := checkExportPrerequisites(settings, secretStore); err != nil {
		return err
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create spinner for non-verbose interactive mode
	var spin *StatusSpinner
	if !verbose && !debugMode && isTerminal() {
		spin = NewStatusSpinner()
	}

	// Handle interrupt gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if spin != nil {
			spin.Stop()
		}
		fmt.Println("\nInterrupt received, saving progress...")
		cancel()
	}()

	// Validate flag combinations
	if err := validateExportFlags(exportSync, exportResume, exportFrom, exportTo); err != nil {
		return err
	}

	// Parse date range flags into Slack timestamps
	dateFrom, dateTo, err := parseDateRange(exportFrom, exportTo)
	if err != nil {
		return err
	}

	exp := exporter.NewExporter(&exporter.ExporterConfig{
		ConfigDir:             configDir,
		RootFolderName:        exportFolder,
		RootFolderID:          exportFolderID,
		ChromePort:            chromePort,
		Debug:                 debugMode,
		GoogleCredentialsFile: settings.GoogleCredentialsFile,
		DateFrom:              dateFrom,
		DateTo:                dateTo,
		SyncMode:              exportSync,
		ResumeMode:            exportResume,
		LocalExportDir:        localExportDir,
		MessageFilter:         messageFilter,
		OnProgress: func(msg string) {
			if verbose || debugMode {
				fmt.Printf("  %s\n", msg)
			} else if spin != nil {
				spin.Update(msg)
			}
		},
	})

	// Initialize connections using the active SecretStore (keychain or file).
	fmt.Println("Initializing...")
	if err := exp.InitializeWithStore(ctx, chromePort, secretStore); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}
	fmt.Println()

	// Run export
	fmt.Println("Starting export...")
	fmt.Println()

	if spin != nil {
		spin.Start()
	}

	results, err := exp.ExportAllParallel(ctx, toExport, exportParallel)

	if spin != nil {
		spin.Stop()
	}

	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Print summary
	return printExportResults(os.Stdout, results, exp.GetRootFolderURL(), verbose || debugMode)
}

// selectConversations determines which conversations to export based on
// args, flags (allDMs, allGroups), and the config's export field.
func selectConversations(cfg *config.ConversationsConfig, args []string, allDMs, allGroups bool) ([]config.ConversationConfig, error) {
	if len(args) > 0 {
		var result []config.ConversationConfig
		for _, id := range args {
			conv := cfg.GetByID(id)
			if conv == nil {
				return nil, fmt.Errorf("conversation not found in config: %s", id)
			}
			result = append(result, *conv)
		}
		return result, nil
	}
	if allDMs {
		return cfg.FilterByType(models.ConversationTypeDM), nil
	}
	if allGroups {
		return cfg.FilterByType(models.ConversationTypeMPIM), nil
	}
	return cfg.FilterByExport(), nil
}

// validateExportFlags checks for invalid flag combinations.
func validateExportFlags(syncMode, resumeMode bool, dateFrom, dateTo string) error {
	if syncMode && (dateFrom != "" || dateTo != "") {
		return fmt.Errorf("--sync cannot be combined with --from or --to")
	}
	if resumeMode && (dateFrom != "" || dateTo != "") {
		return fmt.Errorf("--resume cannot be combined with --from or --to")
	}
	return nil
}

// formatExportDryRun writes the dry-run output showing what would be exported.
func formatExportDryRun(w io.Writer, conversations []config.ConversationConfig) {
	fmt.Fprintln(w, "DRY RUN - Would export:")
	fmt.Fprintln(w)
	for _, c := range conversations {
		fmt.Fprintf(w, "  - %s (%s)\n", c.Name, c.ID)
		fmt.Fprintf(w, "    Type: %s\n", c.Type)
		if c.Share {
			fmt.Fprintf(w, "    Sharing: enabled")
			if len(c.ShareMembers) > 0 {
				fmt.Fprintf(w, " with %d members", len(c.ShareMembers))
			}
			fmt.Fprintln(w)
		}
	}
}

// ExportResultSummary holds the display fields from an export result.
type ExportResultSummary struct {
	Name            string
	MessageCount    int
	DocsCreated     int
	ThreadsExported int
	Error           error
}

// formatExportSummary writes the export results summary and returns the error count.
func formatExportSummary(w io.Writer, results []ExportResultSummary, rootURL string, showErrors bool) int {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Export Summary")
	fmt.Fprintln(w, "==============")
	fmt.Fprintln(w)

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

		fmt.Fprintf(w, "%-30s %6d msgs  %3d docs  %3d threads  [%s]\n",
			truncateName(r.Name, 30),
			r.MessageCount,
			r.DocsCreated,
			r.ThreadsExported,
			status)

		if r.Error != nil && showErrors {
			fmt.Fprintf(w, "  Error: %v\n", r.Error)
		}

		totalMessages += r.MessageCount
		totalDocs += r.DocsCreated
		totalThreads += r.ThreadsExported
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Total: %d messages, %d docs, %d threads\n", totalMessages, totalDocs, totalThreads)

	if errorCount > 0 {
		fmt.Fprintf(w, "Errors: %d conversation(s) failed\n", errorCount)
	}

	if rootURL != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Export folder:")
		fmt.Fprintf(w, "  %s\n", rootURL)
	}

	return errorCount
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

// printExportResults converts export results to summaries and prints the table.
func printExportResults(w io.Writer, results []*exporter.ExportResult, rootURL string, showErrors bool) error {
	summaries := make([]ExportResultSummary, len(results))
	for i, r := range results {
		summaries[i] = ExportResultSummary{
			Name:            r.Name,
			MessageCount:    r.MessageCount,
			DocsCreated:     r.DocsCreated,
			ThreadsExported: r.ThreadsExported,
			Error:           r.Error,
		}
	}
	errorCount := formatExportSummary(w, summaries, rootURL, showErrors)
	if errorCount > 0 {
		return fmt.Errorf("%d export(s) failed", errorCount)
	}
	return nil
}

// checkExportPrerequisites verifies Google credentials and token are available.
func checkExportPrerequisites(settings *config.Settings, store secrets.SecretStore) error {
	if settings.GoogleCredentialsFile != "" {
		// Custom credentials path — still check via store
	}
	if _, err := store.Get(secrets.KeyClientCredentials); err != nil {
		return fmt.Errorf("Google credentials not found — run: get-out init\n\nDownload credentials.json from Google Cloud Console")
	}
	if _, err := store.Get(secrets.KeyOAuthToken); err != nil {
		fmt.Println("Google authorization required. Run 'get-out auth login' first.")
		return fmt.Errorf("no Google token found — run: get-out auth login")
	}
	return nil
}

// resolveExportFolderID determines the Google Drive folder ID from the CLI flag
// and settings. The flag takes priority, then settings.FolderID (set by init),
// then the legacy settings.GoogleDriveFolderID field.
func resolveExportFolderID(flagValue string, settings *config.Settings) string {
	if flagValue != "" {
		return flagValue
	}
	if settings.FolderID != "" {
		return settings.FolderID
	}
	return settings.GoogleDriveFolderID
}

// resolveLocalExportDir determines the local export directory from the CLI
// flag and settings. The flag takes priority over settings.LocalExportOutputDir.
func resolveLocalExportDir(flagValue string, settings *config.Settings) string {
	if flagValue != "" {
		return flagValue
	}
	return settings.LocalExportOutputDir
}

// formatLocalExportDryRun writes the local markdown export section of the
// dry-run output, showing which conversations have localExport enabled and
// where markdown files would be written.
func formatLocalExportDryRun(w io.Writer, conversations []config.ConversationConfig, localExportDir string) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Local Markdown Export: %s\n", localExportDir)
	hasLocal := false
	for _, c := range conversations {
		if c.LocalExport {
			typeName := exporter.SanitizeDirectoryName(string(c.Type), c.Name)
			fmt.Fprintf(w, "  - %s → %s/%s/\n", c.Name, localExportDir, typeName)
			hasLocal = true
		}
	}
	if !hasLocal {
		fmt.Fprintln(w, "  (no conversations have localExport: true)")
	}
}

// validateOllamaPrerequisites checks that the Ollama server is reachable and
// the configured model is available. Returns a user-friendly error with
// remediation hints on failure (SC-002: fail within 5 seconds).
func validateOllamaPrerequisites(ctx context.Context, client *ollama.Client) error {
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("Ollama is not reachable: %w\n\nEnsure Ollama is running (ollama serve) or use --no-sensitivity-filter to skip", err)
	}

	available, err := client.ModelAvailable(ctx)
	if err != nil {
		return fmt.Errorf("failed to check Ollama model availability: %w", err)
	}
	if !available {
		return fmt.Errorf("Ollama model not found\n\nPull the model with: ollama pull <model>\nOr use --no-sensitivity-filter to skip sensitivity filtering")
	}

	return nil
}

// resolveOllamaEndpoint determines the Ollama endpoint using the priority
// chain: CLI flag > settings.json > DefaultOllamaEndpoint.
func resolveOllamaEndpoint(flagValue string, settings *config.Settings) string {
	if flagValue != "" {
		return flagValue
	}
	if settings.Ollama != nil && settings.Ollama.Endpoint != "" {
		return settings.Ollama.Endpoint
	}
	return config.DefaultOllamaEndpoint
}

// parseDateRange converts --from and --to flag values into Slack timestamp
// strings. The --to value is adjusted to end-of-day (23:59:59).
func parseDateRange(from, to string) (dateFrom, dateTo string, err error) {
	dateFrom, err = parseDateFlag(from)
	if err != nil {
		return "", "", fmt.Errorf("invalid --from date: %w", err)
	}
	dateTo, err = parseDateFlag(to)
	if err != nil {
		return "", "", fmt.Errorf("invalid --to date: %w", err)
	}
	if dateTo != "" {
		parts := strings.SplitN(dateTo, ".", 2)
		ts, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return "", "", fmt.Errorf("internal: failed to parse --to timestamp %q: %w", dateTo, err)
		}
		ts += 86400 - 1 // end of day
		dateTo = fmt.Sprintf("%d.000000", ts)
	}
	return dateFrom, dateTo, nil
}
