// Package slackapi provides Slack API client functionality using session tokens.
package slackapi

import "time"

// Conversation represents a Slack conversation (DM, group, channel).
type Conversation struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	IsIM        bool     `json:"is_im"`
	IsMPIM      bool     `json:"is_mpim"`
	IsPrivate   bool     `json:"is_private"`
	IsChannel   bool     `json:"is_channel"`
	IsGroup     bool     `json:"is_group"`
	User        string   `json:"user"` // For DMs, the other user's ID
	NumMembers  int      `json:"num_members"`
	Purpose     Purpose  `json:"purpose"`
	Topic       Topic    `json:"topic"`
	LastRead    string   `json:"last_read"`
	Latest      *Message `json:"latest"`
	UnreadCount int      `json:"unread_count"`
}

// Purpose holds channel purpose info.
type Purpose struct {
	Value string `json:"value"`
}

// Topic holds channel topic info.
type Topic struct {
	Value string `json:"value"`
}

// Message represents a Slack message.
type Message struct {
	Type        string       `json:"type"`
	Subtype     string       `json:"subtype,omitempty"`
	User        string       `json:"user"`
	Text        string       `json:"text"`
	Timestamp   string       `json:"ts"`
	ThreadTS    string       `json:"thread_ts,omitempty"`
	ReplyCount  int          `json:"reply_count,omitempty"`
	Reactions   []Reaction   `json:"reactions,omitempty"`
	Files       []File       `json:"files,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Edited      *Edited      `json:"edited,omitempty"`
	BotID       string       `json:"bot_id,omitempty"`
	Username    string       `json:"username,omitempty"`
}

// ParsedTime converts the Slack timestamp to a Go time.
func (m *Message) ParsedTime() time.Time {
	if m.Timestamp == "" {
		return time.Time{}
	}
	// Slack timestamps are Unix epoch with microseconds after decimal
	var sec, usec int64
	n, _ := parseTimestamp(m.Timestamp, &sec, &usec)
	if n >= 1 {
		return time.Unix(sec, usec*1000)
	}
	return time.Time{}
}

func parseTimestamp(ts string, sec, usec *int64) (int, error) {
	var s, u int64
	n := 0
	for i, c := range ts {
		if c == '.' {
			*sec = s
			for _, c := range ts[i+1:] {
				if c >= '0' && c <= '9' {
					u = u*10 + int64(c-'0')
					n++
				}
			}
			*usec = u
			return 2, nil
		}
		if c >= '0' && c <= '9' {
			s = s*10 + int64(c-'0')
		}
	}
	*sec = s
	return 1, nil
}

// Reaction represents an emoji reaction.
type Reaction struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Users []string `json:"users"`
}

// File represents a shared file.
type File struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Title      string `json:"title"`
	Mimetype   string `json:"mimetype"`
	Filetype   string `json:"filetype"`
	URLPrivate string `json:"url_private"`
	Permalink  string `json:"permalink"`
}

// Attachment represents a message attachment.
type Attachment struct {
	Title     string `json:"title"`
	TitleLink string `json:"title_link"`
	Text      string `json:"text"`
	Fallback  string `json:"fallback"`
	Color     string `json:"color"`
	ImageURL  string `json:"image_url"`
	ThumbURL  string `json:"thumb_url"`
}

// Edited holds edit metadata.
type Edited struct {
	User string `json:"user"`
	TS   string `json:"ts"`
}

// User represents a Slack user.
type User struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	RealName       string          `json:"real_name"`
	DisplayName    string          `json:"display_name"`
	Email          string          `json:"email,omitempty"`
	IsBot          bool            `json:"is_bot"`
	IsAdmin        bool            `json:"is_admin"`
	IsOwner        bool            `json:"is_owner"`
	Deleted        bool            `json:"deleted"`
	Profile        Profile         `json:"profile"`
	TeamID         string          `json:"team_id"`
	EnterpriseUser *EnterpriseUser `json:"enterprise_user,omitempty"`
}

// Profile holds user profile information.
type Profile struct {
	RealName    string `json:"real_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Image24     string `json:"image_24"`
	Image48     string `json:"image_48"`
	Image72     string `json:"image_72"`
	StatusText  string `json:"status_text"`
	StatusEmoji string `json:"status_emoji"`
}

// EnterpriseUser holds enterprise-specific user data.
type EnterpriseUser struct {
	ID             string `json:"id"`
	EnterpriseID   string `json:"enterprise_id"`
	EnterpriseName string `json:"enterprise_name"`
}

// APIError represents a Slack API error response.
type APIError struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Warning  string `json:"warning,omitempty"`
	Needed   string `json:"needed,omitempty"`   // For missing_scope errors
	Provided string `json:"provided,omitempty"` // For missing_scope errors
}

func (e *APIError) String() string {
	if e.Needed != "" {
		return e.Error + " (needed: " + e.Needed + ", provided: " + e.Provided + ")"
	}
	return e.Error
}

// IsEnterpriseRestriction returns true if the error is due to enterprise restrictions.
func (e *APIError) IsEnterpriseRestriction() bool {
	switch e.Error {
	case "restricted_action",
		"enterprise_is_restricted",
		"team_access_not_granted",
		"missing_scope",
		"not_allowed_token_type",
		"access_denied",
		"org_login_required":
		return true
	}
	return false
}

// IsRateLimited returns true if the error is a rate limit.
func (e *APIError) IsRateLimited() bool {
	return e.Error == "ratelimited"
}

// ConversationListResponse is the response from conversations.list.
type ConversationListResponse struct {
	OK               bool           `json:"ok"`
	Error            string         `json:"error,omitempty"`
	Channels         []Conversation `json:"channels"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// ConversationHistoryResponse is the response from conversations.history.
type ConversationHistoryResponse struct {
	OK               bool      `json:"ok"`
	Error            string    `json:"error,omitempty"`
	Messages         []Message `json:"messages"`
	HasMore          bool      `json:"has_more"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// ConversationRepliesResponse is the response from conversations.replies.
type ConversationRepliesResponse struct {
	OK               bool      `json:"ok"`
	Error            string    `json:"error,omitempty"`
	Messages         []Message `json:"messages"`
	HasMore          bool      `json:"has_more"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// UserInfoResponse is the response from users.info.
type UserInfoResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	User  User   `json:"user"`
}

// UsersListResponse is the response from users.list.
type UsersListResponse struct {
	OK               bool   `json:"ok"`
	Error            string `json:"error,omitempty"`
	Members          []User `json:"members"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}
