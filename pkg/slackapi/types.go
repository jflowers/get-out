// Package slackapi provides a client for Slack's API supporting both
// browser-based (xoxc) and bot (xoxb) authentication modes.
package slackapi

import "time"

// Message represents a Slack message from the API.
type Message struct {
	Type        string       `json:"type"`
	User        string       `json:"user"`
	Text        string       `json:"text"`
	TS          string       `json:"ts"`
	ThreadTS    string       `json:"thread_ts,omitempty"`
	ReplyCount  int          `json:"reply_count,omitempty"`
	ReplyUsers  []string     `json:"reply_users,omitempty"`
	Reactions   []Reaction   `json:"reactions,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Files       []File       `json:"files,omitempty"`
	Edited      *Edited      `json:"edited,omitempty"`
	BotID       string       `json:"bot_id,omitempty"`
	Username    string       `json:"username,omitempty"`
	Subtype     string       `json:"subtype,omitempty"`
}

// Reaction represents an emoji reaction on a message.
type Reaction struct {
	Name  string   `json:"name"`
	Users []string `json:"users"`
	Count int      `json:"count"`
}

// Attachment represents a rich attachment (link unfurls, etc.).
type Attachment struct {
	ID            int    `json:"id"`
	Fallback      string `json:"fallback,omitempty"`
	Color         string `json:"color,omitempty"`
	Pretext       string `json:"pretext,omitempty"`
	AuthorName    string `json:"author_name,omitempty"`
	AuthorLink    string `json:"author_link,omitempty"`
	AuthorIcon    string `json:"author_icon,omitempty"`
	Title         string `json:"title,omitempty"`
	TitleLink     string `json:"title_link,omitempty"`
	Text          string `json:"text,omitempty"`
	ImageURL      string `json:"image_url,omitempty"`
	ThumbURL      string `json:"thumb_url,omitempty"`
	Footer        string `json:"footer,omitempty"`
	FooterIcon    string `json:"footer_icon,omitempty"`
	FromURL       string `json:"from_url,omitempty"`
	OriginalURL   string `json:"original_url,omitempty"`
	ServiceName   string `json:"service_name,omitempty"`
	ServiceIcon   string `json:"service_icon,omitempty"`
	MsgUnfurl     bool   `json:"msg_unfurl,omitempty"`
	IsReplyUnfurl bool   `json:"is_reply_unfurl,omitempty"`
	ChannelID     string `json:"channel_id,omitempty"`
	ChannelTeam   string `json:"channel_team,omitempty"`
}

// File represents an uploaded file.
type File struct {
	ID                 string `json:"id"`
	Created            int64  `json:"created"`
	Name               string `json:"name"`
	Title              string `json:"title"`
	Mimetype           string `json:"mimetype"`
	Filetype           string `json:"filetype"`
	PrettyType         string `json:"pretty_type"`
	User               string `json:"user"`
	Size               int64  `json:"size"`
	Mode               string `json:"mode"`
	IsExternal         bool   `json:"is_external"`
	ExternalType       string `json:"external_type"`
	IsPublic           bool   `json:"is_public"`
	PublicURLShared    bool   `json:"public_url_shared"`
	URLPrivate         string `json:"url_private"`
	URLPrivateDownload string `json:"url_private_download"`
	Permalink          string `json:"permalink"`
	PermalinkPublic    string `json:"permalink_public"`
}

// Edited contains information about message edits.
type Edited struct {
	User string `json:"user"`
	TS   string `json:"ts"`
}

// User represents a Slack user.
type User struct {
	ID                string      `json:"id"`
	TeamID            string      `json:"team_id"`
	Name              string      `json:"name"`
	Deleted           bool        `json:"deleted"`
	Color             string      `json:"color"`
	RealName          string      `json:"real_name"`
	TZ                string      `json:"tz"`
	TZLabel           string      `json:"tz_label"`
	TZOffset          int         `json:"tz_offset"`
	Profile           UserProfile `json:"profile"`
	IsAdmin           bool        `json:"is_admin"`
	IsOwner           bool        `json:"is_owner"`
	IsPrimaryOwner    bool        `json:"is_primary_owner"`
	IsRestricted      bool        `json:"is_restricted"`
	IsUltraRestricted bool        `json:"is_ultra_restricted"`
	IsBot             bool        `json:"is_bot"`
	IsAppUser         bool        `json:"is_app_user"`
	Updated           int64       `json:"updated"`
}

// UserProfile contains profile information for a user.
type UserProfile struct {
	Title                 string `json:"title"`
	Phone                 string `json:"phone"`
	Skype                 string `json:"skype"`
	RealName              string `json:"real_name"`
	RealNameNormalized    string `json:"real_name_normalized"`
	DisplayName           string `json:"display_name"`
	DisplayNameNormalized string `json:"display_name_normalized"`
	StatusText            string `json:"status_text"`
	StatusEmoji           string `json:"status_emoji"`
	Email                 string `json:"email"`
	ImageOriginal         string `json:"image_original"`
	Image24               string `json:"image_24"`
	Image32               string `json:"image_32"`
	Image48               string `json:"image_48"`
	Image72               string `json:"image_72"`
	Image192              string `json:"image_192"`
	Image512              string `json:"image_512"`
}

// GetDisplayName returns the best available display name for a user.
func (u *User) GetDisplayName() string {
	if u.Profile.DisplayName != "" {
		return u.Profile.DisplayName
	}
	if u.Profile.RealName != "" {
		return u.Profile.RealName
	}
	return u.Name
}

// Conversation represents a Slack conversation (channel, DM, group).
type Conversation struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	IsChannel          bool     `json:"is_channel"`
	IsGroup            bool     `json:"is_group"`
	IsIM               bool     `json:"is_im"`
	IsMPIM             bool     `json:"is_mpim"`
	IsPrivate          bool     `json:"is_private"`
	Created            int64    `json:"created"`
	IsArchived         bool     `json:"is_archived"`
	IsGeneral          bool     `json:"is_general"`
	Unlinked           int      `json:"unlinked"`
	NameNormalized     string   `json:"name_normalized"`
	IsShared           bool     `json:"is_shared"`
	IsOrgShared        bool     `json:"is_org_shared"`
	IsPendingExtShared bool     `json:"is_pending_ext_shared"`
	IsMember           bool     `json:"is_member"`
	Topic              Topic    `json:"topic"`
	Purpose            Purpose  `json:"purpose"`
	NumMembers         int      `json:"num_members"`
	User               string   `json:"user,omitempty"` // For DMs
	Members            []string `json:"members,omitempty"`
}

// Topic represents a channel topic.
type Topic struct {
	Value   string `json:"value"`
	Creator string `json:"creator"`
	LastSet int64  `json:"last_set"`
}

// Purpose represents a channel purpose/description.
type Purpose struct {
	Value   string `json:"value"`
	Creator string `json:"creator"`
	LastSet int64  `json:"last_set"`
}

// ResponseMetadata contains pagination information.
type ResponseMetadata struct {
	NextCursor string `json:"next_cursor"`
}

// HistoryResponse is the response from conversations.history.
type HistoryResponse struct {
	OK               bool             `json:"ok"`
	Error            string           `json:"error,omitempty"`
	Messages         []Message        `json:"messages"`
	HasMore          bool             `json:"has_more"`
	ResponseMetadata ResponseMetadata `json:"response_metadata"`
}

// RepliesResponse is the response from conversations.replies.
type RepliesResponse struct {
	OK               bool             `json:"ok"`
	Error            string           `json:"error,omitempty"`
	Messages         []Message        `json:"messages"`
	HasMore          bool             `json:"has_more"`
	ResponseMetadata ResponseMetadata `json:"response_metadata"`
}

// ConversationsListResponse is the response from conversations.list.
type ConversationsListResponse struct {
	OK               bool             `json:"ok"`
	Error            string           `json:"error,omitempty"`
	Channels         []Conversation   `json:"channels"`
	ResponseMetadata ResponseMetadata `json:"response_metadata"`
}

// UserInfoResponse is the response from users.info.
type UserInfoResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	User  User   `json:"user"`
}

// UsersListResponse is the response from users.list.
type UsersListResponse struct {
	OK               bool             `json:"ok"`
	Error            string           `json:"error,omitempty"`
	Members          []User           `json:"members"`
	ResponseMetadata ResponseMetadata `json:"response_metadata"`
}

// ConversationInfoResponse is the response from conversations.info.
type ConversationInfoResponse struct {
	OK      bool         `json:"ok"`
	Error   string       `json:"error,omitempty"`
	Channel Conversation `json:"channel"`
}

// MembersResponse is the response from conversations.members.
type MembersResponse struct {
	OK               bool             `json:"ok"`
	Error            string           `json:"error,omitempty"`
	Members          []string         `json:"members"`
	ResponseMetadata ResponseMetadata `json:"response_metadata"`
}

// TSToTime converts a Slack timestamp to time.Time.
func TSToTime(ts string) time.Time {
	// Slack timestamps are Unix seconds with microseconds: "1234567890.123456"
	var sec, usec int64
	for i := 0; i < len(ts); i++ {
		if ts[i] == '.' {
			sec = parseInt(ts[:i])
			usec = parseInt(ts[i+1:])
			break
		}
	}
	if sec == 0 {
		sec = parseInt(ts)
	}
	return time.Unix(sec, usec*1000)
}

func parseInt(s string) int64 {
	var n int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int64(c-'0')
		}
	}
	return n
}
