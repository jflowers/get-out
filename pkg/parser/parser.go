// Package parser converts Slack messages to Markdown format.
package parser

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jflowers/get-out/pkg/slackapi"
)

// Parser converts Slack messages to Markdown.
type Parser struct {
	userMap    map[string]string // User ID -> Display name
	channelMap map[string]string // Channel ID -> Channel name
}

// NewParser creates a new message parser.
func NewParser(userMap, channelMap map[string]string) *Parser {
	if userMap == nil {
		userMap = make(map[string]string)
	}
	if channelMap == nil {
		channelMap = make(map[string]string)
	}
	return &Parser{
		userMap:    userMap,
		channelMap: channelMap,
	}
}

// SetUserMap updates the user ID to name mapping.
func (p *Parser) SetUserMap(userMap map[string]string) {
	p.userMap = userMap
}

// SetChannelMap updates the channel ID to name mapping.
func (p *Parser) SetChannelMap(channelMap map[string]string) {
	p.channelMap = channelMap
}

// MessageToMarkdown converts a single message to Markdown.
func (p *Parser) MessageToMarkdown(msg *slackapi.Message) string {
	var sb strings.Builder

	// Get author name
	author := p.resolveUser(msg.User)
	if msg.Username != "" {
		author = msg.Username // Bot messages use username
	}
	if author == "" {
		author = "Unknown"
	}

	// Get timestamp
	ts := msg.ParsedTime()
	if ts.IsZero() {
		ts = time.Now()
	}
	timestamp := ts.Format("2006-01-02 15:04:05")

	// Format header
	sb.WriteString(fmt.Sprintf("**%s** _%s_", author, timestamp))

	// Add thread indicator
	if msg.ThreadTS != "" && msg.ThreadTS != msg.Timestamp {
		sb.WriteString(" (in thread)")
	}
	sb.WriteString("\n\n")

	// Convert message text
	text := p.convertMrkdwn(msg.Text)
	sb.WriteString(text)
	sb.WriteString("\n")

	// Add reactions
	if len(msg.Reactions) > 0 {
		sb.WriteString("\n")
		for _, reaction := range msg.Reactions {
			sb.WriteString(fmt.Sprintf(":%s: ×%d  ", reaction.Name, reaction.Count))
		}
		sb.WriteString("\n")
	}

	// Add attachments
	for _, att := range msg.Attachments {
		sb.WriteString("\n")
		if att.Title != "" {
			if att.TitleLink != "" {
				sb.WriteString(fmt.Sprintf("> **[%s](%s)**\n", att.Title, att.TitleLink))
			} else {
				sb.WriteString(fmt.Sprintf("> **%s**\n", att.Title))
			}
		}
		if att.Text != "" {
			// Indent attachment text
			for _, line := range strings.Split(att.Text, "\n") {
				sb.WriteString(fmt.Sprintf("> %s\n", line))
			}
		}
	}

	// Add files
	for _, file := range msg.Files {
		sb.WriteString(fmt.Sprintf("\n📎 [%s](%s)\n", file.Name, file.Permalink))
	}

	// Add edit indicator
	if msg.Edited != nil {
		sb.WriteString("\n_(edited)_\n")
	}

	return sb.String()
}

// ConversationToMarkdown converts a full conversation to Markdown.
func (p *Parser) ConversationToMarkdown(conv *slackapi.Conversation, messages []slackapi.Message) string {
	var sb strings.Builder

	// Header
	title := conv.Name
	if conv.IsIM {
		title = "Direct Message with " + p.resolveUser(conv.User)
	} else if conv.IsMPIM {
		title = "Group Message: " + conv.Name
	}

	sb.WriteString(fmt.Sprintf("# %s\n\n", title))

	if conv.Purpose.Value != "" {
		sb.WriteString(fmt.Sprintf("**Purpose:** %s\n\n", conv.Purpose.Value))
	}

	sb.WriteString("---\n\n")

	// Group messages by thread
	threads := make(map[string][]slackapi.Message)
	var rootMessages []slackapi.Message

	for _, msg := range messages {
		if msg.ThreadTS != "" && msg.ThreadTS != msg.Timestamp {
			// This is a reply
			threads[msg.ThreadTS] = append(threads[msg.ThreadTS], msg)
		} else {
			rootMessages = append(rootMessages, msg)
		}
	}

	// Output messages with threads inline
	for _, msg := range rootMessages {
		sb.WriteString(p.MessageToMarkdown(&msg))
		sb.WriteString("\n")

		// Add thread replies if any
		if replies, ok := threads[msg.Timestamp]; ok {
			sb.WriteString("<details>\n<summary>Thread replies</summary>\n\n")
			for _, reply := range replies {
				// Indent thread replies
				replyMd := p.MessageToMarkdown(&reply)
				for _, line := range strings.Split(replyMd, "\n") {
					sb.WriteString("> " + line + "\n")
				}
			}
			sb.WriteString("\n</details>\n\n")
		}

		sb.WriteString("---\n\n")
	}

	return sb.String()
}

