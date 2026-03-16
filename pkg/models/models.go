// Package models defines the shared domain types for Slack message export.
// Only types that are referenced across multiple packages live here.
// Package-specific types (e.g., export index state) live in their own packages.
package models

// ConversationType represents the type of Slack conversation.
type ConversationType string

const (
	ConversationTypeDM             ConversationType = "dm"
	ConversationTypeMPIM           ConversationType = "mpim"
	ConversationTypeChannel        ConversationType = "channel"
	ConversationTypePrivateChannel ConversationType = "private_channel"
)

// ExportMode represents how to access a conversation's messages.
type ExportMode string

const (
	ExportModeAPI     ExportMode = "api"     // Legacy: all exports now use browser mode
	ExportModeBrowser ExportMode = "browser" // Use browser session token (xoxc-)
)
