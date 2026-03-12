package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/jflowers/get-out/pkg/chrome"
	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/exporter"
	"github.com/jflowers/get-out/pkg/gdrive"
	"github.com/jflowers/get-out/pkg/secrets"
	"github.com/jflowers/get-out/pkg/slackapi"
	"github.com/spf13/cobra"
)

// driveIDPattern is compiled once at package init to avoid per-call allocations.
var driveIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ---------------------------------------------------------------------------
// T009: Shared lipgloss style constants
// ---------------------------------------------------------------------------

var (
	passStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	failStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	dimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	boxStyle  = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)
)

func pass(msg string) { fmt.Println(passStyle.Render("  ✓ " + msg)) }
func warn(msg string) { fmt.Println(warnStyle.Render("  ⚠ " + msg)) }
func fail(msg string) { fmt.Println(failStyle.Render("  ✗ " + msg)) }
func hint(msg string) { fmt.Println(dimStyle.Render("    → " + msg)) }

// ---------------------------------------------------------------------------
// T011/T012: init command
// ---------------------------------------------------------------------------

var initNonInteractive bool

var initCmd = &cobra.Command{
	Use:          "init",
	Short:        "Initialize the get-out configuration directory",
	SilenceUsage: true,
	Long: `Initialize the get-out configuration directory (~/.get-out/).

This command will:
  1. Create ~/.get-out/ if it doesn't exist
  2. Migrate existing files from ~/.config/get-out/ (if present)
  3. Create starter templates for missing config files
  4. Prompt for your Google Drive folder ID (interactive mode)
  5. Print next steps

Run this command once after installing get-out.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "Skip interactive prompts")
	rootCmd.AddCommand(initCmd)
}

// managedFiles are the config files that migration copies between directories.
var managedFiles = []string{
	"settings.json",
	"conversations.json",
	"people.json",
	"credentials.json",
	"token.json",
}

// sensitiveFiles require mode 0600.
var sensitiveFiles = map[string]bool{
	"credentials.json": true,
	"token.json":       true,
}

func runInit(cmd *cobra.Command, args []string) error {
	newDir := configDir

	// Step 1: Create config directory.
	info, err := os.Stat(newDir)
	if err == nil && !info.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", newDir)
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat config dir: %w", err)
	}
	if err := os.MkdirAll(newDir, 0700); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}
	fmt.Printf("Config directory: %s\n", newDir)

	// Step 2: Migration from old directory.
	home, _ := os.UserHomeDir()
	oldDir := filepath.Join(home, ".config", "get-out")
	if oldDir != newDir {
		if _, err := os.Stat(oldDir); err == nil {
			fmt.Println("Found existing config at ~/.config/get-out — migrating files...")
			if err := migrateFiles(oldDir, newDir); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}
			fmt.Println("Migration complete. You may delete ~/.config/get-out/ when ready.")
		}
	}

	// Step 3: Create starter conversations.json if absent.
	convPath := filepath.Join(newDir, "conversations.json")
	if _, err := os.Stat(convPath); os.IsNotExist(err) {
		template := []byte(`{"conversations":[]}` + "\n")
		if err := os.WriteFile(convPath, template, 0644); err != nil {
			return fmt.Errorf("failed to create conversations.json: %w", err)
		}
		fmt.Println("Created conversations.json template.")
	}

	// Step 4: Create starter settings.json if absent.
	settingsPath := filepath.Join(newDir, "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		if err := os.WriteFile(settingsPath, []byte("{}\n"), 0600); err != nil {
			return fmt.Errorf("failed to create settings.json: %w", err)
		}
		fmt.Println("Created settings.json template.")
	}

	// Step 5: Prompt for folder ID.
	settings, err := config.LoadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	if !initNonInteractive && settings.FolderID == "" && isTerminal() {
		folderID, err := promptFolderID()
		if err != nil && err != huh.ErrUserAborted {
			return fmt.Errorf("prompt failed: %w", err)
		}
		if folderID != "" {
			settings.FolderID = folderID
			data, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal settings: %w", err)
			}
			if err := os.WriteFile(settingsPath, append(data, '\n'), 0600); err != nil {
				return fmt.Errorf("failed to save settings.json: %w", err)
			}
			fmt.Printf("Saved folder ID to settings.json.\n")
		}
	}

	// Step 6: Migrate secrets to SecretStore (idempotent; safe to call every init).
	interactive := !initNonInteractive
	migratePromptFn := secrets.PromptFunc(func(message string) (bool, error) {
		var confirm bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(message).
					Value(&confirm),
			),
		)
		if err := form.Run(); err != nil {
			return false, err
		}
		return confirm, nil
	})
	if err := secrets.Migrate(secretStore, configDir, interactive, migratePromptFn); err != nil {
		// Non-fatal: log but don't fail init
		fmt.Printf("Warning: secret migration incomplete: %v\n", err)
	}

	// Step 7: Print next steps.
	fmt.Println()
	nextSteps := `Next Steps:
  1. Copy credentials.json to ~/.get-out/
     (Download from: https://console.cloud.google.com/apis/credentials)
  2. Run: get-out auth login
  3. Run: get-out setup-browser
  4. Run: get-out export`
	fmt.Println(boxStyle.Render(nextSteps))

	return nil
}

// migrateFiles copies files from oldDir to newDir that don't already exist in newDir.
func migrateFiles(oldDir, newDir string) error {
	for _, filename := range managedFiles {
		srcPath := filepath.Join(oldDir, filename)
		dstPath := filepath.Join(newDir, filename)

		// Skip if source doesn't exist.
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return fmt.Errorf("failed to stat %s: %w", srcPath, err)
		}

		// Skip if destination already exists.
		if _, err := os.Stat(dstPath); err == nil {
			continue
		}

		// Read source.
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", srcPath, err)
		}

		// Write destination with appropriate permissions.
		perm := os.FileMode(0644)
		if sensitiveFiles[filename] {
			perm = 0600
		}
		if err := os.WriteFile(dstPath, data, perm); err != nil {
			return fmt.Errorf("failed to write %s: %w", dstPath, err)
		}
		fmt.Printf("  Migrated %s\n", filename)
	}
	return nil
}

// validateDriveID validates a Google Drive folder ID.
func validateDriveID(id string) error {
	if len(id) < 28 {
		return fmt.Errorf("ID too short (must be at least 28 characters)")
	}
	if !driveIDPattern.MatchString(id) {
		return fmt.Errorf("ID contains invalid characters (only alphanumeric, _, - allowed)")
	}
	return nil
}

// isTerminal returns true when stdin is a real TTY.
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// promptFolderID shows an interactive huh prompt for the Google Drive folder ID.
func promptFolderID() (string, error) {
	var id string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Google Drive Folder ID").
				Description("Paste the folder ID from your Drive URL:\nhttps://drive.google.com/drive/folders/<FOLDER_ID>").
				Value(&id).
				Validate(validateDriveID),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return id, nil
}

// ---------------------------------------------------------------------------
// T020–T025: doctor command
// ---------------------------------------------------------------------------

var doctorCmd = &cobra.Command{
	Use:          "doctor",
	Short:        "Check system health before exporting",
	SilenceUsage: true,
	Long: `Run health checks to diagnose common setup issues.

Checks:
  1.  Config directory exists and has correct permissions
  2.  credentials.json is present in the secret store
  3.  token.json is present in the secret store
  4.  OAuth token is valid or can be refreshed
  5.  Google Drive API is reachable
  6.  conversations.json is valid
  7.  people.json is present
  8.  Chrome is reachable on the configured port
  9.  A Slack tab is open in Chrome
  10. export-index.json is healthy (or absent for first run)

Exits 0 when all checks pass or only warnings exist.
Exits 1 when any check fails.`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	var passCount, warnCount, failCount int

	fmt.Println("─────────────────────────────────────────")
	fmt.Println("  get-out doctor")
	fmt.Println("─────────────────────────────────────────")
	fmt.Println()

	// Check 1: Config directory (also reports active secret backend)
	checkConfigDir(configDir, secretBackend, &passCount, &warnCount, &failCount)

	// Check 2: credentials (via SecretStore)
	checkSecret("credentials", secrets.KeyClientCredentials, secretStore, "Download credentials.json from: https://console.cloud.google.com/apis/credentials\n    Then run: get-out init  (to migrate to keychain)", &passCount, &failCount)

	// Check 3: token (via SecretStore)
	tokenPresent := checkSecret("token", secrets.KeyOAuthToken, secretStore, "Run: get-out auth login", &passCount, &failCount)

	// Check 4: OAuth token validity
	gdriveAPIOK := false
	if tokenPresent {
		gdriveAPIOK = checkTokenValidity(configDir, secretStore, &passCount, &warnCount, &failCount)
	} else {
		fmt.Println(dimStyle.Render("  — OAuth token check skipped (token absent)"))
	}

	// Check 5: Drive API
	if gdriveAPIOK {
		checkDriveAPI(configDir, secretStore, &passCount, &warnCount, &failCount)
	} else {
		fmt.Println(dimStyle.Render("  — Drive API check skipped"))
	}

	// Check 6: conversations.json
	checkConversations(configDir, &passCount, &warnCount, &failCount)

	// Check 7: people.json
	checkPeople(configDir, &passCount, &warnCount, &failCount)

	// Check 8: Chrome port
	chromeOK := checkChrome(chromePort, &passCount, &warnCount, &failCount)

	// Check 9: Slack tab
	if chromeOK {
		checkSlackTab(chromePort, &passCount, &warnCount, &failCount)
	} else {
		warn("Slack tab check skipped (Chrome not reachable)")
		warnCount++
	}

	// Check 10: export-index.json
	checkExportIndex(configDir, &passCount, &warnCount, &failCount)

	// T019: Old directory warning
	home, _ := os.UserHomeDir()
	oldDir := filepath.Join(home, ".config", "get-out")
	if _, err := os.Stat(oldDir); err == nil {
		if _, err2 := os.Stat(configDir); err2 == nil && oldDir != configDir {
			warn("Old config dir ~/.config/get-out/ still exists alongside ~/.get-out/")
			hint("Run: rm -rf ~/.config/get-out/")
			warnCount++
		}
	}

	fmt.Println()
	fmt.Println("─────────────────────────────────────────")
	summary := fmt.Sprintf("  %d passed · %d warnings · %d failures", passCount, warnCount, failCount)
	if failCount > 0 {
		fmt.Println(failStyle.Render(summary))
	} else if warnCount > 0 {
		fmt.Println(warnStyle.Render(summary))
	} else {
		fmt.Println(passStyle.Render(summary))
	}
	fmt.Println("─────────────────────────────────────────")

	if failCount > 0 {
		os.Exit(1)
	}
	return nil
}

// checkConfigDir checks check 1 — config directory existence, permissions, and secret backend.
func checkConfigDir(dir string, backend secrets.Backend, pass_, warn_, fail_ *int) {
	info, err := os.Stat(dir)
	if err != nil {
		fail(fmt.Sprintf("Config directory not found: %s", dir))
		hint("Run: get-out init")
		*fail_++
		return
	}
	if !info.IsDir() {
		fail(fmt.Sprintf("Config path is not a directory: %s", dir))
		*fail_++
		return
	}
	pass(fmt.Sprintf("Config directory exists (secret storage: %s)", backend))
	if verbose {
		fmt.Println(dimStyle.Render("    " + dir))
	}
	*pass_++
	if info.Mode().Perm()&0077 != 0 {
		warn("Config directory permissions are broader than 0700")
		hint(fmt.Sprintf("Run: chmod 700 %s", dir))
		*warn_++
	}
}

// checkSecret checks whether a secret key is present in the SecretStore (checks 2 and 3).
// Returns true if the secret is present.
func checkSecret(name, key string, store secrets.SecretStore, fixMsg string, pass_, fail_ *int) bool {
	if _, err := store.Get(key); err != nil {
		fail(name + " not found in secret store")
		hint(fixMsg)
		*fail_++
		return false
	}
	pass(name + " present")
	*pass_++
	return true
}

// checkTokenValidity checks check 4. Returns whether a Drive API call can proceed.
func checkTokenValidity(dir string, store secrets.SecretStore, pass_, warn_, fail_ *int) bool {
	token, err := gdrive.LoadTokenFromStore(store)
	if err != nil {
		fail("OAuth token: unreadable — " + err.Error())
		hint("Run: get-out auth login")
		*fail_++
		return false
	}
	if token.Valid() {
		pass("OAuth token is valid")
		if verbose {
			fmt.Println(dimStyle.Render(fmt.Sprintf("    Expires: %s", token.Expiry.Format(time.RFC3339))))
		}
		*pass_++
		return true
	}
	if token.RefreshToken != "" {
		warn("OAuth token is expired but can be auto-refreshed")
		hint("Token will refresh automatically on next use")
		*warn_++
		return true
	}
	fail("OAuth token is expired and has no refresh token")
	hint("Run: get-out auth login")
	*fail_++
	return false
}

// checkDriveAPI checks check 5.
// ClientFromStore is used (not AuthenticateWithStore) so that doctor never
// initiates an interactive browser OAuth flow — it is a read-only diagnostic.
func checkDriveAPI(dir string, store secrets.SecretStore, pass_, warn_, fail_ *int) {
	cfg := gdrive.DefaultConfig(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := gdrive.ClientFromStore(ctx, cfg, store)
	if err != nil {
		fail("Drive API: authentication error — " + err.Error())
		hint("Run: get-out auth login")
		*fail_++
		return
	}
	driveClient, err := gdrive.NewClient(ctx, client)
	if err != nil {
		fail("Drive API: could not create client — " + err.Error())
		*fail_++
		return
	}
	about, err := driveClient.Drive.About.Get().Fields("user").Context(ctx).Do()
	if err != nil {
		fail("Drive API: request failed — " + err.Error())
		hint("Check network connectivity and OAuth scopes")
		*fail_++
		return
	}
	pass(fmt.Sprintf("Drive API: connected as %s", about.User.EmailAddress))
	*pass_++
}

// checkConversations checks check 6.
func checkConversations(dir string, pass_, warn_, fail_ *int) {
	path := filepath.Join(dir, "conversations.json")
	cfg, err := config.LoadConversations(path)
	if err != nil {
		fail("conversations.json: " + err.Error())
		hint("Fix JSON or run: get-out discover")
		*fail_++
		return
	}
	if len(cfg.Conversations) == 0 {
		warn("conversations.json has no conversations configured")
		hint("Run: get-out discover   (or add conversations manually)")
		*warn_++
		return
	}
	pass(fmt.Sprintf("conversations.json: %d conversation(s) configured", len(cfg.Conversations)))
	if verbose {
		fmt.Println(dimStyle.Render("    " + path))
	}
	*pass_++
}

// checkPeople checks check 7.
func checkPeople(dir string, pass_, warn_, fail_ *int) {
	path := filepath.Join(dir, "people.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		warn("people.json not found (user mention linking disabled)")
		hint("Run: get-out discover")
		*warn_++
		return
	}
	pass("people.json exists")
	if verbose {
		fmt.Println(dimStyle.Render("    " + path))
	}
	*pass_++
}

// chromeLaunchCmd returns the OS-appropriate command to launch Chrome with remote debugging.
func chromeLaunchCmd(goos string, port int) string {
	if goos == "darwin" {
		return fmt.Sprintf(`open -a "Google Chrome" --args --remote-debugging-port=%d`, port)
	}
	return fmt.Sprintf("google-chrome --remote-debugging-port=%d", port)
}

// checkChrome checks check 8. Returns true if Chrome is reachable.
func checkChrome(port int, pass_, warn_, fail_ *int) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fail(fmt.Sprintf("Chrome not reachable on port %d", port))
		hint(fmt.Sprintf("Open Chrome with: %s", chromeLaunchCmd(runtime.GOOS, port)))
		*fail_++
		return false
	}
	resp.Body.Close()
	pass(fmt.Sprintf("Chrome reachable on port %d", port))
	*pass_++
	return true
}

// checkSlackTab checks check 9.
func checkSlackTab(port int, pass_, warn_, fail_ *int) {
	cfg := &chrome.Config{DebugPort: port, Timeout: 5 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := chrome.Connect(ctx, cfg)
	if err != nil {
		warn("Could not connect to Chrome to list tabs")
		*warn_++
		return
	}
	defer session.Close()

	targets, err := session.ListTargets(ctx)
	if err != nil {
		warn("Could not list Chrome tabs")
		*warn_++
		return
	}

	var slackCount int
	for _, t := range targets {
		if chrome.IsSlackURL(t.URL) {
			slackCount++
		}
	}

	if slackCount == 0 {
		warn("No Slack tab found in Chrome")
		hint("Open https://app.slack.com in Chrome and log in")
		*warn_++
		return
	}
	pass(fmt.Sprintf("Slack tab found (%d tab(s))", slackCount))
	*pass_++
}

// checkExportIndex checks check 10.
func checkExportIndex(dir string, pass_, warn_, fail_ *int) {
	path := filepath.Join(dir, "export-index.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		warn("export-index.json not found (first run — will be created on export)")
		*warn_++
		return
	}
	_, err := exporter.LoadExportIndex(path)
	if err != nil {
		fail("export-index.json is corrupt: " + err.Error())
		hint("Delete the file and re-run export to rebuild: rm " + path)
		*fail_++
		return
	}
	pass("export-index.json is healthy")
	if verbose {
		fmt.Println(dimStyle.Render("    " + path))
	}
	*pass_++
}

// ---------------------------------------------------------------------------
// T028: setup-browser command
// ---------------------------------------------------------------------------

var setupBrowserCmd = &cobra.Command{
	Use:          "setup-browser",
	Short:        "Guided wizard to verify Chrome and Slack setup",
	SilenceUsage: true,
	Long: `Run a 5-step wizard to verify your Chrome + Slack session is ready for export.

Steps:
  1. Verify Chrome is running with remote debugging enabled
  2. List browser tabs
  3. Detect a logged-in Slack tab
  4. Extract Slack credentials (token + cookie)
  5. Validate credentials against the Slack API`,
	RunE: runSetupBrowser,
}

func init() {
	rootCmd.AddCommand(setupBrowserCmd)
}

func runSetupBrowser(cmd *cobra.Command, args []string) error {
	fmt.Println("─────────────────────────────────────────")
	fmt.Println("  get-out setup-browser")
	fmt.Println("─────────────────────────────────────────")
	fmt.Println()

	stepFailed := false
	slackTabFound := false

	// Step 1: Chrome reachability
	fmt.Print("Step 1  Chrome reachable on port ")
	fmt.Printf("%d ... ", chromePort)
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", chromePort)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Println(failStyle.Render("FAIL"))
		stepFailed = true
		fmt.Println(dimStyle.Render(fmt.Sprintf("  Launch Chrome: %s", chromeLaunchCmd(runtime.GOOS, chromePort))))
	} else {
		resp.Body.Close()
		fmt.Println(passStyle.Render("OK"))
	}

	// Step 2: List tabs
	fmt.Print("Step 2  List browser tabs ... ")
	var session *chrome.Session
	if stepFailed {
		fmt.Println(dimStyle.Render("Skipped"))
	} else {
		cfg := &chrome.Config{DebugPort: chromePort, Timeout: 5 * time.Second}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		session, err = chrome.Connect(ctx, cfg)
		if err != nil {
			fmt.Println(failStyle.Render("FAIL"))
			fmt.Println(dimStyle.Render("  " + err.Error()))
			stepFailed = true
		} else {
			ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel2()
			targets, err := session.ListTargets(ctx2)
			if err != nil {
				fmt.Println(failStyle.Render("FAIL"))
				stepFailed = true
			} else {
				fmt.Println(passStyle.Render(fmt.Sprintf("OK (%d tab(s))", len(targets))))
			}
		}
	}

	// Step 3: Find Slack tab
	fmt.Print("Step 3  Find Slack tab ... ")
	var slackURL string
	if stepFailed || session == nil {
		fmt.Println(dimStyle.Render("Skipped"))
	} else {
		ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel3()
		targets, err := session.ListTargets(ctx3)
		if err != nil {
			fmt.Println(warnStyle.Render("WARN (could not list tabs)"))
		} else {
			var found int
			for _, t := range targets {
				if chrome.IsSlackURL(t.URL) {
					found++
					slackURL = t.URL
				}
			}
			if found == 0 {
				fmt.Println(warnStyle.Render("WARN (no Slack tab found)"))
				fmt.Println(dimStyle.Render("  Open https://app.slack.com in Chrome and log in"))
				// step 3 is warn-only; stepFailed stays false but slackTabFound stays false
				// so steps 4–5 are skipped (nothing to extract without a Slack tab)
			} else {
				slackTabFound = true
				msg := fmt.Sprintf("OK (%d tab(s))", found)
				if verbose {
					msg += "  " + truncateURL(slackURL)
				}
				fmt.Println(passStyle.Render(msg))
			}
		}
	}

	// Step 4: Extract credentials
	fmt.Print("Step 4  Extract Slack credentials ... ")
	var creds interface{ GetToken() string }
	if stepFailed || session == nil || !slackTabFound {
		fmt.Println(dimStyle.Render("Skipped"))
	} else {
		ctx4, cancel4 := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel4()
		extracted, err := session.ExtractCredentials(ctx4)
		if err != nil {
			fmt.Println(failStyle.Render("FAIL"))
			fmt.Println(dimStyle.Render("  " + err.Error()))
			stepFailed = true
		} else {
			fmt.Println(passStyle.Render("OK"))
			fmt.Println(dimStyle.Render(fmt.Sprintf("  Token:  %s", safePreview(extracted.Token))))
			fmt.Println(dimStyle.Render(fmt.Sprintf("  Cookie: %s", safePreview(extracted.Cookie))))
			_ = creds
			creds = &extractedCreds{token: extracted.Token, cookie: extracted.Cookie}
		}
	}

	// Step 5: Validate credentials
	fmt.Print("Step 5  Validate against Slack API ... ")
	if stepFailed || creds == nil {
		fmt.Println(dimStyle.Render("Skipped"))
	} else {
		ec := creds.(*extractedCreds)
		slackClient := slackapi.NewBrowserClient(ec.token, ec.cookie)
		ctx5, cancel5 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel5()
		auth, err := slackClient.ValidateAuth(ctx5)
		if err != nil {
			fmt.Println(failStyle.Render("FAIL"))
			fmt.Println(dimStyle.Render("  " + err.Error()))
			stepFailed = true
		} else {
			fmt.Println(passStyle.Render(fmt.Sprintf("OK (%s / %s)", auth.Team, auth.User)))
		}
	}

	// If no Slack tab was found in step 3, steps 4–5 were skipped —
	// mark as failed so the footer correctly reflects incomplete setup.
	if !slackTabFound {
		stepFailed = true
	}

	fmt.Println()
	fmt.Println("─────────────────────────────────────────")
	if !stepFailed {
		fmt.Println(passStyle.Render("  Setup complete. Run: get-out export"))
	} else {
		fmt.Println(failStyle.Render("  Setup incomplete. Fix the failures above and re-run."))
	}
	fmt.Println("─────────────────────────────────────────")

	if stepFailed {
		os.Exit(1)
	}
	return nil
}

// extractedCreds holds credentials extracted from the browser.
type extractedCreds struct {
	token  string
	cookie string
}

func (e *extractedCreds) GetToken() string { return e.token }