// convertMrkdwn converts Slack's mrkdwn format to Markdown.
func (p *Parser) convertMrkdwn(text string) string {
	if text == "" {
		return ""
	}

	result := text

	// Convert user mentions: <@U12345> -> @username
	userMentionRe := regexp.MustCompile(`<@(U[A-Z0-9]+)(?:\|([^>]+))?>`)
	result = userMentionRe.ReplaceAllStringFunc(result, func(match string) string {
		matches := userMentionRe.FindStringSubmatch(match)
		if len(matches) >= 2 {
			userID := matches[1]
			// If display name provided in mention, use it
			if len(matches) >= 3 && matches[2] != "" {
				return "**@" + matches[2] + "**"
			}
			// Otherwise resolve from user map
			if name, ok := p.userMap[userID]; ok {
				return "**@" + name + "**"
			}
			return "**@" + userID + "**"
		}
		return match
	})

	// Convert channel mentions: <#C12345|channel-name> -> #channel-name
	channelMentionRe := regexp.MustCompile(`<#(C[A-Z0-9]+)(?:\|([^>]+))?>`)
	result = channelMentionRe.ReplaceAllStringFunc(result, func(match string) string {
		matches := channelMentionRe.FindStringSubmatch(match)
		if len(matches) >= 3 && matches[2] != "" {
			return "**#" + matches[2] + "**"
		}
		if len(matches) >= 2 {
			channelID := matches[1]
			if name, ok := p.channelMap[channelID]; ok {
				return "**#" + name + "**"
			}
			return "**#" + channelID + "**"
		}
		return match
	})

	// Convert links: <https://example.com|text> -> [text](https://example.com)
	linkWithTextRe := regexp.MustCompile(`<(https?://[^|>]+)\|([^>]+)>`)
	result = linkWithTextRe.ReplaceAllString(result, "[$2]($1)")

	// Convert plain links: <https://example.com> -> https://example.com
	plainLinkRe := regexp.MustCompile(`<(https?://[^>]+)>`)
	result = plainLinkRe.ReplaceAllString(result, "$1")

	// Convert special mentions
	result = strings.ReplaceAll(result, "<!here>", "**@here**")
	result = strings.ReplaceAll(result, "<!channel>", "**@channel**")
	result = strings.ReplaceAll(result, "<!everyone>", "**@everyone**")

	// Convert special mentions with labels
	specialMentionRe := regexp.MustCompile(`<!([a-z]+)\|([^>]+)>`)
	result = specialMentionRe.ReplaceAllString(result, "**@$2**")

	// Convert bold: *text* -> **text**
	// Be careful not to double-convert
	boldRe := regexp.MustCompile(`(?:^|[^*])\*([^*\n]+)\*(?:[^*]|$)`)
	// Slack uses single asterisks for bold, Markdown uses double
	result = strings.ReplaceAll(result, "*", "**")
	// But we doubled them, so fix triple asterisks from original doubles
	result = strings.ReplaceAll(result, "****", "**")
	_ = boldRe // Silence unused warning

	// Convert italic: _text_ stays the same in Markdown
	// (Already compatible)

	// Convert strikethrough: ~text~ -> ~~text~~
	strikeRe := regexp.MustCompile(`~([^~\n]+)~`)
	result = strikeRe.ReplaceAllString(result, "~~$1~~")

	// Convert code blocks: ```text``` stays the same
	// (Already compatible)

	// Convert inline code: `text` stays the same
	// (Already compatible)

	// Convert blockquotes: &gt; at start of line -> >
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&amp;", "&")

	return result
}

// resolveUser gets a display name for a user ID.
func (p *Parser) resolveUser(userID string) string {
	if name, ok := p.userMap[userID]; ok {
		return name
	}
	return userID
}

// ExportOptions configures the export format.
type ExportOptions struct {
	IncludeThreads   bool
	IncludeReactions bool
	IncludeFiles     bool
	DateFormat       string
	GroupByDate      bool
}

// DefaultExportOptions returns sensible defaults.
func DefaultExportOptions() *ExportOptions {
	return &ExportOptions{
		IncludeThreads:   true,
		IncludeReactions: true,
		IncludeFiles:     true,
		DateFormat:       "2006-01-02 15:04:05",
		GroupByDate:      false,
	}
}
