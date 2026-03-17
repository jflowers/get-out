package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/slackapi"
)

// 3.3 — writePeopleJSON
func TestWritePeopleJSON_NoMerge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "people.json")

	users := []*slackapi.User{
		{ID: "U1", Name: "alice", Profile: slackapi.UserProfile{DisplayName: "Alice", Email: "alice@test.com"}},
		{ID: "U2", Name: "bob", Profile: slackapi.UserProfile{DisplayName: "Bob"}},
	}

	count, err := writePeopleJSON(path, users, nil, false)
	if err != nil {
		t.Fatalf("writePeopleJSON: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	// Verify file content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var cfg config.PeopleConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(cfg.People) != 2 {
		t.Errorf("len(people) = %d, want 2", len(cfg.People))
	}
	if cfg.People[0].SlackID != "U1" {
		t.Errorf("people[0].SlackID = %q, want U1", cfg.People[0].SlackID)
	}
}

func TestWritePeopleJSON_WithMerge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "people.json")

	existing := map[string]config.PersonConfig{
		"U0": {SlackID: "U0", DisplayName: "Existing"},
	}

	users := []*slackapi.User{
		{ID: "U1", Name: "alice", Profile: slackapi.UserProfile{DisplayName: "Alice"}},
	}

	count, err := writePeopleJSON(path, users, existing, true)
	if err != nil {
		t.Fatalf("writePeopleJSON: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2 (1 existing + 1 new)", count)
	}
}

func TestWritePeopleJSON_FiltersBots(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "people.json")

	users := []*slackapi.User{
		{ID: "U1", Name: "alice", Profile: slackapi.UserProfile{DisplayName: "Alice"}},
		{ID: "B1", Name: "bot", IsBot: true},
		{ID: "A1", Name: "app", IsAppUser: true},
		{ID: "D1", Name: "deleted", Deleted: true},
	}

	count, err := writePeopleJSON(path, users, nil, false)
	if err != nil {
		t.Fatalf("writePeopleJSON: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (only non-bot/non-deleted)", count)
	}
}

func TestWritePeopleJSON_EmptyUsers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "people.json")

	count, err := writePeopleJSON(path, nil, nil, false)
	if err != nil {
		t.Fatalf("writePeopleJSON: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

// 3.4 — resolveExportFolderID
func TestResolveExportFolderID_FlagPriority(t *testing.T) {
	settings := &config.Settings{
		FolderID:            "from-init",
		GoogleDriveFolderID: "from-legacy",
	}
	got := resolveExportFolderID("from-flag", settings)
	if got != "from-flag" {
		t.Errorf("got %q, want from-flag", got)
	}
}

func TestResolveExportFolderID_SettingsFolderID(t *testing.T) {
	settings := &config.Settings{
		FolderID:            "from-init",
		GoogleDriveFolderID: "from-legacy",
	}
	got := resolveExportFolderID("", settings)
	if got != "from-init" {
		t.Errorf("got %q, want from-init", got)
	}
}

func TestResolveExportFolderID_LegacyFallback(t *testing.T) {
	settings := &config.Settings{
		GoogleDriveFolderID: "from-legacy",
	}
	got := resolveExportFolderID("", settings)
	if got != "from-legacy" {
		t.Errorf("got %q, want from-legacy", got)
	}
}

func TestResolveExportFolderID_Empty(t *testing.T) {
	settings := &config.Settings{}
	got := resolveExportFolderID("", settings)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// 3.5 — parseDateRange
func TestParseDateRange_BothEmpty(t *testing.T) {
	from, to, err := parseDateRange("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if from != "" || to != "" {
		t.Errorf("got from=%q to=%q, want both empty", from, to)
	}
}

func TestParseDateRange_FromOnly(t *testing.T) {
	from, to, err := parseDateRange("2024-01-15", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if from == "" {
		t.Error("from should not be empty")
	}
	if to != "" {
		t.Errorf("to = %q, want empty", to)
	}
}

func TestParseDateRange_ToAdjustedToEndOfDay(t *testing.T) {
	_, to, err := parseDateRange("", "2024-01-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The end-of-day adjustment adds 86399 seconds to the start of the day.
	// Verify the timestamp is > start of day.
	if to == "" {
		t.Fatal("to should not be empty")
	}
	// The base timestamp for 2024-01-15 00:00:00 UTC is 1705276800.
	// After end-of-day: 1705276800 + 86399 = 1705363199.
	expected := "1705363199.000000"
	if to != expected {
		t.Errorf("to = %q, want %q", to, expected)
	}
}

func TestParseDateRange_InvalidFrom(t *testing.T) {
	_, _, err := parseDateRange("not-a-date", "")
	if err == nil {
		t.Error("expected error for invalid --from date")
	}
}

func TestParseDateRange_InvalidTo(t *testing.T) {
	_, _, err := parseDateRange("", "not-a-date")
	if err == nil {
		t.Error("expected error for invalid --to date")
	}
}
