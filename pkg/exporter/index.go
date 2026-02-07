// Package exporter handles the export of Slack messages to Google Docs.
package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ExportIndex tracks all exported content for checkpointing and link resolution.
type ExportIndex struct {
	mu sync.RWMutex

	// RootFolderID is the ID of the root "Slack Exports" folder in Drive
	RootFolderID  string `json:"root_folder_id"`
	RootFolderURL string `json:"root_folder_url"`

	// Conversations maps conversation ID to its export state
	Conversations map[string]*ConversationExport `json:"conversations"`

	// Users maps Slack user ID to cached user info
	Users map[string]*UserCache `json:"users"`

	// UpdatedAt is the last time this index was modified
	UpdatedAt time.Time `json:"updated_at"`

	// path is where this index is saved (not serialized)
	path string
}

// ConversationExport tracks the export state of a single conversation.
type ConversationExport struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"` // dm, mpim, channel, private_channel
	FolderID        string `json:"folder_id"`
	FolderURL       string `json:"folder_url"`
	ThreadsFolderID string `json:"threads_folder_id,omitempty"`

	// Status tracks export completion: "in_progress" or "complete"
	Status string `json:"status"`

	// DailyDocs maps date string (YYYY-MM-DD) to doc info
	DailyDocs map[string]*DocExport `json:"daily_docs"`

	// Threads maps thread_ts to thread export info
	Threads map[string]*ThreadExport `json:"threads"`

	// LastMessageTS is the timestamp of the last exported message
	LastMessageTS string `json:"last_message_ts"`

	// MessageCount is the total number of messages exported
	MessageCount int `json:"message_count"`

	// LastUpdated is when this conversation was last exported
	LastUpdated time.Time `json:"last_updated"`
}

// DocExport tracks a single Google Doc.
type DocExport struct {
	DocID  string `json:"doc_id"`
	DocURL string `json:"doc_url"`
	Title  string `json:"title"`
	Date   string `json:"date,omitempty"` // For daily docs

	// LastMessageTS is the last message timestamp in this doc
	LastMessageTS string `json:"last_message_ts,omitempty"`

	// MessageCount in this doc
	MessageCount int `json:"message_count"`
}

// ThreadExport tracks an exported thread.
type ThreadExport struct {
	ThreadTS   string `json:"thread_ts"`
	FolderID   string `json:"folder_id"`
	FolderURL  string `json:"folder_url"`
	FolderName string `json:"folder_name"`

	// DailyDocs for this thread (threads can span multiple days)
	DailyDocs map[string]*DocExport `json:"daily_docs"`

	// ReplyCount is the number of replies exported
	ReplyCount int `json:"reply_count"`

	// LastReplyTS is the timestamp of the last exported reply
	LastReplyTS string `json:"last_reply_ts"`
}

// UserCache caches Slack user information.
type UserCache struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	RealName    string    `json:"real_name"`
	DisplayName string    `json:"display_name"`
	IsBot       bool      `json:"is_bot"`
	CachedAt    time.Time `json:"cached_at"`
}

// NewExportIndex creates a new empty export index.
func NewExportIndex(path string) *ExportIndex {
	return &ExportIndex{
		Conversations: make(map[string]*ConversationExport),
		Users:         make(map[string]*UserCache),
		UpdatedAt:     time.Now(),
		path:          path,
	}
}

// LoadExportIndex loads an export index from a file, or creates a new one.
func LoadExportIndex(path string) (*ExportIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewExportIndex(path), nil
		}
		return nil, fmt.Errorf("failed to read export index: %w", err)
	}

	var index ExportIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse export index: %w", err)
	}

	index.path = path

	// Initialize maps if nil (for backwards compatibility)
	if index.Conversations == nil {
		index.Conversations = make(map[string]*ConversationExport)
	}
	if index.Users == nil {
		index.Users = make(map[string]*UserCache)
	}

	return &index, nil
}

// Save writes the export index to disk.
func (idx *ExportIndex) Save() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.UpdatedAt = time.Now()

	// Ensure directory exists
	dir := filepath.Dir(idx.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal export index: %w", err)
	}

	if err := os.WriteFile(idx.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write export index: %w", err)
	}

	return nil
}

// GetConversation returns the export state for a conversation.
func (idx *ExportIndex) GetConversation(id string) *ConversationExport {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.Conversations[id]
}

