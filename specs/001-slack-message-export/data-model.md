# Data Model: Slack Message Export

**Feature**: 001-slack-message-export  
**Date**: 2026-02-03

## Core Entities

### Conversation

Represents a DM or group thread container.

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Slack channel/conversation ID (e.g., `D123ABC456`, `C789DEF012`) |
| `type` | enum | `dm`, `mpim` (multi-party IM), `channel`, `private_channel` |
| `name` | string | Display name (user name for DMs, channel name for groups) |
| `participants` | []string | List of user IDs in the conversation |
| `created` | timestamp | When the conversation was created |
| `is_archived` | bool | Whether the conversation is archived |

**Relationships**:
- Has many `Message` entities
- References multiple `User` entities via `participants`

### Message

A single communication unit within a conversation.

| Field | Type | Description |
|-------|------|-------------|
| `ts` | string | Slack timestamp (unique ID, e.g., `1704067200.000100`) |
| `conversation_id` | string | Parent conversation ID |
| `user_id` | string | Sender's user ID |
| `text` | string | Message content in mrkdwn format |
| `thread_ts` | string? | Parent message ts (if this is a reply) |
| `reply_count` | int | Number of replies (if this is a thread parent) |
| `reactions` | []Reaction | Emoji reactions on the message |
| `attachments` | []Attachment | File attachments or rich previews |
| `edited` | EditInfo? | Edit timestamp and user if edited |

**Relationships**:
- Belongs to one `Conversation`
- References one `User` as sender
- May have many child `Message` entities (thread replies)
- May have many `Reaction` entities

### User

A Slack workspace member.

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Slack user ID (e.g., `U123ABC456`) |
| `name` | string | Username (handle) |
| `real_name` | string | Display name |
| `display_name` | string | Preferred display name |
| `avatar_url` | string | Profile picture URL |
| `is_bot` | bool | Whether this is a bot user |
| `deleted` | bool | Whether the user has been deactivated |

**Relationships**:
- May be sender of many `Message` entities
- May participate in many `Conversation` entities

### Reaction

An emoji reaction on a message.

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Emoji shortcode (e.g., `thumbsup`, `heart`) |
| `users` | []string | User IDs who added this reaction |
| `count` | int | Total reaction count |

### Attachment

A file or rich media attachment.

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Attachment ID |
| `type` | string | Attachment type (file, image, video, etc.) |
| `title` | string | Display title |
| `url` | string | Download/view URL |
| `mimetype` | string | MIME type |
| `size` | int | File size in bytes |

### ExportSession

Tracks the state of an export operation for resumability.

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique session ID (UUID) |
| `conversation_id` | string | Target conversation being exported |
| `status` | enum | `pending`, `in_progress`, `completed`, `failed`, `paused` |
| `last_ts` | string | Last successfully processed message timestamp |
| `messages_exported` | int | Count of messages exported so far |
| `started_at` | timestamp | When the export began |
| `updated_at` | timestamp | Last checkpoint update |
| `error` | string? | Error message if failed |
| `output_path` | string | Directory where export is being written |

**State Transitions**:
```
pending → in_progress → completed
                     ↘ failed
                     ↘ paused → in_progress (resume)
```

## Value Objects

### EditInfo

| Field | Type | Description |
|-------|------|-------------|
| `user` | string | User ID who edited |
| `ts` | string | Timestamp of edit |

### Cursor

Pagination state for API requests.

| Field | Type | Description |
|-------|------|-------------|
| `next_cursor` | string | Opaque cursor for next page |
| `has_more` | bool | Whether more results exist |

## Go Type Definitions

```go
package models

import "time"

type ConversationType string

const (
    ConversationTypeDM             ConversationType = "dm"
    ConversationTypeMPIM           ConversationType = "mpim"
    ConversationTypeChannel        ConversationType = "channel"
    ConversationTypePrivateChannel ConversationType = "private_channel"
)

type Conversation struct {
    ID           string           `json:"id"`
    Type         ConversationType `json:"type"`
    Name         string           `json:"name"`
    Participants []string         `json:"participants"`
    Created      time.Time        `json:"created"`
    IsArchived   bool             `json:"is_archived"`
}

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
}

type User struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    RealName    string `json:"real_name"`
    DisplayName string `json:"display_name"`
    AvatarURL   string `json:"avatar_url"`
    IsBot       bool   `json:"is_bot"`
    Deleted     bool   `json:"deleted"`
}

type Reaction struct {
    Name  string   `json:"name"`
    Users []string `json:"users"`
    Count int      `json:"count"`
}

type Attachment struct {
    ID       string `json:"id"`
    Type     string `json:"type"`
    Title    string `json:"title"`
    URL      string `json:"url"`
    MimeType string `json:"mimetype"`
    Size     int    `json:"size"`
}

type EditInfo struct {
    User string `json:"user"`
    TS   string `json:"ts"`
}

type ExportStatus string

const (
    ExportStatusPending    ExportStatus = "pending"
    ExportStatusInProgress ExportStatus = "in_progress"
    ExportStatusCompleted  ExportStatus = "completed"
    ExportStatusFailed     ExportStatus = "failed"
    ExportStatusPaused     ExportStatus = "paused"
)

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
```

## Validation Rules

### Conversation
- `id` must match pattern `^[CDGW][A-Z0-9]+$`
- `type` must be a valid ConversationType
- `participants` must contain at least 1 user ID for DMs, 2+ for groups

### Message
- `ts` must be a valid Slack timestamp (numeric with decimal)
- `user_id` must match pattern `^[UWB][A-Z0-9]+$`
- `thread_ts` if present must be <= `ts` (parent comes before reply)

### User
- `id` must match pattern `^[UWB][A-Z0-9]+$`
- `real_name` or `display_name` must be non-empty (for resolution)

### ExportSession
- `last_ts` must be valid timestamp when status is `in_progress` or `paused`
- `output_path` must be a valid directory path
