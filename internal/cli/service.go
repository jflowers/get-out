package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const (
	serviceLabel    = "com.jflowers.get-out"
	wrapperName     = "get-out-wrapper.sh"
	plistName       = "com.jflowers.get-out.plist"
	systemdService  = "get-out.service"
	systemdTimer    = "get-out.timer"
	serviceInterval = 3600 // seconds (1 hour)
)

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install as an hourly background service",
	Long: `Install get-out as an hourly background service that runs:

  get-out export --sync --parallel 5

Uses launchd on macOS and systemd on Linux.
Re-running install safely overwrites the previous installation.`,
	RunE: runInstall,
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the hourly background service",
	Long:  `Stop and remove the get-out background service and wrapper script.`,
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
}

// ---------------------------------------------------------------------------
// Install
// ---------------------------------------------------------------------------

func runInstall(cmd *cobra.Command, args []string) error {
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine binary path: %w", err)
	}

	// Verify config directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return fmt.Errorf("config directory %s does not exist — run: get-out init", configDir)
	}

	fmt.Println("Installing get-out as an hourly background service...")
	fmt.Printf("  Binary: %s\n", binaryPath)

	switch runtime.GOOS {
	case "darwin":
		return installMacOS(binaryPath)
	case "linux":
		return installLinux(binaryPath)
	default:
		return fmt.Errorf("unsupported platform: %s (supported: darwin, linux)", runtime.GOOS)
	}
}

// writeWrapper creates the wrapper script directory and writes the script.
func writeWrapper(binaryPath, logPath, envFile string) (wrapperPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	wrapperDir := filepath.Join(home, ".local", "bin")
	wrapperPath = filepath.Join(wrapperDir, wrapperName)

	if err := os.MkdirAll(wrapperDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", wrapperDir, err)
	}

	wrapper := generateWrapper(binaryPath, logPath, envFile)
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0755); err != nil {
		return "", fmt.Errorf("failed to write wrapper script: %w", err)
	}
	fmt.Printf("  Wrapper: %s\n", wrapperPath)
	return wrapperPath, nil
}

func installMacOS(binaryPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	logPath := filepath.Join(home, "Library", "Logs", "get-out.log")
	envFile := filepath.Join(home, ".get-out", ".env")
	plistDir := filepath.Join(home, "Library", "LaunchAgents")
	plistPath := filepath.Join(plistDir, plistName)

	wrapperPath, err := writeWrapper(binaryPath, logPath, envFile)
	if err != nil {
		return err
	}

	// Write plist
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", plistDir, err)
	}
	plist := generatePlist(wrapperPath, logPath, home, binaryPath)
	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}
	fmt.Printf("  Service: %s\n", plistPath)

	// Unload previous (ignore errors — may not be loaded)
	uid := fmt.Sprintf("%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", "gui/"+uid, plistPath).Run()

	// Load new
	if err := exec.Command("launchctl", "bootstrap", "gui/"+uid, plistPath).Run(); err != nil {
		return fmt.Errorf("launchctl bootstrap failed: %w", err)
	}

	printInstallSummary(logPath, "launchctl list | grep get-out", "tail -f "+logPath)
	return nil
}

func printInstallSummary(logPath, statusCmd, logCmd string) {
	fmt.Printf("  Interval: hourly\n")
	fmt.Printf("  Log: %s\n", logPath)
	fmt.Printf("  Command: get-out export --sync --parallel 5\n")
	fmt.Println()
	fmt.Println(passStyle.Render("  ✓ Service installed and started"))
	fmt.Println()
	fmt.Printf("To check status: %s\n", statusCmd)
	fmt.Printf("To view logs: %s\n", logCmd)
	fmt.Println("To remove: get-out uninstall")
}

func installLinux(binaryPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	logPath := filepath.Join(home, ".local", "share", "get-out", "get-out.log")
	envFile := filepath.Join(home, ".get-out", ".env")
	systemdDir := filepath.Join(home, ".config", "systemd", "user")
	servicePath := filepath.Join(systemdDir, systemdService)
	timerPath := filepath.Join(systemdDir, systemdTimer)

	wrapperPath, err := writeWrapper(binaryPath, logPath, envFile)
	if err != nil {
		return err
	}

	// Write systemd units
	if err := os.MkdirAll(systemdDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", systemdDir, err)
	}
	if err := writeFile(servicePath, generateSystemdService(wrapperPath, home), "service unit"); err != nil {
		return err
	}
	if err := writeFile(timerPath, generateSystemdTimer(), "timer unit"); err != nil {
		return err
	}

	// Reload and enable
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	if err := exec.Command("systemctl", "--user", "enable", "--now", "get-out.timer").Run(); err != nil {
		return fmt.Errorf("systemctl enable failed: %w", err)
	}

	printInstallSummary(logPath, "systemctl --user status get-out.timer", "journalctl --user -u get-out.service")
	return nil
}

func writeFile(path, content, label string) error {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", label, err)
	}
	fmt.Printf("  %s: %s\n", strings.Title(label), path)
	return nil
}

// ---------------------------------------------------------------------------
// Uninstall
// ---------------------------------------------------------------------------

