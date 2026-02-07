package exporter

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jflowers/get-out/pkg/gdrive"
	"github.com/jflowers/get-out/pkg/parser"
	"github.com/jflowers/get-out/pkg/slackapi"
)

// DocWriter handles writing messages to Google Docs.
type DocWriter struct {
	client          *gdrive.Client
	userResolver    *parser.UserResolver
	channelResolver *parser.ChannelResolver
	personResolver  *parser.PersonResolver
	linkResolver    parser.SlackLinkResolver
}

// NewDocWriter creates a new doc writer.
func NewDocWriter(client *gdrive.Client, userResolver *parser.UserResolver, channelResolver *parser.ChannelResolver, personResolver *parser.PersonResolver, linkResolver parser.SlackLinkResolver) *DocWriter {
	return &DocWriter{
		client:          client,
		userResolver:    userResolver,
		channelResolver: channelResolver,
		personResolver:  personResolver,
		linkResolver:    linkResolver,
	}
}

// WriteMessages writes messages to a Google Doc.
func (w *DocWriter) WriteMessages(ctx context.Context, docID string, messages []slackapi.Message) error {
	if len(messages) == 0 {
		return nil
	}

	// Sort messages by timestamp (oldest first)
	sorted := make([]slackapi.Message, len(messages))
	copy(sorted, messages)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TS < sorted[j].TS
	})

	// Convert to message blocks
	var blocks []gdrive.MessageBlock
	for _, msg := range sorted {
		block := w.messageToBlock(msg)
		if block.Content != "" || block.SenderName != "" {
			blocks = append(blocks, block)
		}
	}

	if len(blocks) == 0 {
		return nil
	}

	return w.client.BatchAppendMessages(ctx, docID, blocks)
}

// messageToBlock converts a Slack message to a doc message block.
func (w *DocWriter) messageToBlock(msg slackapi.Message) gdrive.MessageBlock {
	// Get sender name
	senderName := w.getSenderName(msg)

	// Format timestamp
	timestamp := formatMessageTime(msg.TS)

	// Convert message text and collect link annotations
	content, links := parser.ConvertMrkdwnWithLinks(msg.Text, w.userResolver, w.channelResolver, w.personResolver, w.linkResolver)

	// Convert parser.LinkAnnotation to gdrive.LinkAnnotation
	var docLinks []gdrive.LinkAnnotation
	for _, l := range links {
		docLinks = append(docLinks, gdrive.LinkAnnotation{Text: l.Text, URL: l.URL})
	}

	// Add attachment info if present
	if len(msg.Attachments) > 0 {
		for _, att := range msg.Attachments {
			if att.Text != "" {
				if content != "" {
					content += "\n"
				}
				content += "> " + att.Text
			}
			if att.Title != "" && att.TitleLink != "" {
				if content != "" {
					content += "\n"
				}
				content += fmt.Sprintf("[%s](%s)", att.Title, att.TitleLink)
			}
		}
	}

	// Add file info if present
	if len(msg.Files) > 0 {
		for _, file := range msg.Files {
			if content != "" {
				content += "\n"
			}
			content += fmt.Sprintf("[File: %s]", file.Name)
		}
	}

	// Add reactions if present
	if len(msg.Reactions) > 0 {
		if content != "" {
			content += "\n"
		}
		content += "Reactions: "
		for i, r := range msg.Reactions {
			if i > 0 {
				content += " "
			}
			content += fmt.Sprintf(":%s: (%d)", r.Name, r.Count)
		}
	}

	return gdrive.MessageBlock{
		SenderName: senderName,
		Timestamp:  timestamp,
		Content:    content,
		Links:      docLinks,
	}
}

// getSenderName returns the display name for a message sender.
// Appends [bot] for bot users and [deactivated] for deleted users.
func (w *DocWriter) getSenderName(msg slackapi.Message) string {
	// Check for bot messages with username
	if msg.Username != "" {
		return msg.Username + " [bot]"
	}

	// Resolve user ID
	if msg.User != "" {
		if w.userResolver != nil {
			name := w.userResolver.Resolve(msg.User)
			// Check for bot/deleted indicators
			if user := w.userResolver.GetUser(msg.User); user != nil {
				if user.IsBot || user.IsAppUser {
					name += " [bot]"
				} else if user.Deleted {
					name += " [deactivated]"
				}
			}
			return name
		}
		return msg.User
	}

	// Check for bot ID
	if msg.BotID != "" {
		return "Bot"
	}

	return "Unknown"
}

// formatMessageTime formats a timestamp for display in the doc.
func formatMessageTime(ts string) string {
	t := parseSlackTS(ts)
	return t.Format("3:04 PM")
}

// parseSlackTS parses a Slack timestamp string.
func parseSlackTS(ts string) time.Time {
	var sec int64
	for i := 0; i < len(ts); i++ {
		if ts[i] == '.' {
			break
		}
		sec = sec*10 + int64(ts[i]-'0')
	}
	return time.Unix(sec, 0)
}

// GroupMessagesByDate groups messages by their date.
func GroupMessagesByDate(messages []slackapi.Message) map[string][]slackapi.Message {
	groups := make(map[string][]slackapi.Message)

	for _, msg := range messages {
		date := DateFromTS(msg.TS)
		groups[date] = append(groups[date], msg)
	}

	return groups
}

// SortedDates returns the dates from a message group in sorted order.
func SortedDates(groups map[string][]slackapi.Message) []string {
	dates := make([]string, 0, len(groups))
	for date := range groups {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	return dates
}

// FilterMainMessages returns only top-level messages (not thread replies).
func FilterMainMessages(messages []slackapi.Message) []slackapi.Message {
	var main []slackapi.Message
	for _, msg := range messages {
		// A message is a main message if it doesn't have a thread_ts,
		// or if its ts equals its thread_ts (it's the thread parent)
		if msg.ThreadTS == "" || msg.TS == msg.ThreadTS {
			main = append(main, msg)
		}
	}
	return main
}

// FilterThreadMessages returns messages that are part of a specific thread.
func FilterThreadMessages(messages []slackapi.Message, threadTS string) []slackapi.Message {
	var thread []slackapi.Message
	for _, msg := range messages {
		if msg.ThreadTS == threadTS {
			thread = append(thread, msg)
		}
	}
	return thread
}

// GetThreadParents returns messages that have replies (thread parents).
func GetThreadParents(messages []slackapi.Message) []slackapi.Message {
	var parents []slackapi.Message
	for _, msg := range messages {
		if msg.ReplyCount > 0 {
			parents = append(parents, msg)
		}
	}
	return parents
}
