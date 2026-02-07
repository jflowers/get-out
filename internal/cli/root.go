// Package cli implements the command-line interface for get-out.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	debugMode  bool
	chromePort int
	configDir  string
	verbose    bool

	// Build info (set via SetVersion)
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

// SetVersion sets the build version info from main.go ldflags.
func SetVersion(version, commit, date string) {
	buildVersion = version
	buildCommit = commit
	buildDate = date
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)
}

// rootCmd is the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "get-out",
	Short: "Export Slack messages to Google Docs",
	Long: `get-out is a tool to export Slack DMs, groups, and channels to Google Docs.

It supports both browser-based authentication (for DMs and group messages)
and API-based authentication (for channels where a bot is installed).

Prerequisites:
  1. Google Cloud project with Drive API and Docs API enabled
  2. OAuth credentials.json in config directory
  3. For browser mode: Chrome/Chromium running with remote debugging enabled

Quick Start:
  # Authenticate with Google
  get-out auth

  # List configured conversations
  get-out list

  # Export all configured conversations
  get-out export

  # Export a specific conversation
  get-out export D123ABC456`,
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Enable debug output")
	rootCmd.PersistentFlags().IntVar(&chromePort, "chrome-port", 9222, "Chrome DevTools Protocol port")
	rootCmd.PersistentFlags().StringVar(&configDir, "config", defaultConfigDir(), "Config directory path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
}

func defaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".config/get-out"
	}
	return fmt.Sprintf("%s/.config/get-out", home)
}
