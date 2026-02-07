package exporter

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jflowers/get-out/pkg/chrome"
	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/gdrive"
	"github.com/jflowers/get-out/pkg/parser"
	"github.com/jflowers/get-out/pkg/slackapi"
)

// Exporter orchestrates the export of Slack conversations to Google Docs.
type Exporter struct {
	// Configuration
	configDir             string
	rootFolderName        string
	rootFolderID          string
	googleCredentialsFile string
	slackBotToken         string

	// Clients
	slackClient  *slackapi.Client
	gdriveClient *gdrive.Client

	// Helpers
	folderStructure *FolderStructure
	docWriter       *DocWriter
	userResolver    *parser.UserResolver
	channelResolver *parser.ChannelResolver
	index           *ExportIndex

	// Progress callback
	onProgress func(msg string)

	// Options
	debug    bool
	dateFrom string // Slack timestamp: only messages after this
	dateTo   string // Slack timestamp: only messages before this
	syncMode bool   // Use LastMessageTS from index as oldest
}

// ExporterConfig holds configuration for creating an Exporter.
type ExporterConfig struct {
	ConfigDir      string
	RootFolderName string
	RootFolderID   string // Optional: use existing folder by ID instead of creating by name
	ChromePort     int
	Debug          bool
	OnProgress     func(msg string)

	// Optional paths from settings.json
	GoogleCredentialsFile string // Custom path to credentials.json
	SlackBotToken         string // Bot token for API mode

	// Date range and sync options
	DateFrom string // Slack timestamp: only export messages after this
	DateTo   string // Slack timestamp: only export messages before this
	SyncMode bool   // Only export messages since last successful export
}

// Progress is a helper to report progress.
func (e *Exporter) Progress(format string, args ...interface{}) {
	if e.onProgress != nil {
		e.onProgress(fmt.Sprintf(format, args...))
	}
}

// NewExporter creates a new exporter with the given configuration.
// It does NOT initialize connections - call Initialize() separately.
func NewExporter(cfg *ExporterConfig) *Exporter {
	return &Exporter{
		configDir:             cfg.ConfigDir,
		rootFolderName:        cfg.RootFolderName,
		rootFolderID:          cfg.RootFolderID,
		googleCredentialsFile: cfg.GoogleCredentialsFile,
		slackBotToken:         cfg.SlackBotToken,
		debug:                 cfg.Debug,
		onProgress:            cfg.OnProgress,
		dateFrom:              cfg.DateFrom,
		dateTo:                cfg.DateTo,
		syncMode:              cfg.SyncMode,
		userResolver:          parser.NewUserResolver(),
		channelResolver:       parser.NewChannelResolver(),
	}
}

// Initialize sets up connections to Chrome/Slack and Google Drive.
func (e *Exporter) Initialize(ctx context.Context, chromePort int) error {
	e.Progress("Loading export index...")
	indexPath := DefaultIndexPath(e.configDir)
	index, err := LoadExportIndex(indexPath)
	if err != nil {
		return fmt.Errorf("failed to load export index: %w", err)
	}
	e.index = index

	e.Progress("Authenticating with Google Drive...")
	gdriveCfg := gdrive.DefaultConfig(e.configDir)
	if e.googleCredentialsFile != "" {
		gdriveCfg.CredentialsPath = e.googleCredentialsFile
		gdriveCfg.TokenPath = filepath.Join(filepath.Dir(e.googleCredentialsFile), "token.json")
	}
	gdriveClient, err := gdrive.NewClientFromConfig(ctx, gdriveCfg)
	if err != nil {
		return fmt.Errorf("failed to authenticate with Google: %w", err)
	}
	e.gdriveClient = gdriveClient

	e.Progress("Connecting to Chrome (port %d)...", chromePort)
	chromeCfg := &chrome.Config{
		DebugPort: chromePort,
		Timeout:   30 * time.Second,
	}
	session, err := chrome.Connect(ctx, chromeCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to Chrome: %w", err)
	}
	defer session.Close()

	e.Progress("Extracting Slack credentials...")
	creds, err := session.ExtractCredentials(ctx)
	if err != nil {
		return fmt.Errorf("failed to extract Slack credentials: %w", err)
	}
	e.Progress("Found Slack team: %s", creds.TeamDomain)

	// Create Slack client with browser credentials
	e.slackClient = slackapi.NewBrowserClient(creds.Token, creds.Cookie)

	// Create folder structure manager
	e.folderStructure = NewFolderStructure(e.gdriveClient, e.index, &FolderStructureConfig{
		RootFolderName: e.rootFolderName,
		RootFolderID:   e.rootFolderID,
	})

	// Create doc writer
	e.docWriter = NewDocWriter(e.gdriveClient, e.userResolver, e.channelResolver)

	return nil
}

