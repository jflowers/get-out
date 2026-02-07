package parser

import (
	"regexp"
	"strings"
	"time"
)

// Slack mrkdwn patterns
var (
	// User mention: <@U123ABC>
	userMentionPattern = regexp.MustCompile(`<@(U[A-Z0-9]+)(?:\|([^>]+))?>`)

	// Channel mention: <#C123ABC|channel-name> or <#C123ABC>
	channelMentionPattern = regexp.MustCompile(`<#(C[A-Z0-9]+)(?:\|([^>]+))?>`)

	// URL with text: <https://example.com|Example>
	urlWithTextPattern = regexp.MustCompile(`<(https?://[^|>]+)\|([^>]+)>`)

	// URL without text: <https://example.com>
	urlOnlyPattern = regexp.MustCompile(`<(https?://[^>]+)>`)

	// Special mentions
	specialMentionPattern = regexp.MustCompile(`<!([a-z]+)(?:\|([^>]+))?>`)

	// Bold: *text*
	boldPattern = regexp.MustCompile(`\*([^*]+)\*`)

	// Italic: _text_
	italicPattern = regexp.MustCompile(`_([^_]+)_`)

	// Strikethrough: ~text~
	strikePattern = regexp.MustCompile(`~([^~]+)~`)

	// Inline code: `code`
	inlineCodePattern = regexp.MustCompile("`([^`]+)`")

	// Code block: ```code```
	codeBlockPattern = regexp.MustCompile("```([^`]*)```")

	// Slack message link: https://myworkspace.slack.com/archives/C123/p1234567890123456
	slackMessageLinkPattern = regexp.MustCompile(`https://[a-z0-9-]+\.slack\.com/archives/([A-Z0-9]+)/p(\d+)`)
)

// ParsedSegment represents a segment of parsed text with formatting.
type ParsedSegment struct {
	Text    string
	Bold    bool
	Italic  bool
	Strike  bool
	Code    bool
	Link    string // URL if this is a link
	Mention bool   // True if this is a @mention
}

// ConvertMrkdwn converts Slack mrkdwn to plain text with metadata about formatting.
// This is a simplified version that returns plain text suitable for Google Docs.
func ConvertMrkdwn(text string, userResolver *UserResolver, channelResolver *ChannelResolver) string {
	result, _ := ConvertMrkdwnWithLinks(text, userResolver, channelResolver, nil)
	return result
}

// LinkAnnotation records a substring in converted text that should become a hyperlink.
type LinkAnnotation struct {
	Text string // The display text (e.g., "@John Smith")
	URL  string // The link URL (e.g., "mailto:john@example.com")
}

// ConvertMrkdwnWithLinks converts Slack mrkdwn to plain text and returns link annotations
// for @mentions that have Google email mappings via the PersonResolver.
func ConvertMrkdwnWithLinks(text string, userResolver *UserResolver, channelResolver *ChannelResolver, personResolver *PersonResolver) (string, []LinkAnnotation) {
	result := text
	var links []LinkAnnotation

	// Replace user mentions — track link annotations for users with Google emails
	result = userMentionPattern.ReplaceAllStringFunc(result, func(match string) string {
		matches := userMentionPattern.FindStringSubmatch(match)
		if len(matches) >= 2 {
			userID := matches[1]
			displayName := ""

			// Get display name
			if len(matches) >= 3 && matches[2] != "" {
				displayName = matches[2]
			} else if userResolver != nil {
				displayName = userResolver.Resolve(userID)
			} else {
				displayName = userID
			}

			mention := "@" + displayName

			// Check for Google email link
			if personResolver != nil {
				if email := personResolver.ResolveEmail(userID); email != "" {
					links = append(links, LinkAnnotation{
						Text: mention,
						URL:  "mailto:" + email,
					})
				}
			}

			return mention
		}
		return match
	})

	// Replace channel mentions
	result = channelMentionPattern.ReplaceAllStringFunc(result, func(match string) string {
		matches := channelMentionPattern.FindStringSubmatch(match)
		if len(matches) >= 2 {
			if len(matches) >= 3 && matches[2] != "" {
				return "#" + matches[2]
			}
			channelID := matches[1]
			if channelResolver != nil {
				return "#" + channelResolver.Resolve(channelID)
			}
			return "#" + channelID
		}
		return match
	})

	// Replace URLs with text
	result = urlWithTextPattern.ReplaceAllString(result, "$2 ($1)")

	// Replace URLs without text (keep as-is but remove brackets)
	result = urlOnlyPattern.ReplaceAllString(result, "$1")

	// Replace special mentions
	result = specialMentionPattern.ReplaceAllStringFunc(result, func(match string) string {
		matches := specialMentionPattern.FindStringSubmatch(match)
		if len(matches) >= 2 {
			switch matches[1] {
			case "here":
				return "@here"
			case "channel":
				return "@channel"
			case "everyone":
				return "@everyone"
			default:
				if len(matches) >= 3 && matches[2] != "" {
					return matches[2]
				}
				return "@" + matches[1]
			}
		}
		return match
	})

	// Remove formatting markers but keep text
	result = boldPattern.ReplaceAllString(result, "$1")
	result = italicPattern.ReplaceAllString(result, "$1")
	result = strikePattern.ReplaceAllString(result, "$1")

	// Keep code blocks and inline code text
	result = codeBlockPattern.ReplaceAllString(result, "$1")
	result = inlineCodePattern.ReplaceAllString(result, "$1")

	// Decode HTML entities
	result = decodeHTMLEntities(result)

	return result, links
}

