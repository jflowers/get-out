package exporter

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/jflowers/get-out/pkg/chrome"
	"github.com/jflowers/get-out/pkg/config"
	"github.com/jflowers/get-out/pkg/gdrive"
	"github.com/jflowers/get-out/pkg/parser"
	"github.com/jflowers/get-out/pkg/secrets"
	"github.com/jflowers/get-out/pkg/slackapi"
)

// Exporter orchestrates the export of Slack conversations to Google Docs.
type Exporter struct {
	// Configuration
	configDir             string
	rootFolderName        string
	rootFolderID          string
	googleCredentialsFile string

	// Clients
	slackClient  *slackapi.Client
	gdriveClient *gdrive.Client

	// Helpers
	folderStructure *FolderStructure
	docWriter       *DocWriter
	mdWriter        *MarkdownWriter
	userResolver    *parser.UserResolver
	channelResolver *parser.ChannelResolver
	personResolver  *parser.PersonResolver
	index           *ExportIndex

	// Local markdown export
	localExportDir string

	// Progress callback
	onProgress func(msg string)

	// Options
	debug      bool
	dateFrom   string // Slack timestamp: only messages after this
	dateTo     string // Slack timestamp: only messages before this
	syncMode   bool   // Use LastMessageTS from index as oldest
	resumeMode bool   // Resume incomplete exports, skip completed ones
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

	// Date range, sync, and resume options
	DateFrom   string // Slack timestamp: only export messages after this
	DateTo     string // Slack timestamp: only export messages before this
	SyncMode   bool   // Only export messages since last successful export
	ResumeMode bool   // Resume incomplete exports, skip completed ones

	// Local markdown export directory (expanded absolute path)
	LocalExportDir string
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
		debug:                 cfg.Debug,
		onProgress:            cfg.OnProgress,
		dateFrom:              cfg.DateFrom,
		dateTo:                cfg.DateTo,
		syncMode:              cfg.SyncMode,
		resumeMode:            cfg.ResumeMode,
		localExportDir:        cfg.LocalExportDir,
		userResolver:          parser.NewUserResolver(),
		channelResolver:       parser.NewChannelResolver(),
	}
}

// InitializeWithStore sets up connections to Chrome/Slack and Google Drive,
// using a SecretStore for credential and token I/O. This is the preferred
// function for CLI commands where a SecretStore has been initialized.
func (e *Exporter) InitializeWithStore(ctx context.Context, chromePort int, store secrets.SecretStore) error {
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
	gdriveClient, err := gdrive.NewClientFromStore(ctx, gdriveCfg, store)
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

	e.slackClient = slackapi.NewBrowserClient(creds.Token, creds.Cookie)
	e.slackClient.SetDebug(e.debug)

	e.folderStructure = NewFolderStructure(e.gdriveClient, e.index, &FolderStructureConfig{
		RootFolderName: e.rootFolderName,
		RootFolderID:   e.rootFolderID,
	})

	e.loadPersonResolver()

	e.docWriter = NewDocWriter(e.gdriveClient, e.slackClient, e.userResolver, e.channelResolver, e.personResolver, e.index.LookupDocURL, e.index.LookupThreadURL)

	// Initialize MarkdownWriter for local markdown export when configured
	if e.localExportDir != "" {
		e.mdWriter = NewMarkdownWriter(e.userResolver, e.channelResolver, e.personResolver)
	}

	return nil
}

