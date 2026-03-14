package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/models"
)

// testConversationsConfig returns a ConversationsConfig with a variety of
// conversation types for use across multiple tests.
func testConversationsConfig() *config.ConversationsConfig {
	return &config.ConversationsConfig{
		Conversations: []config.ConversationConfig{
			{ID: "C001ABC", Name: "general", Type: models.ConversationTypeChannel, Mode: models.ExportModeBrowser, Export: true},
			{ID: "C002DEF", Name: "random", Type: models.ConversationTypeChannel, Mode: models.ExportModeBrowser, Export: false},
			{ID: "D003GHI", Name: "dm-alice", Type: models.ConversationTypeDM, Mode: models.ExportModeBrowser, Export: true},
			{ID: "D004JKL", Name: "dm-bob", Type: models.ConversationTypeDM, Mode: models.ExportModeBrowser, Export: false},
			{ID: "G005MNO", Name: "group-chat", Type: models.ConversationTypeMPIM, Mode: models.ExportModeAPI, Export: true},
		},
	}
}

func TestSelectConversations_ByArgs(t *testing.T) {
	cfg := testConversationsConfig()

	result, err := selectConversations(cfg, []string{"C001ABC", "D003GHI"}, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(result))
	}
	if result[0].ID != "C001ABC" {
		t.Errorf("result[0].ID = %q, want %q", result[0].ID, "C001ABC")
	}
	if result[1].ID != "D003GHI" {
		t.Errorf("result[1].ID = %q, want %q", result[1].ID, "D003GHI")
	}
}

func TestSelectConversations_ByArgs_NotFound(t *testing.T) {
	cfg := testConversationsConfig()

	_, err := selectConversations(cfg, []string{"CNOTEXIST"}, false, false)
	if err == nil {
		t.Fatal("expected error for missing conversation, got nil")
	}
	if !strings.Contains(err.Error(), "CNOTEXIST") {
		t.Errorf("error should mention the missing ID, got: %v", err)
	}
}

func TestSelectConversations_AllDMs(t *testing.T) {
	cfg := testConversationsConfig()

	result, err := selectConversations(cfg, nil, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 DM conversations, got %d", len(result))
	}
	for _, r := range result {
		if r.Type != models.ConversationTypeDM {
			t.Errorf("expected type %q, got %q for %s", models.ConversationTypeDM, r.Type, r.ID)
		}
	}
}

func TestSelectConversations_AllGroups(t *testing.T) {
	cfg := testConversationsConfig()

	result, err := selectConversations(cfg, nil, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 MPIM conversation, got %d", len(result))
	}
	if result[0].Type != models.ConversationTypeMPIM {
		t.Errorf("expected type %q, got %q", models.ConversationTypeMPIM, result[0].Type)
	}
}

