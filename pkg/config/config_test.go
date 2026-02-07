package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConversations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "conversations.json")

	data := `{
		"conversations": [
			{
				"id": "C04KFBJTDJR",
				"name": "team-engineering",
				"type": "channel",
				"mode": "api",
				"export": true,
				"share": false
			},
			{
				"id": "D06DDJ2UH2M",
				"name": "John Smith",
				"type": "dm",
				"mode": "browser",
				"export": true,
				"share": true
			}
		]
	}`

	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConversations(path)
	if err != nil {
		t.Fatalf("LoadConversations() error: %v", err)
	}

	if len(cfg.Conversations) != 2 {
		t.Fatalf("len(Conversations) = %d, want 2", len(cfg.Conversations))
	}

	if cfg.Conversations[0].ID != "C04KFBJTDJR" {
		t.Errorf("First ID = %q, want C04KFBJTDJR", cfg.Conversations[0].ID)
	}
	if cfg.Conversations[1].Name != "John Smith" {
		t.Errorf("Second Name = %q, want John Smith", cfg.Conversations[1].Name)
	}
}

func TestLoadConversations_Invalid(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name string
		data string
	}{
		{
			name: "missing id",
			data: `{"conversations": [{"name": "test", "type": "channel", "mode": "api"}]}`,
		},
		{
			name: "invalid id format",
			data: `{"conversations": [{"id": "invalid", "name": "test", "type": "channel", "mode": "api"}]}`,
		},
		{
			name: "missing name",
			data: `{"conversations": [{"id": "C123ABC", "type": "channel", "mode": "api"}]}`,
		},
		{
			name: "invalid type",
			data: `{"conversations": [{"id": "C123ABC", "name": "test", "type": "invalid", "mode": "api"}]}`,
		},
		{
			name: "invalid mode",
			data: `{"conversations": [{"id": "C123ABC", "name": "test", "type": "channel", "mode": "invalid"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name+".json")
			if err := os.WriteFile(path, []byte(tt.data), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := LoadConversations(path)
			if err == nil {
				t.Error("LoadConversations() expected error, got nil")
			}
		})
	}
}

func TestFilterByExport(t *testing.T) {
	cfg := &ConversationsConfig{
		Conversations: []ConversationConfig{
			{ID: "C1", Name: "a", Type: "channel", Mode: "api", Export: true},
			{ID: "C2", Name: "b", Type: "channel", Mode: "api", Export: false},
			{ID: "C3", Name: "c", Type: "channel", Mode: "api", Export: true},
		},
	}

	result := cfg.FilterByExport()
	if len(result) != 2 {
		t.Fatalf("FilterByExport() returned %d, want 2", len(result))
	}
	if result[0].ID != "C1" || result[1].ID != "C3" {
		t.Errorf("FilterByExport() = {%s, %s}, want {C1, C3}", result[0].ID, result[1].ID)
	}
}

func TestGetByID(t *testing.T) {
	cfg := &ConversationsConfig{
		Conversations: []ConversationConfig{
			{ID: "C1", Name: "one", Type: "channel", Mode: "api"},
			{ID: "C2", Name: "two", Type: "channel", Mode: "api"},
		},
	}

	// Found
	conv := cfg.GetByID("C2")
	if conv == nil {
		t.Fatal("GetByID(C2) returned nil")
	}
	if conv.Name != "two" {
		t.Errorf("GetByID(C2).Name = %q, want %q", conv.Name, "two")
	}

	// Not found
	if cfg.GetByID("C999") != nil {
		t.Error("GetByID(C999) should return nil")
	}
}

func TestLoadPeople(t *testing.T) {
	dir := t.TempDir()

	// Valid file
	path := filepath.Join(dir, "people.json")
	data := `{
		"people": [
			{
				"slackId": "U1234567890",
				"email": "user@example.com",
				"displayName": "John Doe"
			}
		]
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	people, err := LoadPeople(path)
	if err != nil {
		t.Fatalf("LoadPeople() error: %v", err)
	}
	if len(people.People) != 1 {
		t.Fatalf("len(People) = %d, want 1", len(people.People))
	}
	if people.People[0].DisplayName != "John Doe" {
		t.Errorf("DisplayName = %q, want %q", people.People[0].DisplayName, "John Doe")
	}

	// Non-existent file should return empty config (optional file)
	missing, err := LoadPeople(filepath.Join(dir, "missing.json"))
	if err != nil {
		t.Fatalf("LoadPeople(missing) error: %v", err)
	}
	if len(missing.People) != 0 {
		t.Errorf("Missing people file should return empty, got %d", len(missing.People))
	}
}

func TestLoadSettings(t *testing.T) {
	dir := t.TempDir()

	// Valid file
	path := filepath.Join(dir, "settings.json")
	data := `{
		"slackBotToken": "xoxb-test",
		"googleDriveFolderId": "folder123",
		"logLevel": "DEBUG"
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadSettings(path)
	if err != nil {
		t.Fatalf("LoadSettings() error: %v", err)
	}
	if s.SlackBotToken != "xoxb-test" {
		t.Errorf("SlackBotToken = %q, want xoxb-test", s.SlackBotToken)
	}
	if s.GoogleDriveFolderID != "folder123" {
		t.Errorf("GoogleDriveFolderID = %q, want folder123", s.GoogleDriveFolderID)
	}
	if s.LogLevel != "DEBUG" {
		t.Errorf("LogLevel = %q, want DEBUG", s.LogLevel)
	}

	// Missing file returns defaults
	def, err := LoadSettings(filepath.Join(dir, "missing.json"))
	if err != nil {
		t.Fatalf("LoadSettings(missing) error: %v", err)
	}
	if def.LogLevel != "INFO" {
		t.Errorf("Default LogLevel = %q, want INFO", def.LogLevel)
	}
}

func TestBuildUserMap(t *testing.T) {
	cfg := &PeopleConfig{
		People: []PersonConfig{
			{SlackID: "U111", DisplayName: "Alice"},
			{SlackID: "U222", DisplayName: "Bob"},
		},
	}

	m := cfg.BuildUserMap()
	if len(m) != 2 {
		t.Fatalf("BuildUserMap() len = %d, want 2", len(m))
	}
	if m["U111"].DisplayName != "Alice" {
		t.Errorf("m[U111].DisplayName = %q, want Alice", m["U111"].DisplayName)
	}
	if m["U222"].DisplayName != "Bob" {
		t.Errorf("m[U222].DisplayName = %q, want Bob", m["U222"].DisplayName)
	}
}
