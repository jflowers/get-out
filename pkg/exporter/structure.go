package exporter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jflowers/get-out/pkg/gdrive"
)

// FolderStructure manages the Google Drive folder organization for exports.
type FolderStructure struct {
	client *gdrive.Client
	index  *ExportIndex

	// Root folder name in Google Drive (used when creating new folder)
	rootFolderName string

	// Root folder ID in Google Drive (if specified, uses existing folder)
	rootFolderID string
}

// FolderStructureConfig holds configuration for folder structure.
type FolderStructureConfig struct {
	// RootFolderName is the name for the root export folder (default: "Slack Exports")
	RootFolderName string

	// RootFolderID is an optional existing folder ID to use as the root.
	// If provided, RootFolderName is ignored and this folder is used directly.
	RootFolderID string
}

// NewFolderStructure creates a new folder structure manager.
func NewFolderStructure(client *gdrive.Client, index *ExportIndex, cfg *FolderStructureConfig) *FolderStructure {
	if cfg == nil {
		cfg = &FolderStructureConfig{}
	}
	if cfg.RootFolderName == "" {
		cfg.RootFolderName = "Slack Exports"
	}
	return &FolderStructure{
		client:         client,
		index:          index,
		rootFolderName: cfg.RootFolderName,
		rootFolderID:   cfg.RootFolderID,
	}
}

// EnsureRootFolder creates or finds the root export folder.
// If a root folder ID was configured, it uses that existing folder.
// Otherwise, it finds or creates a folder by name.
func (fs *FolderStructure) EnsureRootFolder(ctx context.Context) (*gdrive.FolderInfo, error) {
	// Check if we already have it in the index
	if fs.index.RootFolderID != "" {
		return &gdrive.FolderInfo{
			ID:   fs.index.RootFolderID,
			Name: fs.rootFolderName,
			URL:  fs.index.RootFolderURL,
		}, nil
	}

	// If a specific folder ID was configured, use it directly
	if fs.rootFolderID != "" {
		// Verify the folder exists and get its info
		folder, err := fs.client.GetFolder(ctx, fs.rootFolderID)
		if err != nil {
			return nil, fmt.Errorf("failed to access folder %s: %w", fs.rootFolderID, err)
		}

		// Update index with the provided folder
		fs.index.RootFolderID = folder.ID
		fs.index.RootFolderURL = folder.URL

		return folder, nil
	}

	// Find or create the root folder by name
	folder, err := fs.client.FindOrCreateFolder(ctx, fs.rootFolderName, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create root folder: %w", err)
	}

	// Update index
	fs.index.RootFolderID = folder.ID
	fs.index.RootFolderURL = folder.URL

	return folder, nil
}

// ConversationFolderName generates the folder name for a conversation.
func ConversationFolderName(convType, name string) string {
	var prefix string
	switch convType {
	case "dm":
		prefix = "DM"
	case "mpim":
		prefix = "Group"
	case "channel":
		prefix = "Channel"
	case "private_channel":
		prefix = "Private"
	default:
		prefix = "Chat"
	}
	return fmt.Sprintf("%s - %s", prefix, sanitizeFolderName(name))
}

// EnsureConversationFolder creates or finds the folder for a conversation.
func (fs *FolderStructure) EnsureConversationFolder(ctx context.Context, convID, convType, name string) (*ConversationExport, error) {
	// Check if we already have it
	conv := fs.index.GetConversation(convID)
	if conv != nil && conv.FolderID != "" {
		return conv, nil
	}

	// Ensure root folder exists
	root, err := fs.EnsureRootFolder(ctx)
	if err != nil {
		return nil, err
	}

	// Create conversation folder
	folderName := ConversationFolderName(convType, name)
	folder, err := fs.client.FindOrCreateFolder(ctx, folderName, root.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation folder: %w", err)
	}

	// Update or create conversation export
	if conv == nil {
		conv = fs.index.GetOrCreateConversation(convID, name, convType)
	}
	conv.FolderID = folder.ID
	conv.FolderURL = folder.URL
	conv.Name = name
	conv.Type = convType

	return conv, nil
}

// EnsureThreadsFolder creates or finds the "Threads" subfolder for a conversation.
func (fs *FolderStructure) EnsureThreadsFolder(ctx context.Context, convID string) (string, error) {
	conv := fs.index.GetConversation(convID)
	if conv == nil {
		return "", fmt.Errorf("conversation not found in index: %s", convID)
	}

	// Check if we already have it
	if conv.ThreadsFolderID != "" {
		return conv.ThreadsFolderID, nil
	}

	// Create Threads subfolder
	folder, err := fs.client.FindOrCreateFolder(ctx, "Threads", conv.FolderID)
	if err != nil {
		return "", fmt.Errorf("failed to create Threads folder: %w", err)
	}

	conv.ThreadsFolderID = folder.ID
	return folder.ID, nil
}

