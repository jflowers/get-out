// Package config handles configuration file loading and validation.
package config

import "github.com/jflowers/get-out/pkg/models"

// ConversationsConfig is the root structure for conversations.json.
type ConversationsConfig struct {
	Conversations []ConversationConfig `json:"conversations"`
}

// ConversationConfig defines a single conversation to export.
type ConversationConfig struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Type         models.ConversationType `json:"type"`
	Mode         models.ExportMode       `json:"mode"`
	Export       bool                    `json:"export"`
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