// loadPersonResolver loads people.json and creates a PersonResolver for @mention linking.
func (e *Exporter) loadPersonResolver() {
	peoplePath := filepath.Join(e.configDir, "people.json")
	people, err := config.LoadPeople(peoplePath)
	if err != nil {
		e.Progress("Note: people.json not found, @mentions won't have Google links")
		return
	}
	e.personResolver = parser.NewPersonResolver(people)
	if e.personResolver.Count() > 0 {
		e.Progress("Loaded %d person→email mappings for @mention linking", e.personResolver.Count())
	}
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

// determineExportRange returns the oldest and latest Slack timestamps for
// the export window based on sync mode, date flags, or defaults (full export).
func (e *Exporter) determineExportRange(convExport *ConversationExport) (oldest, latest string) {
	if e.syncMode {
		if convExport.LastMessageTS != "" {
			oldest = convExport.LastMessageTS
			e.Progress("Syncing from last export timestamp: %s", oldest)
		} else {
			e.Progress("No previous export found, fetching all messages")
		}
		return oldest, latest
	}

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
	return oldest, latest
}

// exportThreads exports all thread parents found in the message batch and
// returns the count of threads processed.
func (e *Exporter) exportThreads(ctx context.Context, convID string, allMessages []slackapi.Message) int {
	threadParents := GetThreadParents(allMessages)
	if len(threadParents) == 0 {
		return 0
	}

	e.Progress("Exporting %d threads...", len(threadParents))
	exported := 0
	for _, parent := range threadParents {
		if err := e.exportThread(ctx, convID, parent); err != nil {
			e.Progress("Warning: failed to export thread %s: %v", parent.TS, err)
		}
		exported++
	}
	return exported
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

	// Set status to in_progress — hold the per-struct mutex so concurrent
	// Save() calls that marshal this struct see a consistent snapshot.
	convExport.mu.Lock()
	convExport.Status = "in_progress"
	convExport.mu.Unlock()

	// Determine oldest/latest bounds
	oldest, latest := e.determineExportRange(convExport)

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
		e.Progress("No new messages to export for %s", conv.Name)
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

	// Export threads first so we have links for the daily docs
	result.ThreadsExported = e.exportThreads(ctx, conv.ID, allMessages)

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
		if err := e.docWriter.WriteMessages(ctx, docExport.DocID, conv.ID, convExport.FolderID, msgs); err != nil {
			return result, fmt.Errorf("failed to write messages for %s: %w", date, err)
		}

		result.DocsCreated++
		result.MessageCount += len(msgs)
		e.Progress("Wrote %d messages to %s", len(msgs), date)

		// Save checkpoint after each daily doc — hold the per-struct mutex so
		// the index-level Save() sees a consistent view of this struct's fields.
		// Save() itself also acquires convExport.mu, so we must release it first.
		convExport.mu.Lock()
		docExport.MessageCount += len(msgs)
		if len(msgs) > 0 {
			docExport.LastMessageTS = msgs[len(msgs)-1].TS
		}
		if len(allMessages) > 0 {
			convExport.LastMessageTS = allMessages[0].TS
			convExport.MessageCount += len(msgs)
			convExport.LastUpdated = time.Now()
		}
		convExport.mu.Unlock()
		if err := e.index.Save(); err != nil {
			e.Progress("Warning: failed to save checkpoint: %v", err)
		}

		// Write local markdown if configured and conversation opted in
		if e.mdWriter != nil && e.localExportDir != "" && conv.LocalExport {
			mdContent, mdErr := e.mdWriter.RenderDailyDoc(conv.Name, string(conv.Type), date, msgs)
			if mdErr != nil {
				e.Progress("Warning: failed to render markdown for %s: %v", date, mdErr)
				result.MarkdownErrors++
			} else {
				typeName := SanitizeDirectoryName(string(conv.Type), conv.Name)
				if writeErr := WriteMarkdownFile(e.localExportDir, typeName, date, mdContent); writeErr != nil {
					e.Progress("Warning: failed to write markdown for %s: %v", date, writeErr)
					result.MarkdownErrors++
				} else {
					result.MarkdownFilesWritten++
				}
			}
		}
	}

	// Update conversation export state — mark as complete.
	convExport.mu.Lock()
	convExport.Status = "complete"
	convExport.LastUpdated = time.Now()
	if len(allMessages) > 0 {
		convExport.LastMessageTS = allMessages[0].TS // Messages come in reverse order
	}
	convExport.mu.Unlock()

	// Save final index
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
	// Resolve the raw Slack mrkdwn to readable text for the folder name
	resolvedText, _ := parser.ConvertMrkdwnWithLinks(parent.Text, e.userResolver, e.channelResolver, e.personResolver, nil)
	topicPreview := truncate(resolvedText, 40)
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

		if err := e.docWriter.WriteMessages(ctx, docExport.DocID, convID, threadExport.FolderID, msgs); err != nil {
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
	// Pre-validate connections before starting the long export
	if err := e.ValidateConnections(ctx); err != nil {
		return nil, fmt.Errorf("pre-export validation failed: %w", err)
	}

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
		// In resume mode, skip conversations that are already complete
		if e.resumeMode {
			if existing := e.index.GetConversation(conv.ID); existing != nil && existing.Status == "complete" {
				e.Progress("Skipping completed conversation %d/%d: %s", i+1, len(conversations), conv.Name)
				results = append(results, &ExportResult{
					ConversationID: conv.ID,
					Name:           conv.Name,
					FolderURL:      existing.FolderURL,
					Skipped:        true,
				})
				continue
			}
		}

		e.Progress("Exporting conversation %d/%d: %s", i+1, len(conversations), conv.Name)

		result, err := e.ExportConversation(ctx, conv)
		results = append(results, result)

		if err != nil {
			result.Error = err
			e.Progress("Error exporting %s: %v", conv.Name, err)
			// Continue with other conversations
		}
	}

	// Second pass: resolve cross-conversation Slack links — but only if any
	// conversations actually exported new messages.
	hasNewContent := false
	for _, r := range results {
		if r.MessageCount > 0 {
			hasNewContent = true
			break
		}
	}

	if hasNewContent {
		if replaced, err := e.ResolveCrossLinks(ctx); err != nil {
			e.Progress("Warning: cross-link resolution had errors: %v", err)
		} else if replaced > 0 {
			e.Progress("Resolved %d cross-conversation links", replaced)
		}
	}

	return results, nil
}

