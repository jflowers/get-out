package exporter

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jflowers/get-out/pkg/models"
	"github.com/jflowers/get-out/pkg/parser"
	"github.com/jflowers/get-out/pkg/slackapi"
)

// MarkdownWriter converts Slack messages to markdown format.
type MarkdownWriter struct {
	userResolver    *parser.UserResolver
	channelResolver *parser.ChannelResolver
	personResolver  *parser.PersonResolver
}

// NewMarkdownWriter creates a new MarkdownWriter with the given resolvers.
func NewMarkdownWriter(userResolver *parser.UserResolver, channelResolver *parser.ChannelResolver, personResolver *parser.PersonResolver) *MarkdownWriter {
	return &MarkdownWriter{
		userResolver:    userResolver,
		channelResolver: channelResolver,
		personResolver:  personResolver,
	}
}

// RenderDailyDoc produces a complete markdown document with YAML frontmatter
// for the given conversation's messages on a specific date.
//
// When filterResult is non-nil, a sensitivity: block is added to the YAML
// frontmatter for audit purposes. When filterResult is nil, no sensitivity
// block is emitted (backward compatible with pre-filter exports).
func (w *MarkdownWriter) RenderDailyDoc(convName string, convType string, date string, messages []slackapi.Message, filterResult *FilterResult) ([]byte, error) {
	// Sort messages by timestamp (oldest first)
	sorted := make([]slackapi.Message, len(messages))
	copy(sorted, messages)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TS < sorted[j].TS
	})

	// Collect unique participant names
	participants := w.collectParticipants(sorted)

	// Build the document
	var b strings.Builder

	// YAML frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("conversation: %s\n", convName))
	b.WriteString(fmt.Sprintf("type: %s\n", convType))
	b.WriteString(fmt.Sprintf("date: \"%s\"\n", date))
	b.WriteString("participants:\n")
	for _, p := range participants {
		b.WriteString(fmt.Sprintf("  - %s\n", p))
	}

	// Sensitivity metadata — only when a filter was applied.
	if filterResult != nil {
		w.renderSensitivityFrontmatter(&b, filterResult)
	}

	b.WriteString("---\n\n")

	// Render each message
	for _, msg := range sorted {
		w.renderMessage(&b, msg)
	}

	return []byte(b.String()), nil
}

// renderSensitivityFrontmatter writes the sensitivity: YAML block into the
// frontmatter builder. Called only when a FilterResult is available.
func (w *MarkdownWriter) renderSensitivityFrontmatter(b *strings.Builder, fr *FilterResult) {
	b.WriteString(fmt.Sprintf("sensitivity:\n"))
	b.WriteString(fmt.Sprintf("  filtered_count: %d\n", fr.FilteredCount))
	if fr.FilteredCount > 0 && len(fr.CategoryBreakdown) > 0 {
		b.WriteString("  categories:\n")
		// Sort categories for deterministic output.
		cats := sortedMapKeys(fr.CategoryBreakdown)
		for _, cat := range cats {
			b.WriteString(fmt.Sprintf("    %s: %d\n", cat, fr.CategoryBreakdown[cat]))
		}
	}
}

// sortedMapKeys returns the keys of a map[string]int in sorted order.
func sortedMapKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// collectParticipants extracts unique sender names from the messages, sorted alphabetically.
func (w *MarkdownWriter) collectParticipants(messages []slackapi.Message) []string {
	seen := make(map[string]bool)
	for _, msg := range messages {
		name := w.getSenderName(msg)
		if name != "" {
			seen[name] = true
		}
	}

	participants := make([]string, 0, len(seen))
	for name := range seen {
		participants = append(participants, name)
	}
	sort.Strings(participants)
	return participants
}

// getSenderName returns the display name for a message sender.
// Resolution order: Username (bot) -> PersonResolver -> UserResolver -> raw ID -> BotID -> "Unknown".
func (w *MarkdownWriter) getSenderName(msg slackapi.Message) string {
	// Check for bot messages with username
	if msg.Username != "" {
		return msg.Username + " [bot]"
	}

	// Resolve user ID
	if msg.User != "" {
		name := ""

		// Try people.json first (primary source)
		if w.personResolver != nil {
			name = w.personResolver.ResolveName(msg.User)
		}

		// Fall back to Slack API cache
		if name == "" && w.userResolver != nil {
			resolved := w.userResolver.Resolve(msg.User)
			if resolved != msg.User {
				name = resolved
			}
		}

		// If still unresolved, use raw ID
		if name == "" {
			name = msg.User
		}

		// Check for bot/deleted indicators
		if w.userResolver != nil {
			if user := w.userResolver.GetUser(msg.User); user != nil {
				if user.IsBot || user.IsAppUser {
					name += " [bot]"
				} else if user.Deleted {
					name += " [deactivated]"
				}
			}
		}
		return name
	}

	// Check for bot ID
	if msg.BotID != "" {
		return "Bot"
	}

	return "Unknown"
}