func TestSelectConversations_Default(t *testing.T) {
	cfg := testConversationsConfig()

	result, err := selectConversations(cfg, nil, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return only conversations with Export=true: C001ABC, D003GHI, G005MNO
	if len(result) != 3 {
		t.Fatalf("expected 3 export=true conversations, got %d", len(result))
	}
	for _, r := range result {
		if !r.Export {
			t.Errorf("conversation %s has export=false, expected true", r.ID)
		}
	}
}

func TestValidateExportFlags_Valid(t *testing.T) {
	tests := []struct {
		name     string
		sync     bool
		resume   bool
		dateFrom string
		dateTo   string
	}{
		{name: "all defaults", sync: false, resume: false, dateFrom: "", dateTo: ""},
		{name: "sync only", sync: true, resume: false, dateFrom: "", dateTo: ""},
		{name: "resume only", sync: false, resume: true, dateFrom: "", dateTo: ""},
		{name: "from only", sync: false, resume: false, dateFrom: "2025-01-01", dateTo: ""},
		{name: "to only", sync: false, resume: false, dateFrom: "", dateTo: "2025-06-30"},
		{name: "from and to", sync: false, resume: false, dateFrom: "2025-01-01", dateTo: "2025-06-30"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateExportFlags(tt.sync, tt.resume, tt.dateFrom, tt.dateTo); err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestValidateExportFlags_SyncWithDate(t *testing.T) {
	tests := []struct {
		name     string
		dateFrom string
		dateTo   string
	}{
		{name: "sync with from", dateFrom: "2025-01-01", dateTo: ""},
		{name: "sync with to", dateFrom: "", dateTo: "2025-06-30"},
		{name: "sync with both", dateFrom: "2025-01-01", dateTo: "2025-06-30"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExportFlags(true, false, tt.dateFrom, tt.dateTo)
			if err == nil {
				t.Fatal("expected error for --sync with date flags, got nil")
			}
			if !strings.Contains(err.Error(), "--sync") {
				t.Errorf("error should mention --sync, got: %v", err)
			}
		})
	}
}

func TestValidateExportFlags_ResumeWithDate(t *testing.T) {
	tests := []struct {
		name     string
		dateFrom string
		dateTo   string
	}{
		{name: "resume with from", dateFrom: "2025-01-01", dateTo: ""},
		{name: "resume with to", dateFrom: "", dateTo: "2025-06-30"},
		{name: "resume with both", dateFrom: "2025-01-01", dateTo: "2025-06-30"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExportFlags(false, true, tt.dateFrom, tt.dateTo)
			if err == nil {
				t.Fatal("expected error for --resume with date flags, got nil")
			}
			if !strings.Contains(err.Error(), "--resume") {
				t.Errorf("error should mention --resume, got: %v", err)
			}
		})
	}
}

func TestFormatExportDryRun(t *testing.T) {
	convs := []config.ConversationConfig{
		{
			ID:   "C001ABC",
			Name: "general",
			Type: models.ConversationTypeChannel,
			Mode: models.ExportModeBrowser,
		},
		{
			ID:           "D002DEF",
			Name:         "dm-alice",
			Type:         models.ConversationTypeDM,
			Mode:         models.ExportModeBrowser,
			Share:        true,
			ShareMembers: []string{"alice@example.com", "bob@example.com"},
		},
		{
			ID:    "G003GHI",
			Name:  "group-chat",
			Type:  models.ConversationTypeMPIM,
			Mode:  models.ExportModeAPI,
			Share: true,
		},
	}

	var buf bytes.Buffer
	formatExportDryRun(&buf, convs)
	output := buf.String()

	// Verify header
	if !strings.Contains(output, "DRY RUN - Would export:") {
		t.Error("missing dry-run header")
	}

	// Verify each conversation appears
	if !strings.Contains(output, "general (C001ABC)") {
		t.Error("missing general conversation")
	}
	if !strings.Contains(output, "dm-alice (D002DEF)") {
		t.Error("missing dm-alice conversation")
	}
	if !strings.Contains(output, "group-chat (G003GHI)") {
		t.Error("missing group-chat conversation")
	}

	// Verify type/mode lines
	if !strings.Contains(output, "Type: channel, Mode: browser") {
		t.Error("missing type/mode for general")
	}

	// Verify sharing info with members
	if !strings.Contains(output, "Sharing: enabled with 2 members") {
		t.Error("missing sharing info with member count for dm-alice")
	}

	// Verify sharing without member count
	if !strings.Contains(output, "Sharing: enabled\n") {
		t.Error("missing sharing line without member count for group-chat")
	}
}

func TestFormatExportSummary(t *testing.T) {
	results := []ExportResultSummary{
		{Name: "general", MessageCount: 150, DocsCreated: 3, ThreadsExported: 10},
		{Name: "dm-alice", MessageCount: 42, DocsCreated: 1, ThreadsExported: 2, Error: fmt.Errorf("auth expired")},
		{Name: "random", MessageCount: 200, DocsCreated: 5, ThreadsExported: 15},
	}

	var buf bytes.Buffer
	errorCount := formatExportSummary(&buf, results, "https://drive.google.com/drive/folders/abc123", true)
	output := buf.String()

	// Verify header
	if !strings.Contains(output, "Export Summary") {
		t.Error("missing summary header")
	}

	// Verify OK status
	if !strings.Contains(output, "[OK]") {
		t.Error("missing OK status")
	}

	// Verify FAILED status
	if !strings.Contains(output, "[FAILED]") {
		t.Error("missing FAILED status")
	}

	// Verify error details shown (showErrors=true)
	if !strings.Contains(output, "Error: auth expired") {
		t.Error("missing error details when showErrors=true")
	}

	// Verify totals
	if !strings.Contains(output, "Total: 392 messages, 9 docs, 27 threads") {
		t.Errorf("incorrect totals in output: %s", output)
	}

	// Verify error count
	if errorCount != 1 {
		t.Errorf("expected errorCount=1, got %d", errorCount)
	}
	if !strings.Contains(output, "Errors: 1 conversation(s) failed") {
		t.Error("missing error count line")
	}

	// Verify folder URL
	if !strings.Contains(output, "https://drive.google.com/drive/folders/abc123") {
		t.Error("missing folder URL")
	}

	// Test with showErrors=false
	var buf2 bytes.Buffer
	formatExportSummary(&buf2, results, "", false)
	output2 := buf2.String()

	if strings.Contains(output2, "Error: auth expired") {
		t.Error("error details should be hidden when showErrors=false")
	}

	// No root URL provided — should not print folder section
	if strings.Contains(output2, "Export folder:") {
		t.Error("should not print export folder when rootURL is empty")
	}
}
