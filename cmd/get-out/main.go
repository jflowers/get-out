// get-out: A Slack message exporter to Google Docs.
// Designed for enterprise workspaces where traditional API access is restricted.
package main

import (
	"os"

	"github.com/jflowers/get-out/internal/cli"
)

// Build-time variables injected by GoReleaser via ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetVersion(version, commit, date)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