// renderMessage formats a single message and writes it to the builder.
func (w *MarkdownWriter) renderMessage(b *strings.Builder, msg slackapi.Message) {
	senderName := w.getSenderName(msg)
	timestamp := parser.FormatTimestamp(msg.TS)

	// Header line: **time -- sender**
	b.WriteString(fmt.Sprintf("**%s -- %s**\n\n", timestamp, senderName))

	// Message content converted from Slack mrkdwn to standard Markdown
	content := parser.ConvertMrkdwnToMarkdown(msg.Text, w.userResolver, w.channelResolver, w.personResolver)
	if content != "" {
		b.WriteString(content)
		b.WriteString("\n\n")
	}

	// Reactions
	reactText := formatReactions(msg.Reactions)
	if reactText != "" {
		b.WriteString(reactText)
		b.WriteString("\n\n")
	}

	// Attachments (blockquoted)
	attText := w.formatAttachmentsMarkdown(msg.Attachments)
	if attText != "" {
		b.WriteString(attText)
		b.WriteString("\n\n")
	}

	// Thread parent marker
	if msg.ReplyCount > 0 && (msg.ThreadTS == "" || msg.TS == msg.ThreadTS) {
		b.WriteString("**Thread replies:**\n\n")
	}
}

// formatAttachmentsMarkdown converts attachments to blockquoted markdown text.
func (w *MarkdownWriter) formatAttachmentsMarkdown(attachments []slackapi.Attachment) string {
	if len(attachments) == 0 {
		return ""
	}
	var parts []string
	for _, att := range attachments {
		if att.Text != "" {
			// Blockquote each line of the attachment text
			lines := strings.Split(att.Text, "\n")
			for _, line := range lines {
				parts = append(parts, "> "+line)
			}
		}
		if att.Title != "" && att.TitleLink != "" {
			parts = append(parts, fmt.Sprintf("> [%s](%s)", att.Title, att.TitleLink))
		}
	}
	return strings.Join(parts, "\n")
}

// sanitizeDirNameRe matches non-alphanumeric characters except hyphens.
var sanitizeDirNameRe = regexp.MustCompile(`[^a-z0-9-]`)

// collapseHyphensRe matches consecutive hyphens.
var collapseHyphensRe = regexp.MustCompile(`-{2,}`)

// SanitizeDirectoryName maps a ConversationType to a directory prefix and
// sanitizes the conversation name for use as a filesystem directory name.
func SanitizeDirectoryName(convType, convName string) string {
	// Map conversation type to directory prefix
	prefix := mapTypeToPrefix(convType)

	// Sanitize the name
	sanitized := sanitizeName(convName)

	if sanitized == "" {
		return prefix
	}
	return prefix + "-" + sanitized
}

// mapTypeToPrefix maps a ConversationType string to a directory prefix.
func mapTypeToPrefix(convType string) string {
	switch models.ConversationType(convType) {
	case models.ConversationTypeDM:
		return "dm"
	case models.ConversationTypeMPIM:
		return "group"
	case models.ConversationTypeChannel:
		return "channel"
	case models.ConversationTypePrivateChannel:
		return "private-channel"
	default:
		return convType
	}
}

// sanitizeName processes a conversation name into a filesystem-safe string.
func sanitizeName(name string) string {
	// Lowercase
	s := strings.ToLower(name)

	// Spaces to hyphens
	s = strings.ReplaceAll(s, " ", "-")

	// Remove non-alphanumeric (except hyphens)
	s = sanitizeDirNameRe.ReplaceAllString(s, "")

	// Collapse consecutive hyphens
	s = collapseHyphensRe.ReplaceAllString(s, "-")

	// Trim leading/trailing hyphens
	s = strings.Trim(s, "-")

	// Truncate to 100 chars
	if len(s) > 100 {
		s = s[:100]
		// If truncation left a trailing hyphen, trim it
		s = strings.TrimRight(s, "-")
	}

	return s
}
