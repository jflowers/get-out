// Package config handles configuration file loading and validation.
package config

import "github.com/jflowers/get-out/pkg/models"

// Settings is the root structure for settings.json.
// It contains application-wide configuration options.
type Settings struct {
	// Google Drive configuration
	GoogleCredentialsFile string `json:"googleCredentialsFile,omitempty"`
	GoogleDriveFolderID   string `json:"googleDriveFolderId,omitempty"`

	// FolderID is the Google Drive folder ID used by default for exports.
	// Set via `get-out init` or directly in settings.json.
	FolderID string `json:"folder_id,omitempty"`

	// Local export configuration (for future use)
	LocalExportOutputDir string `json:"localExportOutputDir,omitempty"`

	// Slack configuration
	SlackWorkspaceURL string `json:"slackWorkspaceUrl,omitempty"`

	// Logging
	LogLevel string `json:"logLevel,omitempty"`
}

// DefaultSettings returns settings with default values.
func DefaultSettings() *Settings {
	return &Settings{
		SlackWorkspaceURL: "https://app.slack.com",
		LogLevel:          "INFO",
	}
}

// ConversationsConfig is the root structure for conversations.json.
type ConversationsConfig struct {
	Conversations []ConversationConfig `json:"conversations"`
}

// ConversationConfig defines a single conversation to export.
type ConversationConfig struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Type         models.ConversationType `json:"type"`
	Export       bool                    `json:"export"`
	LocalExport  bool                    `json:"localExport,omitempty"`
	Share        bool                    `json:"share"`
	ShareMembers []string                `json:"shareMembers,omitempty"`
}

// PeopleConfig is the root structure for people.json.
type PeopleConfig struct {
	People []PersonConfig `json:"people"`
}

// PersonConfig defines a person's Slack-to-Google mapping.
type PersonConfig struct {
	SlackID         string `json:"slackId"`
	Email           string `json:"email,omitempty"`
	DisplayName     string `json:"displayName,omitempty"`
	GoogleEmail     string `json:"googleEmail,omitempty"`
	NoNotifications bool   `json:"noNotifications,omitempty"`
	NoShare         bool   `json:"noShare,omitempty"`
}