// ParseMrkdwnSegments parses Slack mrkdwn into segments with formatting metadata.
// This allows applying rich formatting in Google Docs.
func ParseMrkdwnSegments(text string, userResolver *UserResolver, channelResolver *ChannelResolver) []ParsedSegment {
	var segments []ParsedSegment

	// For now, return a single segment with the converted text
	// TODO: Implement full segment parsing for rich formatting
	converted := ConvertMrkdwn(text, userResolver, channelResolver)
	if converted != "" {
		segments = append(segments, ParsedSegment{Text: converted})
	}

	return segments
}

// ExtractSlackLinks finds Slack message/thread links in text.
type SlackLink struct {
	FullURL    string
	ChannelID  string
	MessageTS  string
	StartIndex int
	EndIndex   int
}

// FindSlackLinks extracts Slack message links from text.
func FindSlackLinks(text string) []SlackLink {
	var links []SlackLink

	matches := slackMessageLinkPattern.FindAllStringSubmatchIndex(text, -1)
	for _, match := range matches {
		if len(match) >= 6 {
			fullURL := text[match[0]:match[1]]
			channelID := text[match[2]:match[3]]
			// Convert p1234567890123456 to 1234567890.123456
			pTimestamp := text[match[4]:match[5]]
			messageTS := pTimestamp[:10] + "." + pTimestamp[10:]

			links = append(links, SlackLink{
				FullURL:    fullURL,
				ChannelID:  channelID,
				MessageTS:  messageTS,
				StartIndex: match[0],
				EndIndex:   match[1],
			})
		}
	}

	return links
}

// ReplaceSlackLinks replaces Slack message links with Google Docs links.
func ReplaceSlackLinks(text string, linkResolver func(channelID, messageTS string) string) string {
	return slackMessageLinkPattern.ReplaceAllStringFunc(text, func(match string) string {
		matches := slackMessageLinkPattern.FindStringSubmatch(match)
		if len(matches) >= 3 {
			channelID := matches[1]
			pTimestamp := matches[2]
			messageTS := pTimestamp[:10] + "." + pTimestamp[10:]

			if replacement := linkResolver(channelID, messageTS); replacement != "" {
				return replacement
			}
		}
		return match
	})
}

// decodeHTMLEntities decodes common HTML entities in Slack messages.
func decodeHTMLEntities(text string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"&nbsp;", " ",
	)
	return replacer.Replace(text)
}

// FormatTimestamp formats a Slack timestamp as a readable time string.
func FormatTimestamp(ts string) string {
	t := tsToTime(ts)
	return t.Format("3:04 PM")
}

// FormatTimestampFull formats a Slack timestamp with date and time.
func FormatTimestampFull(ts string) string {
	t := tsToTime(ts)
	return t.Format("Jan 2, 2006 3:04 PM")
}

// tsToTime converts a Slack timestamp to time.Time.
func tsToTime(ts string) time.Time {
	var sec int64
	for i := 0; i < len(ts); i++ {
		if ts[i] == '.' {
			break
		}
		sec = sec*10 + int64(ts[i]-'0')
	}
	return time.Unix(sec, 0)
}