// SetConversation updates the export state for a conversation.
func (idx *ExportIndex) SetConversation(conv *ConversationExport) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.Conversations[conv.ID] = conv
}

// AllConversations returns all conversation exports sorted by name.
func (idx *ExportIndex) AllConversations() []*ConversationExport {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	convs := make([]*ConversationExport, 0, len(idx.Conversations))
	for _, c := range idx.Conversations {
		convs = append(convs, c)
	}
	sort.Slice(convs, func(i, j int) bool {
		return convs[i].Name < convs[j].Name
	})
	return convs
}

// GetOrCreateConversation returns an existing conversation export or creates a new one.
func (idx *ExportIndex) GetOrCreateConversation(id, name, convType string) *ConversationExport {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if conv, ok := idx.Conversations[id]; ok {
		return conv
	}

	conv := &ConversationExport{
		ID:        id,
		Name:      name,
		Type:      convType,
		DailyDocs: make(map[string]*DocExport),
		Threads:   make(map[string]*ThreadExport),
	}
	idx.Conversations[id] = conv
	return conv
}

// GetUser returns cached user info.
func (idx *ExportIndex) GetUser(id string) *UserCache {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.Users[id]
}

// SetUser caches user info.
func (idx *ExportIndex) SetUser(user *UserCache) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	user.CachedAt = time.Now()
	idx.Users[user.ID] = user
}

// GetDailyDoc returns the doc for a specific date in a conversation.
func (idx *ExportIndex) GetDailyDoc(convID, date string) *DocExport {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	conv, ok := idx.Conversations[convID]
	if !ok {
		return nil
	}
	return conv.DailyDocs[date]
}

// SetDailyDoc sets the doc for a specific date in a conversation.
func (idx *ExportIndex) SetDailyDoc(convID, date string, doc *DocExport) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	conv, ok := idx.Conversations[convID]
	if !ok {
		return
	}
	if conv.DailyDocs == nil {
		conv.DailyDocs = make(map[string]*DocExport)
	}
	conv.DailyDocs[date] = doc
}

// GetThread returns thread export info.
func (idx *ExportIndex) GetThread(convID, threadTS string) *ThreadExport {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	conv, ok := idx.Conversations[convID]
	if !ok {
		return nil
	}
	return conv.Threads[threadTS]
}

// SetThread sets thread export info.
func (idx *ExportIndex) SetThread(convID string, thread *ThreadExport) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	conv, ok := idx.Conversations[convID]
	if !ok {
		return
	}
	if conv.Threads == nil {
		conv.Threads = make(map[string]*ThreadExport)
	}
	conv.Threads[thread.ThreadTS] = thread
}

// LookupDocURL finds the Google Docs URL for a Slack message.
// Used for replacing Slack links with Google Docs links.
func (idx *ExportIndex) LookupDocURL(convID, messageTS string) string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	conv, ok := idx.Conversations[convID]
	if !ok {
		return ""
	}

	// Convert timestamp to date and look up daily doc
	date := tsToDate(messageTS)
	if doc, ok := conv.DailyDocs[date]; ok {
		return doc.DocURL
	}

	return conv.FolderURL
}

// LookupThreadURL finds the Google Docs URL for a thread.
func (idx *ExportIndex) LookupThreadURL(convID, threadTS string) string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	conv, ok := idx.Conversations[convID]
	if !ok {
		return ""
	}

	if thread, ok := conv.Threads[threadTS]; ok {
		return thread.FolderURL
	}

	return ""
}

// LookupConversationURL finds the Google Drive folder URL for a conversation.
func (idx *ExportIndex) LookupConversationURL(convID string) string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if conv, ok := idx.Conversations[convID]; ok {
		return conv.FolderURL
	}
	return ""
}

// tsToDate converts a Slack timestamp to a date string (YYYY-MM-DD).
func tsToDate(ts string) string {
	// Parse the Unix timestamp part (before the dot)
	var sec int64
	for i := 0; i < len(ts); i++ {
		if ts[i] == '.' {
			break
		}
		sec = sec*10 + int64(ts[i]-'0')
	}
	t := time.Unix(sec, 0)
	return t.Format("2006-01-02")
}

// DefaultIndexPath returns the default path for the export index.
func DefaultIndexPath(configDir string) string {
	return filepath.Join(configDir, "_metadata", "export-index.json")
}