// InitializeWithSlackClient initializes with an existing Slack client (for API mode).
func (e *Exporter) InitializeWithSlackClient(ctx context.Context, slackClient *slackapi.Client) error {
	e.Progress("Loading export index...")
	indexPath := DefaultIndexPath(e.configDir)
	index, err := LoadExportIndex(indexPath)
	if err != nil {
		return fmt.Errorf("failed to load export index: %w", err)
	}
	e.index = index

	e.Progress("Authenticating with Google Drive...")
	gdriveCfg := gdrive.DefaultConfig(e.configDir)
	if e.googleCredentialsFile != "" {
		gdriveCfg.CredentialsPath = e.googleCredentialsFile
		gdriveCfg.TokenPath = filepath.Join(filepath.Dir(e.googleCredentialsFile), "token.json")
	}
	gdriveClient, err := gdrive.NewClientFromConfig(ctx, gdriveCfg)
	if err != nil {
		return fmt.Errorf("failed to authenticate with Google: %w", err)
	}
	e.gdriveClient = gdriveClient

	e.slackClient = slackClient

	// Create folder structure manager
	e.folderStructure = NewFolderStructure(e.gdriveClient, e.index, &FolderStructureConfig{
		RootFolderName: e.rootFolderName,
		RootFolderID:   e.rootFolderID,
	})

	// Create doc writer
	e.docWriter = NewDocWriter(e.gdriveClient, e.userResolver, e.channelResolver)

	return nil
}

// LoadUsersForConversations loads user data for the specific conversations being exported.
func (e *Exporter) LoadUsersForConversations(ctx context.Context, channelIDs []string) error {
	e.Progress("Loading users from %d conversations...", len(channelIDs))
	if err := e.userResolver.LoadUsersForConversations(ctx, e.slackClient, channelIDs, func(id string, count int) {
		if count < 0 {
			e.Progress("Could not access members for %s (will resolve on-the-fly)", id)
		} else if id == "users" {
			e.Progress("Fetched %d user profiles...", count)
		} else {
			e.Progress("Found %d unique members so far...", count)
		}
	}); err != nil {
		return fmt.Errorf("failed to load users: %w", err)
	}
	e.Progress("Loaded %d users", e.userResolver.Count())
	return nil
}