// EnsureThreadFolder creates or finds a folder for a specific thread.
func (fs *FolderStructure) EnsureThreadFolder(ctx context.Context, convID, threadTS, topicPreview string) (*ThreadExport, error) {
	// Check if we already have it
	thread := fs.index.GetThread(convID, threadTS)
	if thread != nil && thread.FolderID != "" {
		return thread, nil
	}

	// Ensure Threads folder exists
	threadsFolderID, err := fs.EnsureThreadsFolder(ctx, convID)
	if err != nil {
		return nil, err
	}

	// Generate thread folder name: "YYYY-MM-DD - Topic preview"
	date := tsToDate(threadTS)
	folderName := fmt.Sprintf("%s - %s", date, sanitizeFolderName(truncate(topicPreview, 40)))

	folder, err := fs.client.FindOrCreateFolder(ctx, folderName, threadsFolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to create thread folder: %w", err)
	}

	// Create or update thread export
	if thread == nil {
		thread = &ThreadExport{
			ThreadTS:  threadTS,
			DailyDocs: make(map[string]*DocExport),
		}
	}
	thread.FolderID = folder.ID
	thread.FolderURL = folder.URL
	thread.FolderName = folderName

	fs.index.SetThread(convID, thread)

	return thread, nil
}

// EnsureDailyDoc creates or finds a daily Google Doc for a conversation.
func (fs *FolderStructure) EnsureDailyDoc(ctx context.Context, convID, date string) (*DocExport, error) {
	// Check if we already have it
	doc := fs.index.GetDailyDoc(convID, date)
	if doc != nil && doc.DocID != "" {
		return doc, nil
	}

	conv := fs.index.GetConversation(convID)
	if conv == nil {
		return nil, fmt.Errorf("conversation not found in index: %s", convID)
	}

	// Create the doc with date as title
	title := date // e.g., "2026-02-03"
	gdoc, err := fs.client.FindOrCreateDocument(ctx, title, conv.FolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to create daily doc: %w", err)
	}

	doc = &DocExport{
		DocID:  gdoc.ID,
		DocURL: gdoc.URL,
		Title:  title,
		Date:   date,
	}

	fs.index.SetDailyDoc(convID, date, doc)

	return doc, nil
}

// EnsureThreadDailyDoc creates or finds a daily doc within a thread folder.
func (fs *FolderStructure) EnsureThreadDailyDoc(ctx context.Context, convID, threadTS, date string) (*DocExport, error) {
	thread := fs.index.GetThread(convID, threadTS)
	if thread == nil {
		return nil, fmt.Errorf("thread not found in index: %s/%s", convID, threadTS)
	}

	// Check if we already have it
	if doc, ok := thread.DailyDocs[date]; ok && doc.DocID != "" {
		return doc, nil
	}

	// Create the doc
	title := date
	gdoc, err := fs.client.FindOrCreateDocument(ctx, title, thread.FolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to create thread daily doc: %w", err)
	}

	doc := &DocExport{
		DocID:  gdoc.ID,
		DocURL: gdoc.URL,
		Title:  title,
		Date:   date,
	}

	if thread.DailyDocs == nil {
		thread.DailyDocs = make(map[string]*DocExport)
	}
	thread.DailyDocs[date] = doc

	return doc, nil
}

// GetDocForMessage returns the appropriate doc for a message based on its timestamp.
func (fs *FolderStructure) GetDocForMessage(ctx context.Context, convID, messageTS string, isThread bool, threadTS string) (*DocExport, error) {
	date := tsToDate(messageTS)

	if isThread && threadTS != "" {
		return fs.EnsureThreadDailyDoc(ctx, convID, threadTS, date)
	}

	return fs.EnsureDailyDoc(ctx, convID, date)
}

// sanitizeFolderName removes or replaces characters not allowed in folder names.
func sanitizeFolderName(name string) string {
	// Replace problematic characters
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "'",
		"<", "(",
		">", ")",
		"|", "-",
	)
	result := replacer.Replace(name)

	// Trim whitespace and limit length
	result = strings.TrimSpace(result)
	if len(result) > 100 {
		result = result[:100]
	}

	return result
}

// truncate shortens a string to maxLen, adding "..." if truncated.
// It also sanitizes the string by replacing newlines and tabs with spaces.
func truncate(s string, maxLen int) string {
	// Sanitize first: replace newlines, tabs, and carriage returns with spaces
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.TrimSpace(s)

	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// TSToTime converts a Slack timestamp to time.Time.
func TSToTime(ts string) time.Time {
	var sec int64
	for i := 0; i < len(ts); i++ {
		if ts[i] == '.' {
			break
		}
		sec = sec*10 + int64(ts[i]-'0')
	}
	return time.Unix(sec, 0)
}

// DateFromTS extracts the date string from a Slack timestamp.
func DateFromTS(ts string) string {
	return tsToDate(ts)
}
