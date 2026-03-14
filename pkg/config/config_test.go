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

func TestBuildEmailMap(t *testing.T) {
	cfg := &PeopleConfig{
		People: []PersonConfig{
			{SlackID: "U111", Email: "alice@work.com", GoogleEmail: "alice@gmail.com"},
			{SlackID: "U222", Email: "bob@work.com"},
			{SlackID: "U333", GoogleEmail: "carol@gmail.com"},
			{SlackID: "U444"}, // no email at all
		},
	}

	m := cfg.BuildEmailMap()

	// Should have 3 entries: alice@work.com, alice@gmail.com, bob@work.com, carol@gmail.com
	if len(m) != 4 {
		t.Fatalf("BuildEmailMap() len = %d, want 4", len(m))
	}
	if m["alice@work.com"].SlackID != "U111" {
		t.Errorf("m[alice@work.com].SlackID = %q, want U111", m["alice@work.com"].SlackID)
	}
	if m["alice@gmail.com"].SlackID != "U111" {
		t.Errorf("m[alice@gmail.com].SlackID = %q, want U111", m["alice@gmail.com"].SlackID)
	}
	if m["bob@work.com"].SlackID != "U222" {
		t.Errorf("m[bob@work.com].SlackID = %q, want U222", m["bob@work.com"].SlackID)
	}
	if m["carol@gmail.com"].SlackID != "U333" {
		t.Errorf("m[carol@gmail.com].SlackID = %q, want U333", m["carol@gmail.com"].SlackID)
	}

	// Empty config
	empty := &PeopleConfig{}
	em := empty.BuildEmailMap()
	if len(em) != 0 {
		t.Errorf("BuildEmailMap() on empty config: len = %d, want 0", len(em))
	}
}

func TestFilterByType(t *testing.T) {
	cfg := &ConversationsConfig{
		Conversations: []ConversationConfig{
			{ID: "C1", Name: "ch1", Type: "channel", Mode: "api"},
			{ID: "D1", Name: "dm1", Type: "dm", Mode: "browser"},
			{ID: "C2", Name: "ch2", Type: "channel", Mode: "api"},
			{ID: "G1", Name: "grp", Type: "mpim", Mode: "browser"},
		},
	}

	channels := cfg.FilterByType("channel")
	if len(channels) != 2 {
		t.Fatalf("FilterByType(channel) = %d, want 2", len(channels))
	}
	if channels[0].ID != "C1" || channels[1].ID != "C2" {
		t.Errorf("FilterByType(channel) IDs = {%s, %s}, want {C1, C2}", channels[0].ID, channels[1].ID)
	}

	dms := cfg.FilterByType("dm")
	if len(dms) != 1 {
		t.Fatalf("FilterByType(dm) = %d, want 1", len(dms))
	}
	if dms[0].ID != "D1" {
		t.Errorf("FilterByType(dm) ID = %s, want D1", dms[0].ID)
	}

	// No matches
	none := cfg.FilterByType("private_channel")
	if len(none) != 0 {
		t.Errorf("FilterByType(private_channel) = %d, want 0", len(none))
	}
}

func TestFilterByMode(t *testing.T) {
	cfg := &ConversationsConfig{
		Conversations: []ConversationConfig{
			{ID: "C1", Name: "ch1", Type: "channel", Mode: "api"},
			{ID: "D1", Name: "dm1", Type: "dm", Mode: "browser"},
			{ID: "C2", Name: "ch2", Type: "channel", Mode: "api"},
			{ID: "G1", Name: "grp", Type: "mpim", Mode: "browser"},
		},
	}

	api := cfg.FilterByMode("api")
	if len(api) != 2 {
		t.Fatalf("FilterByMode(api) = %d, want 2", len(api))
	}
	if api[0].ID != "C1" || api[1].ID != "C2" {
		t.Errorf("FilterByMode(api) IDs = {%s, %s}, want {C1, C2}", api[0].ID, api[1].ID)
	}

	browser := cfg.FilterByMode("browser")
	if len(browser) != 2 {
		t.Fatalf("FilterByMode(browser) = %d, want 2", len(browser))
	}
	if browser[0].ID != "D1" || browser[1].ID != "G1" {
		t.Errorf("FilterByMode(browser) IDs = {%s, %s}, want {D1, G1}", browser[0].ID, browser[1].ID)
	}
}

