package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jflowers/get-out/pkg/exporter"
)

func TestStatusCore_NoConversations(t *testing.T) {
	index := exporter.NewExportIndex("")

	var buf bytes.Buffer
	total, complete := statusCore(&buf, index)
	out := buf.String()

	if total != 0 {
		t.Errorf("expected total=0, got %d", total)
	}
	if complete != 0 {
		t.Errorf("expected complete=0, got %d", complete)
	}
	if !strings.Contains(out, "No exports found") {
		t.Errorf("expected 'No exports found' in output, got:\n%s", out)
	}
}

func TestStatusCore_WithConversations(t *testing.T) {
	index := exporter.NewExportIndex("")
	index.RootFolderURL = "https://drive.google.com/drive/folders/abc123"
	index.UpdatedAt = time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	index.SetConversation(&exporter.ConversationExport{
		ID:           "C001",
		Name:         "general",
		Type:         "channel",
		Status:       "complete",
		MessageCount: 150,
		DailyDocs: map[string]*exporter.DocExport{
			"2025-06-14": {DocID: "doc1"},
			"2025-06-15": {DocID: "doc2"},
		},
		Threads:     map[string]*exporter.ThreadExport{},
		LastUpdated: time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC),
	})

	index.SetConversation(&exporter.ConversationExport{
		ID:           "D001",
		Name:         "alice",
		Type:         "dm",
		Status:       "in_progress",
		MessageCount: 42,
		DailyDocs: map[string]*exporter.DocExport{
			"2025-06-15": {DocID: "doc3"},
		},
		Threads: map[string]*exporter.ThreadExport{
			"1234.5678": {ThreadTS: "1234.5678", ReplyCount: 5},
		},
		LastUpdated: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
	})

	index.SetConversation(&exporter.ConversationExport{
		ID:           "D002",
		Name:         "bob",
		Type:         "dm",
		Status:       "",
		MessageCount: 0,
		DailyDocs:    map[string]*exporter.DocExport{},
		Threads:      map[string]*exporter.ThreadExport{},
	})

	var buf bytes.Buffer
	total, complete := statusCore(&buf, index)
	out := buf.String()

	// Verify return values
	if total != 3 {
		t.Errorf("expected total=3, got %d", total)
	}
	if complete != 1 {
		t.Errorf("expected complete=1, got %d", complete)
	}

	// Verify root folder URL appears
	if !strings.Contains(out, "https://drive.google.com/drive/folders/abc123") {
		t.Errorf("expected root folder URL in output, got:\n%s", out)
	}

	// Verify conversation names appear
	for _, name := range []string{"general", "alice", "bob"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected conversation name %q in output, got:\n%s", name, out)
		}
	}

	// Verify status icons
	if !strings.Contains(out, "✅") {
		t.Errorf("expected complete icon '✅' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "🔄") {
		t.Errorf("expected in_progress icon '🔄' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "⏸") {
		t.Errorf("expected unknown/paused icon '⏸' in output, got:\n%s", out)
	}

	// Verify summary line
	if !strings.Contains(out, "3 conversations (1 complete)") {
		t.Errorf("expected summary '3 conversations (1 complete)' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "192 messages") {
		t.Errorf("expected '192 messages' in summary, got:\n%s", out)
	}
	if !strings.Contains(out, "3 docs") {
		t.Errorf("expected '3 docs' in summary, got:\n%s", out)
	}
	if !strings.Contains(out, "1 threads") {
		t.Errorf("expected '1 threads' in summary, got:\n%s", out)
	}

	// Verify table header
	if !strings.Contains(out, "STATUS") {
		t.Errorf("expected 'STATUS' table header in output, got:\n%s", out)
	}
}