// ExportConversation exports a single conversation to Google Docs.
func (e *Exporter) ExportConversation(ctx context.Context, conv config.ConversationConfig) (*ExportResult, error) {
	result := &ExportResult{
		ConversationID: conv.ID,
		Name:           conv.Name,
	}

	startTime := time.Now()
	e.Progress("Exporting conversation: %s (%s)", conv.Name, conv.ID)

	// Create folder structure
	e.Progress("Creating folder structure...")
	convExport, err := e.folderStructure.EnsureConversationFolder(ctx, conv.ID, string(conv.Type), conv.Name)
	if err != nil {
		return result, fmt.Errorf("failed to create folder: %w", err)
	}
	result.FolderURL = convExport.FolderURL

	// Determine oldest/latest bounds
	oldest := ""
	latest := ""

	if e.syncMode {
		// Sync mode: pick up from where we left off
		if convExport.LastMessageTS != "" {
			oldest = convExport.LastMessageTS
			e.Progress("Syncing from last export timestamp: %s", oldest)
		} else {
			e.Progress("No previous export found, fetching all messages")
		}
	} else {
		// Date range mode or full export
		if e.dateFrom != "" {
			oldest = e.dateFrom
		}
		if e.dateTo != "" {
			latest = e.dateTo
		}
		if oldest != "" || latest != "" {
			e.Progress("Date range: %s to %s", orDefault(oldest, "beginning"), orDefault(latest, "now"))
		}
	}

	// Fetch all messages
	e.Progress("Fetching messages...")
	var allMessages []slackapi.Message
	messageCount := 0

	err = e.slackClient.GetAllMessages(ctx, conv.ID, oldest, latest, func(batch []slackapi.Message) error {
		allMessages = append(allMessages, batch...)
		messageCount += len(batch)
		e.Progress("Fetched %d messages...", messageCount)
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("failed to fetch messages: %w", err)
	}

	if len(allMessages) == 0 {
		e.Progress("No new messages to export")
		result.Duration = time.Since(startTime)
		return result, nil
	}

	e.Progress("Processing %d messages...", len(allMessages))

	// Filter to main messages (not thread replies)
	mainMessages := FilterMainMessages(allMessages)
	e.Progress("Found %d main messages, %d thread replies", len(mainMessages), len(allMessages)-len(mainMessages))

	// Group messages by date
	messagesByDate := GroupMessagesByDate(mainMessages)
	dates := SortedDates(messagesByDate)

	e.Progress("Writing to %d daily docs...", len(dates))

	// Write each day's messages to a doc
	for _, date := range dates {
		msgs := messagesByDate[date]

		// Create or find daily doc
		docExport, err := e.folderStructure.EnsureDailyDoc(ctx, conv.ID, date)
		if err != nil {
			return result, fmt.Errorf("failed to create doc for %s: %w", date, err)
		}

		// Write messages to doc
		if err := e.docWriter.WriteMessages(ctx, docExport.DocID, msgs); err != nil {
			return result, fmt.Errorf("failed to write messages for %s: %w", date, err)
		}

		// Update doc stats
		docExport.MessageCount += len(msgs)
		if len(msgs) > 0 {
			docExport.LastMessageTS = msgs[len(msgs)-1].TS
		}

		result.DocsCreated++
		result.MessageCount += len(msgs)
		e.Progress("Wrote %d messages to %s", len(msgs), date)
	}

	// Export threads if any
	threadParents := GetThreadParents(allMessages)
	if len(threadParents) > 0 {
		e.Progress("Exporting %d threads...", len(threadParents))
		for _, parent := range threadParents {
			if err := e.exportThread(ctx, conv.ID, parent); err != nil {
				e.Progress("Warning: failed to export thread %s: %v", parent.TS, err)
				// Continue with other threads
			}
			result.ThreadsExported++
		}
	}

	// Update conversation export state
	if len(allMessages) > 0 {
		convExport.LastMessageTS = allMessages[0].TS // Messages come in reverse order
		convExport.MessageCount += len(allMessages)
		convExport.LastUpdated = time.Now()
	}

	// Save index checkpoint
	if err := e.index.Save(); err != nil {
		e.Progress("Warning: failed to save index: %v", err)
	}

	result.Duration = time.Since(startTime)
	e.Progress("Completed export of %s in %v", conv.Name, result.Duration)

	return result, nil
}

// exportThread exports a single thread to its own folder.
func (e *Exporter) exportThread(ctx context.Context, convID string, parent slackapi.Message) error {
	// Create topic preview from parent message
	topicPreview := truncate(parent.Text, 40)
	if topicPreview == "" {
		topicPreview = "Thread"
	}

	// Create thread folder
	threadExport, err := e.folderStructure.EnsureThreadFolder(ctx, convID, parent.TS, topicPreview)
	if err != nil {
		return fmt.Errorf("failed to create thread folder: %w", err)
	}

	// Fetch thread replies
	var replies []slackapi.Message
	err = e.slackClient.GetAllReplies(ctx, convID, parent.TS, func(batch []slackapi.Message) error {
		replies = append(replies, batch...)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to fetch replies: %w", err)
	}

	if len(replies) == 0 {
		return nil
	}

	// Group by date and write
	replyByDate := GroupMessagesByDate(replies)
	dates := SortedDates(replyByDate)

	for _, date := range dates {
		msgs := replyByDate[date]

		// Create thread daily doc
		docExport, err := e.folderStructure.EnsureThreadDailyDoc(ctx, convID, parent.TS, date)
		if err != nil {
			return fmt.Errorf("failed to create thread doc: %w", err)
		}

		if err := e.docWriter.WriteMessages(ctx, docExport.DocID, msgs); err != nil {
			return fmt.Errorf("failed to write thread messages: %w", err)
		}

		docExport.MessageCount += len(msgs)
	}

	// Update thread state
	threadExport.ReplyCount = len(replies)
	if len(replies) > 0 {
		threadExport.LastReplyTS = replies[len(replies)-1].TS
	}

	return nil
}

// ExportAll exports all conversations in the provided list.
func (e *Exporter) ExportAll(ctx context.Context, conversations []config.ConversationConfig) ([]*ExportResult, error) {
	// Collect channel IDs and load only users from those conversations
	channelIDs := make([]string, len(conversations))
	for i, conv := range conversations {
		channelIDs[i] = conv.ID
	}
	if err := e.LoadUsersForConversations(ctx, channelIDs); err != nil {
		return nil, err
	}

	var results []*ExportResult
	for i, conv := range conversations {
		e.Progress("Exporting conversation %d/%d: %s", i+1, len(conversations), conv.Name)

		result, err := e.ExportConversation(ctx, conv)
		results = append(results, result)

		if err != nil {
			result.Error = err
			e.Progress("Error exporting %s: %v", conv.Name, err)
			// Continue with other conversations
		}
	}

	return results, nil
}

// GetRootFolderURL returns the URL of the root export folder.
func (e *Exporter) GetRootFolderURL() string {
	if e.index != nil {
		return e.index.RootFolderURL
	}
	return ""
}

// ExportResult contains the results of exporting a conversation.
type ExportResult struct {
	ConversationID  string
	Name            string
	FolderURL       string
	MessageCount    int
	DocsCreated     int
	ThreadsExported int
	Duration        time.Duration
	Error           error
}

// String returns a summary of the export result.
func (r *ExportResult) String() string {
	status := "OK"
	if r.Error != nil {
		status = fmt.Sprintf("ERROR: %v", r.Error)
	}
	return fmt.Sprintf("%s: %d messages, %d docs, %d threads (%v) - %s",
		r.Name, r.MessageCount, r.DocsCreated, r.ThreadsExported, r.Duration, status)
}

// orDefault returns s if non-empty, otherwise the fallback.
func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
