package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/jflowers/get-out/pkg/models"
)

var (
	// Slack ID patterns
	conversationIDPattern = regexp.MustCompile(`^[CDGW][A-Z0-9]+$`)
	userIDPattern         = regexp.MustCompile(`^U[A-Z0-9]+$`)
	emailPattern          = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

// LoadSettings loads settings.json from the config directory.
// Returns default settings if the file doesn't exist.
func LoadSettings(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// settings.json is optional, return defaults
			return DefaultSettings(), nil
		}
		return nil, fmt.Errorf("failed to read settings: %w", err)
	}

	settings := DefaultSettings()
	if err := json.Unmarshal(data, settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings: %w", err)
	}

	return settings, nil
}

// LoadConversations loads and validates conversations.json.
func LoadConversations(path string) (*ConversationsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read conversations config: %w", err)
	}

	var cfg ConversationsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse conversations config: %w", err)
	}

	// Set defaults and validate
	for i := range cfg.Conversations {
		conv := &cfg.Conversations[i]

		// Default export to true if not specified
		// Note: Go's zero value for bool is false, so we can't distinguish
		// between explicitly set false and not set. The JSON should always include it.

		if err := validateConversationConfig(conv); err != nil {
			return nil, fmt.Errorf("invalid conversation config at index %d: %w", i, err)
		}
	}

	return &cfg, nil
}

// LoadPeople loads and validates people.json.
func LoadPeople(path string) (*PeopleConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// people.json is optional
		if os.IsNotExist(err) {
			return &PeopleConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read people config: %w", err)
	}

	var cfg PeopleConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse people config: %w", err)
	}

	for i := range cfg.People {
		person := &cfg.People[i]
		if err := validatePersonConfig(person); err != nil {
			return nil, fmt.Errorf("invalid person config at index %d: %w", i, err)
		}
	}

	return &cfg, nil
}

// validateConversationConfig validates a single conversation config entry.
func validateConversationConfig(c *ConversationConfig) error {
	if c.ID == "" {
		return fmt.Errorf("id is required")
	}
	if !conversationIDPattern.MatchString(c.ID) {
		return fmt.Errorf("invalid conversation id format: %s", c.ID)
	}
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !isValidConversationType(c.Type) {
		return fmt.Errorf("invalid type: %s", c.Type)
	}
	if !isValidExportMode(c.Mode) {
		return fmt.Errorf("invalid mode: %s", c.Mode)
	}
	return nil
}

// validatePersonConfig validates a single person config entry.
func validatePersonConfig(p *PersonConfig) error {
	if p.SlackID == "" {
		return fmt.Errorf("slackId is required")
	}
	if !userIDPattern.MatchString(p.SlackID) {
		return fmt.Errorf("invalid slackId format: %s", p.SlackID)
	}
	if p.Email != "" && !emailPattern.MatchString(p.Email) {
		return fmt.Errorf("invalid email format: %s", p.Email)
	}
	if p.GoogleEmail != "" && !emailPattern.MatchString(p.GoogleEmail) {
		return fmt.Errorf("invalid googleEmail format: %s", p.GoogleEmail)
	}
	return nil
}

func isValidConversationType(t models.ConversationType) bool {
	switch t {
	case models.ConversationTypeDM,
		models.ConversationTypeMPIM,
		models.ConversationTypeChannel,
		models.ConversationTypePrivateChannel:
		return true
	}
	return false
}

func isValidExportMode(m models.ExportMode) bool {
	switch m {
	case models.ExportModeAPI, models.ExportModeBrowser:
		return true
	}
	return false
}

// FilterByExport returns only conversations where export=true.
func (c *ConversationsConfig) FilterByExport() []ConversationConfig {
	var result []ConversationConfig
	for _, conv := range c.Conversations {
		if conv.Export {
			result = append(result, conv)
		}
	}
	return result
}

// FilterByMode returns conversations matching the given export mode.
func (c *ConversationsConfig) FilterByMode(mode models.ExportMode) []ConversationConfig {
	var result []ConversationConfig
	for _, conv := range c.Conversations {
		if conv.Mode == mode {
			result = append(result, conv)
		}
	}
	return result
}

// FilterByType returns conversations matching the given type.
func (c *ConversationsConfig) FilterByType(t models.ConversationType) []ConversationConfig {
	var result []ConversationConfig
	for _, conv := range c.Conversations {
		if conv.Type == t {
			result = append(result, conv)
		}
	}
	return result
}

// GetByID returns a conversation config by ID, or nil if not found.
func (c *ConversationsConfig) GetByID(id string) *ConversationConfig {
	for i := range c.Conversations {
		if c.Conversations[i].ID == id {
			return &c.Conversations[i]
		}
	}
	return nil
}

// BuildUserMap creates a map from Slack ID to PersonConfig for quick lookups.
func (p *PeopleConfig) BuildUserMap() map[string]*PersonConfig {
	m := make(map[string]*PersonConfig, len(p.People))
	for i := range p.People {
		m[p.People[i].SlackID] = &p.People[i]
	}
	return m
}

// BuildEmailMap creates a map from email to PersonConfig for sharing lookups.
func (p *PeopleConfig) BuildEmailMap() map[string]*PersonConfig {
	m := make(map[string]*PersonConfig)
	for i := range p.People {
		if p.People[i].Email != "" {
			m[p.People[i].Email] = &p.People[i]
		}
		if p.People[i].GoogleEmail != "" {
			m[p.People[i].GoogleEmail] = &p.People[i]
		}
	}
	return m
}
