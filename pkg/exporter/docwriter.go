package exporter

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jflowers/get-out/pkg/gdrive"
	"github.com/jflowers/get-out/pkg/parser"
	"github.com/jflowers/get-out/pkg/slackapi"
)

// DocWriter handles writing messages to Google Docs.
type DocWriter struct {
	client          *gdrive.Client
	slackClient     *slackapi.Client
	userResolver    *parser.UserResolver
	channelResolver *parser.ChannelResolver
	personResolver  *parser.PersonResolver
	linkResolver    parser.SlackLinkResolver
	threadResolver  parser.SlackLinkResolver
}

// NewDocWriter creates a new doc writer.
func NewDocWriter(client *gdrive.Client, slackClient *slackapi.Client, userResolver *parser.UserResolver, channelResolver *parser.ChannelResolver, personResolver *parser.PersonResolver, linkResolver parser.SlackLinkResolver, threadResolver parser.SlackLinkResolver) *DocWriter {
	return &DocWriter{
		client:          client,
		slackClient:     slackClient,
		userResolver:    userResolver,
		channelResolver: channelResolver,
		personResolver:  personResolver,
		linkResolver:    linkResolver,
		threadResolver:  threadResolver,
	}
}

// WriteMessages writes messages to a Google Doc.
// convID is the Slack conversation ID (for thread link resolution).
// folderID is the ID of the conversation folder (used for temp image uploads).
func (w *DocWriter) WriteMessages(ctx context.Context, docID string, convID string, folderID string, messages []slackapi.Message) error {
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
		block := w.messageToBlock(ctx, convID, folderID, msg)
		if block.Content != "" || block.SenderName != "" || len(block.Images) > 0 {
			blocks = append(blocks, block)
		}
	}

	if len(blocks) == 0 {
		return nil
	}

	return w.client.BatchAppendMessages(ctx, docID, blocks)
}

// messageToBlock converts a Slack message to a doc message block.
func (w *DocWriter) messageToBlock(ctx context.Context, convID string, folderID string, msg slackapi.Message) gdrive.MessageBlock {
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

	// Add thread link if it's a parent
	if msg.ReplyCount > 0 && w.threadResolver != nil {
		threadURL := w.threadResolver(convID, msg.TS)
		if threadURL != "" {
			if content != "" {
				content += "\n"
			}
			linkText := "→ View Thread"
			content += linkText
			docLinks = append(docLinks, gdrive.LinkAnnotation{Text: linkText, URL: threadURL})
		}
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

	// Process files (handle images)
	var docImages []gdrive.ImageAnnotation
	if len(msg.Files) > 0 {
		for _, file := range msg.Files {
			// If it's an image, try to embed it
			if strings.HasPrefix(file.Mimetype, "image/") && w.slackClient != nil && w.client != nil {
				// Download from Slack
				data, err := w.slackClient.DownloadFile(ctx, file.URLPrivateDownload)
				if err == nil {
					// Upload to Drive temporarily
					fileID, err := w.client.UploadFile(ctx, file.Name, file.Mimetype, data, folderID)
					if err == nil {
						// Make public for Docs API
						if err := w.client.MakePublic(ctx, fileID); err == nil {
							// Get web content link
							url, err := w.client.GetWebContentLink(ctx, fileID)
							if err == nil {
								docImages = append(docImages, gdrive.ImageAnnotation{URL: url})
							}
						}
					}
				}
			} else {
				// Non-image file: just add a text reference
				if content != "" {
					content += "\n"
				}
				content += fmt.Sprintf("[File: %s]", file.Name)
			}
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
		Images:     docImages,
	}
}

// getSenderName returns the display name for a message sender.
// Resolution order: people.json (PersonResolver) → Slack API cache (UserResolver) → raw ID.
// Appends [bot] for bot users and [deactivated] for deleted users.
func (w *DocWriter) getSenderName(msg slackapi.Message) string {
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
