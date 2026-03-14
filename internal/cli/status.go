package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jflowers/get-out/pkg/exporter"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show export status for all conversations",
	Long: `Show the current export status for all conversations in the export index.

Displays which conversations have been exported, their status (complete/in-progress),
message counts, number of docs created, and last updated time.`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	indexPath := exporter.DefaultIndexPath(configDir)
	index, err := exporter.LoadExportIndex(indexPath)
	if err != nil {
		return fmt.Errorf("failed to load export index: %w", err)
	}
	statusCore(os.Stdout, index)
	return nil
}

// statusCore formats and writes export status to w.
// Returns (totalConversations, completeCount) for summary.
func statusCore(w io.Writer, index *exporter.ExportIndex) (int, int) {
	convs := index.AllConversations()
	if len(convs) == 0 {
		fmt.Fprintln(w, "No exports found. Run 'get-out export' to start exporting.")
		return 0, 0
	}

	// Header
	if index.RootFolderURL != "" {
		fmt.Fprintf(w, "Root folder: %s\n", index.RootFolderURL)
	}
	fmt.Fprintf(w, "Last updated: %s\n", index.UpdatedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "\nExported conversations (%d):\n\n", len(convs))

	// Table header
	fmt.Fprintf(w, "  %-10s %-8s %-30s %6s %5s %5s  %s\n",
		"STATUS", "TYPE", "NAME", "MSGS", "DOCS", "THRDS", "LAST UPDATED")
	fmt.Fprintf(w, "  %-10s %-8s %-30s %6s %5s %5s  %s\n",
		"──────────", "────────", "──────────────────────────────", "──────", "─────", "─────", "────────────")

	totalMsgs := 0
	totalDocs := 0
	totalThreads := 0
	complete := 0

	for _, conv := range convs {
		status := conv.Status
		if status == "" {
			status = "unknown"
		}
		statusIcon := "⏸"
		if status == "complete" {
			statusIcon = "✅"
			complete++
		} else if status == "in_progress" {
			statusIcon = "🔄"
		}

		docCount := len(conv.DailyDocs)
		threadCount := len(conv.Threads)

		lastUpdated := "never"
		if !conv.LastUpdated.IsZero() {
			lastUpdated = conv.LastUpdated.Format("Jan 2 15:04")
		}

		name := conv.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}

		fmt.Fprintf(w, "  %s %-8s %-8s %-30s %6d %5d %5d  %s\n",
			statusIcon, status, conv.Type, name, conv.MessageCount, docCount, threadCount, lastUpdated)

		totalMsgs += conv.MessageCount
		totalDocs += docCount
		totalThreads += threadCount
	}

	fmt.Fprintf(w, "\nSummary: %d conversations (%d complete), %d messages, %d docs, %d threads\n",
		len(convs), complete, totalMsgs, totalDocs, totalThreads)

	return len(convs), complete
}