// clampConcurrency validates and clamps the maxConcurrent parameter to [1, 5].
func clampConcurrency(maxConcurrent int) int {
	if maxConcurrent < 1 {
		return 1
	}
	if maxConcurrent > 5 {
		return 5
	}
	return maxConcurrent
}

// collectParallelResults filters nil entries from a results slice.
func collectParallelResults(results []*ExportResult) []*ExportResult {
	var filtered []*ExportResult
	for _, r := range results {
		if r != nil {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// ExportAllParallel exports conversations concurrently with a max parallelism limit.
// maxConcurrent controls how many conversations are exported at the same time (1-5).
func (e *Exporter) ExportAllParallel(ctx context.Context, conversations []config.ConversationConfig, maxConcurrent int) ([]*ExportResult, error) {
	maxConcurrent = clampConcurrency(maxConcurrent)

	// If parallelism is 1, use the simpler sequential path
	if maxConcurrent == 1 {
		return e.ExportAll(ctx, conversations)
	}

	// Pre-validate connections
	if err := e.ValidateConnections(ctx); err != nil {
		return nil, fmt.Errorf("pre-export validation failed: %w", err)
	}

	// Load users for all conversations first (sequential)
	channelIDs := make([]string, len(conversations))
	for i, conv := range conversations {
		channelIDs[i] = conv.ID
	}
	if err := e.LoadUsersForConversations(ctx, channelIDs); err != nil {
		return nil, err
	}

	// Semaphore channel to limit concurrency
	sem := make(chan struct{}, maxConcurrent)

	// Results with mutex for safe concurrent access
	var mu sync.Mutex
	results := make([]*ExportResult, len(conversations))

	var wg sync.WaitGroup

outer:
	for i, conv := range conversations {
		// Check if context cancelled — use a labeled break to exit the for loop,
		// not just the select (bare break inside select only exits the select).
		select {
		case <-ctx.Done():
			break outer
		default:
		}

		// In resume mode, skip completed conversations
		if e.resumeMode {
			if existing := e.index.GetConversation(conv.ID); existing != nil && existing.Status == "complete" {
				e.Progress("Skipping completed conversation %d/%d: %s", i+1, len(conversations), conv.Name)
				results[i] = &ExportResult{
					ConversationID: conv.ID,
					Name:           conv.Name,
					FolderURL:      existing.FolderURL,
					Skipped:        true,
				}
				continue
			}
		}

		wg.Add(1)
		go func(idx int, c config.ConversationConfig) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			e.Progress("[parallel %d/%d] Exporting: %s", idx+1, len(conversations), c.Name)

			result, err := e.ExportConversation(ctx, c)
			if err != nil {
				result.Error = err
				e.Progress("[parallel %d/%d] Error: %s: %v", idx+1, len(conversations), c.Name, err)
			}

			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, conv)
	}

	wg.Wait()

	// Second pass: resolve cross-conversation links — but only if any
	// conversations actually exported new messages (skip when sync mode
	// found nothing new, to avoid scanning hundreds of docs pointlessly).
	collected := collectParallelResults(results)
	hasNewContent := false
	for _, r := range collected {
		if r.MessageCount > 0 {
			hasNewContent = true
			break
		}
	}

	if hasNewContent {
		if replaced, err := e.ResolveCrossLinks(ctx); err != nil {
			e.Progress("Warning: cross-link resolution had errors: %v", err)
		} else if replaced > 0 {
			e.Progress("Resolved %d cross-conversation links", replaced)
		}
	}

	return collected, nil
}

// ResolveCrossLinks scans all exported docs for remaining Slack message links
// and replaces them with Google Docs URLs using the now-complete index.
// This handles forward references where conversation B was referenced in conversation A
// before conversation B was exported.
func (e *Exporter) ResolveCrossLinks(ctx context.Context) (int, error) {
	if e.index == nil || e.gdriveClient == nil {
		return 0, nil
	}

	// Count total docs to scan for progress reporting
	totalDocs := 0
	for _, conv := range e.index.Conversations {
		for _, doc := range conv.DailyDocs {
			if doc.DocID != "" {
				totalDocs++
			}
		}
		for _, thread := range conv.Threads {
			for _, doc := range thread.DailyDocs {
				if doc.DocID != "" {
					totalDocs++
				}
			}
		}
	}

	if totalDocs == 0 {
		return 0, nil
	}

	e.Progress("Resolving cross-conversation links across %d docs...", totalDocs)

	totalReplaced := 0
	scanned := 0

	for _, conv := range e.index.Conversations {
		for _, doc := range conv.DailyDocs {
			if doc.DocID == "" {
				continue
			}

			if ctx.Err() != nil {
				return totalReplaced, ctx.Err()
			}

			replaced, err := e.resolveLinksInDoc(ctx, doc.DocID)
			if err != nil {
				if e.debug {
					e.Progress("Warning: could not resolve links in doc %s: %v", doc.DocID, err)
				}
				continue
			}
			totalReplaced += replaced
			scanned++

			if scanned%10 == 0 {
				e.Progress("Scanning docs for cross-links... %d/%d", scanned, totalDocs)
			}
		}

		// Also check thread docs
		for _, thread := range conv.Threads {
			for _, doc := range thread.DailyDocs {
				if doc.DocID == "" {
					continue
				}

				if ctx.Err() != nil {
					return totalReplaced, ctx.Err()
				}

				replaced, err := e.resolveLinksInDoc(ctx, doc.DocID)
				if err != nil {
					if e.debug {
						e.Progress("Warning: could not resolve links in thread doc %s: %v", doc.DocID, err)
					}
					continue
				}
				totalReplaced += replaced
				scanned++

				if scanned%10 == 0 {
					e.Progress("Scanning docs for cross-links... %d/%d", scanned, totalDocs)
				}
			}
		}
	}

	return totalReplaced, nil
}

// resolveLinksInDoc reads a doc, finds Slack links, and replaces them with Google Docs URLs.
func (e *Exporter) resolveLinksInDoc(ctx context.Context, docID string) (int, error) {
	content, err := e.gdriveClient.GetDocumentContent(ctx, docID)
	if err != nil {
		return 0, err
	}

	// Find Slack links in the content
	links := parser.FindSlackLinks(content)
	if len(links) == 0 {
		return 0, nil
	}

	// Build replacement map for links that can now be resolved
	replacements := make(map[string]string)
	for _, link := range links {
		docsURL := e.index.LookupDocURL(link.ChannelID, link.MessageTS)
		if docsURL != "" && docsURL != link.FullURL {
			replacements[link.FullURL] = docsURL
		}
	}

	if len(replacements) == 0 {
		return 0, nil
	}

	return e.gdriveClient.ReplaceText(ctx, docID, replacements)
}

// ValidateConnections checks that both Slack and Google connections are alive
// before starting a potentially long export. Returns clear error messages
// if the session has expired or the token needs refreshing.
func (e *Exporter) ValidateConnections(ctx context.Context) error {
	if e.slackClient == nil {
		return fmt.Errorf("Slack client not initialized")
	}

	e.Progress("Validating Slack session...")
	authResp, err := e.slackClient.ValidateAuth(ctx)
	if err != nil {
		return fmt.Errorf("Slack session expired or invalid: %w\n\nPlease refresh your Slack session in the browser and try again", err)
	}
	e.Progress("Slack session valid: %s @ %s", authResp.User, authResp.Team)

	return nil
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
	Skipped         bool // True if skipped during --resume (already complete)

	// Local markdown export stats
	MarkdownFilesWritten int
	MarkdownErrors       int
}

// String returns a summary of the export result.
func (r *ExportResult) String() string {
	if r.Skipped {
		return fmt.Sprintf("%s: skipped (already complete)", r.Name)
	}
	status := "OK"
	if r.Error != nil {
		status = fmt.Sprintf("ERROR: %v", r.Error)
	}
	summary := fmt.Sprintf("%s: %d messages, %d docs, %d threads",
		r.Name, r.MessageCount, r.DocsCreated, r.ThreadsExported)
	if r.MarkdownFilesWritten > 0 || r.MarkdownErrors > 0 {
		summary += fmt.Sprintf(", %d md files", r.MarkdownFilesWritten)
		if r.MarkdownErrors > 0 {
			summary += fmt.Sprintf(" (%d md errors)", r.MarkdownErrors)
		}
	}
	return fmt.Sprintf("%s (%v) - %s", summary, r.Duration, status)
}

// orDefault returns s if non-empty, otherwise the fallback.
func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
