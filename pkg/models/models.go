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

// All conversations are exported using browser session tokens (xoxc-) via Chrome CDP.
// The ExportMode type was removed — there is no longer a distinction between
// API and browser export modes.
