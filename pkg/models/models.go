// Package models defines the domain types for Slack message export.
package models

import "time"

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
	ExportModeAPI     ExportMode = "api"     // Use Slack bot token (xoxb-)
	ExportModeBrowser ExportMode = "browser" // Use browser session token (xoxc-)
)

// Conversation represents a DM, group, or channel.
type Conversation struct {
	ID           string           `json:"id"`
	Type         ConversationType `json:"type"`
	Name         string           `json:"name"`
	Participants []string         `json:"participants"`
	Created      time.Time        `json:"created"`
	IsArchived   bool             `json:"is_archived"`
}

// Message represents a single Slack message.
type Message struct {
	TS             string       `json:"ts"`
	ConversationID string       `json:"conversation_id"`
	UserID         string       `json:"user"`
	Text           string       `json:"text"`
	ThreadTS       string       `json:"thread_ts,omitempty"`
	ReplyCount     int          `json:"reply_count,omitempty"`
	Reactions      []Reaction   `json:"reactions,omitempty"`
	Attachments    []Attachment `json:"attachments,omitempty"`
	Edited         *EditInfo    `json:"edited,omitempty"`
	Files          []File       `json:"files,omitempty"`
}

// User represents a Slack workspace member.
type User struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RealName    string `json:"real_name"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	IsBot       bool   `json:"is_bot"`
	Deleted     bool   `json:"deleted"`
}

// GetDisplayName returns the best available display name for a user.
func (u *User) GetDisplayName() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.RealName != "" {
		return u.RealName
	}
	return u.Name
}

// Reaction represents an emoji reaction on a message.
type Reaction struct {
	Name  string   `json:"name"`
	Users []string `json:"users"`
	Count int      `json:"count"`
}

// Attachment represents a rich attachment (unfurls, etc.).
type Attachment struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	TitleLink  string `json:"title_link,omitempty"`
	Text       string `json:"text,omitempty"`
	Fallback   string `json:"fallback,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
	ThumbURL   string `json:"thumb_url,omitempty"`
	AuthorName string `json:"author_name,omitempty"`
	Color      string `json:"color,omitempty"`
}

// File represents an uploaded file.
type File struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	MimeType string `json:"mimetype"`
	Size     int64  `json:"size"`
	URL      string `json:"url_private"`
}

// EditInfo contains information about message edits.
type EditInfo struct {
	User string `json:"user"`
	TS   string `json:"ts"`
}

// ExportStatus represents the state of an export operation.
type ExportStatus string

const (
	ExportStatusPending    ExportStatus = "pending"
	ExportStatusInProgress ExportStatus = "in_progress"
	ExportStatusCompleted  ExportStatus = "completed"
	ExportStatusFailed     ExportStatus = "failed"
	ExportStatusPaused     ExportStatus = "paused"
)

// ExportSession tracks the state of an export operation.
type ExportSession struct {
	ID               string       `json:"id"`
	ConversationID   string       `json:"conversation_id"`
	Status           ExportStatus `json:"status"`
	LastTS           string       `json:"last_ts"`
	MessagesExported int          `json:"messages_exported"`
	StartedAt        time.Time    `json:"started_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	Error            string       `json:"error,omitempty"`
	OutputPath       string       `json:"output_path"`
}

// UserMapping maps a Slack user to a Google account.
type UserMapping struct {
	SlackID         string `json:"slackId"`
	Email           string `json:"email,omitempty"`
	DisplayName     string `json:"displayName,omitempty"`
	GoogleEmail     string `json:"googleEmail,omitempty"`
	NoNotifications bool   `json:"noNotifications,omitempty"`
	NoShare         bool   `json:"noShare,omitempty"`
}

// ExportIndex tracks all exported content for link resolution.
type ExportIndex struct {
	Conversations map[string]ConversationExport `json:"conversations"`
	UpdatedAt     time.Time                     `json:"updated_at"`
}

// ConversationExport tracks an exported conversation.
type ConversationExport struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	FolderID    string                  `json:"folder_id"`
	FolderURL   string                  `json:"folder_url"`
	DailyDocs   map[string]DocExport    `json:"daily_docs"`  // date -> doc
	ThreadDocs  map[string]ThreadExport `json:"thread_docs"` // thread_ts -> thread folder
	LastUpdated time.Time               `json:"last_updated"`
}

// DocExport tracks an exported Google Doc.
type DocExport struct {
	DocID  string `json:"doc_id"`
	DocURL string `json:"doc_url"`
	Date   string `json:"date"`
	Title  string `json:"title"`
}

// ThreadExport tracks an exported thread folder.
type ThreadExport struct {
	ThreadTS   string               `json:"thread_ts"`
	FolderID   string               `json:"folder_id"`
	FolderURL  string               `json:"folder_url"`
	FolderName string               `json:"folder_name"`
	DailyDocs  map[string]DocExport `json:"daily_docs"` // date -> doc
}