func TestLoadConversations_ContractAssertions(t *testing.T) {
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

	if cfg == nil {
		t.Fatal("LoadConversations() returned nil config")
	}
	if len(cfg.Conversations) != 2 {
		t.Fatalf("len(Conversations) = %d, want 2", len(cfg.Conversations))
	}

	// Contract assertions on first conversation — all fields
	c0 := cfg.Conversations[0]
	if c0.ID != "C04KFBJTDJR" {
		t.Errorf("Conversations[0].ID = %q, want C04KFBJTDJR", c0.ID)
	}
	if c0.Name != "team-engineering" {
		t.Errorf("Conversations[0].Name = %q, want team-engineering", c0.Name)
	}
	if c0.Type != "channel" {
		t.Errorf("Conversations[0].Type = %q, want channel", c0.Type)
	}
	if c0.Mode != "api" {
		t.Errorf("Conversations[0].Mode = %q, want api", c0.Mode)
	}
	if !c0.Export {
		t.Error("Conversations[0].Export = false, want true")
	}
	if c0.Share {
		t.Error("Conversations[0].Share = true, want false")
	}

	// Contract assertions on second conversation — all fields
	c1 := cfg.Conversations[1]
	if c1.ID != "D06DDJ2UH2M" {
		t.Errorf("Conversations[1].ID = %q, want D06DDJ2UH2M", c1.ID)
	}
	if c1.Name != "John Smith" {
		t.Errorf("Conversations[1].Name = %q, want John Smith", c1.Name)
	}
	if c1.Type != "dm" {
		t.Errorf("Conversations[1].Type = %q, want dm", c1.Type)
	}
	if c1.Mode != "browser" {
		t.Errorf("Conversations[1].Mode = %q, want browser", c1.Mode)
	}
	if !c1.Export {
		t.Error("Conversations[1].Export = false, want true")
	}
	if !c1.Share {
		t.Error("Conversations[1].Share = false, want true")
	}

	// Test FilterByExport on loaded config
	exported := cfg.FilterByExport()
	if len(exported) != 2 {
		t.Errorf("FilterByExport() = %d, want 2", len(exported))
	}

	// Test GetByID on loaded config
	found := cfg.GetByID("D06DDJ2UH2M")
	if found == nil {
		t.Fatal("GetByID(D06DDJ2UH2M) returned nil")
	}
	if found.Name != "John Smith" {
		t.Errorf("GetByID(D06DDJ2UH2M).Name = %q, want John Smith", found.Name)
	}

	notFound := cfg.GetByID("NONEXISTENT")
	if notFound != nil {
		t.Error("GetByID(NONEXISTENT) should return nil")
	}
}

func TestDefaultSettings_ContractAssertions(t *testing.T) {
	s := DefaultSettings()
	if s == nil {
		t.Fatal("DefaultSettings() returned nil")
	}

	// Assert every field has its expected default zero/non-zero value
	if s.SlackBotToken != "" {
		t.Errorf("SlackBotToken = %q, want empty", s.SlackBotToken)
	}
	if s.GoogleCredentialsFile != "" {
		t.Errorf("GoogleCredentialsFile = %q, want empty", s.GoogleCredentialsFile)
	}
	if s.GoogleDriveFolderID != "" {
		t.Errorf("GoogleDriveFolderID = %q, want empty", s.GoogleDriveFolderID)
	}
	if s.FolderID != "" {
		t.Errorf("FolderID = %q, want empty", s.FolderID)
	}
	if s.LocalExportOutputDir != "" {
		t.Errorf("LocalExportOutputDir = %q, want empty", s.LocalExportOutputDir)
	}
	if s.LogLevel != "INFO" {
		t.Errorf("LogLevel = %q, want INFO", s.LogLevel)
	}
}