func runUninstall(cmd *cobra.Command, args []string) error {
	fmt.Println("Removing get-out background service...")

	switch runtime.GOOS {
	case "darwin":
		return uninstallMacOS()
	case "linux":
		return uninstallLinux()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func uninstallMacOS() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", plistName)
	wrapperPath := filepath.Join(home, ".local", "bin", wrapperName)

	// Unload (ignore errors — may not be loaded)
	uid := fmt.Sprintf("%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", "gui/"+uid, plistPath).Run()

	// Remove files (tolerate missing)
	removeIfExists(plistPath)
	removeIfExists(wrapperPath)

	fmt.Println(passStyle.Render("  ✓ Service stopped and removed"))
	return nil
}

func uninstallLinux() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	systemdDir := filepath.Join(home, ".config", "systemd", "user")
	servicePath := filepath.Join(systemdDir, systemdService)
	timerPath := filepath.Join(systemdDir, systemdTimer)
	wrapperPath := filepath.Join(home, ".local", "bin", wrapperName)

	// Disable timer (ignore errors)
	_ = exec.Command("systemctl", "--user", "disable", "--now", "get-out.timer").Run()

	// Remove files (tolerate missing)
	removeIfExists(servicePath)
	removeIfExists(timerPath)
	removeIfExists(wrapperPath)

	// Reload systemd
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	fmt.Println(passStyle.Render("  ✓ Service stopped and removed"))
	return nil
}

func removeIfExists(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "  Warning: could not remove %s: %v\n", path, err)
	}
}

// ---------------------------------------------------------------------------
// Doctor check helper
// ---------------------------------------------------------------------------

// isServiceInstalled checks whether the background service files exist.
func isServiceInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	switch runtime.GOOS {
	case "darwin":
		_, err := os.Stat(filepath.Join(home, "Library", "LaunchAgents", plistName))
		return err == nil
	case "linux":
		_, err := os.Stat(filepath.Join(home, ".config", "systemd", "user", systemdTimer))
		return err == nil
	default:
		return false
	}
}

// checkServiceInstalled reports whether the background sync service is installed.
// Used by the doctor command.
func checkServiceInstalled(passCount, warnCount *int) {
	if isServiceInstalled() {
		pass("Hourly sync service is installed")
		*passCount++
	} else {
		warn("Hourly sync service not installed")
		hint("Run: get-out install")
		*warnCount++
	}
}

// ---------------------------------------------------------------------------
// Template generators (pure functions — testable in isolation)
// ---------------------------------------------------------------------------

// generateWrapper returns the bash wrapper script content.
func generateWrapper(binaryPath, logPath, envFile string) string {
	// Single-quote the binary path for shell safety
	quotedBin := "'" + strings.ReplaceAll(binaryPath, "'", "'\\''") + "'"

	return fmt.Sprintf(`#!/bin/bash
# get-out service wrapper
# Generated by 'get-out install'

set -euo pipefail

# Source env file if it exists
ENV_FILE="%s"
if [ -f "$ENV_FILE" ]; then
    set -a
    source "$ENV_FILE"
    set +a
fi

# --- Log rotation ---
# Rotate log when it exceeds 5 MB, keeping one backup (.1).
# Max disk usage: ~10 MB (5 MB active + 5 MB rotated).
LOG_FILE="%s"
MAX_LOG_BYTES=$((5 * 1024 * 1024))  # 5 MB
if [ -f "$LOG_FILE" ]; then
    LOG_SIZE=$(stat -f%%z "$LOG_FILE" 2>/dev/null || stat --format=%%s "$LOG_FILE" 2>/dev/null || echo 0)
    if [ "$LOG_SIZE" -gt "$MAX_LOG_BYTES" ]; then
        mv "$LOG_FILE" "${LOG_FILE}.1"
        echo "$(date '+%%Y-%%m-%%d %%H:%%M:%%S') — Log rotated (was ${LOG_SIZE} bytes)"
    fi
fi

echo "$(date '+%%Y-%%m-%%d %%H:%%M:%%S') — Starting get-out sync"
%s export --sync --parallel 5 && RC=$? || RC=$?
echo "Exit code: $RC"
echo "$(date '+%%Y-%%m-%%d %%H:%%M:%%S') — Completed get-out sync"
exit $RC
`, envFile, logPath, quotedBin)
}

// generatePlist returns the macOS launchd plist XML content.
func generatePlist(wrapperPath, logPath, home, binaryPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>

    <key>ProgramArguments</key>
    <array>
        <string>/bin/bash</string>
        <string>%s</string>
    </array>

    <key>StartInterval</key>
    <integer>%d</integer>

    <key>RunAtLoad</key>
    <true/>

    <key>StandardOutPath</key>
    <string>%s</string>

    <key>StandardErrorPath</key>
    <string>%s</string>

    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin</string>
        <key>HOME</key>
        <string>%s</string>
    </dict>
</dict>
</plist>
`, serviceLabel, wrapperPath, serviceInterval, logPath, logPath, home)
}

// generateSystemdService returns the systemd service unit content.
func generateSystemdService(wrapperPath, home string) string {
	return fmt.Sprintf(`[Unit]
Description=get-out - Slack message export sync
Documentation=https://github.com/jflowers/get-out

[Service]
Type=oneshot
ExecStart=/bin/bash %s
Environment=HOME=%s

[Install]
WantedBy=default.target
`, wrapperPath, home)
}

// generateSystemdTimer returns the systemd timer unit content.
func generateSystemdTimer() string {
	return `[Unit]
Description=Run get-out export sync hourly

[Timer]
OnCalendar=hourly
Persistent=true
RandomizedDelaySec=120

[Install]
WantedBy=timers.target
`
}
