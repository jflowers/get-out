// Package util provides utilities for rate limiting, checkpointing, and file I/O.
package util

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Checkpoint tracks export progress for resumability.
type Checkpoint struct {
	mu       sync.RWMutex
	filePath string
	data     CheckpointData
	dirty    bool
}

// CheckpointData holds the persistent checkpoint state.
type CheckpointData struct {
	Version          int                       `json:"version"`
	LastUpdated      time.Time                 `json:"last_updated"`
	Conversations    map[string]ConvCheckpoint `json:"conversations"`
	CompletedConvs   []string                  `json:"completed_conversations"`
	TotalMessages    int                       `json:"total_messages"`
	ExportedMessages int                       `json:"exported_messages"`
}

// ConvCheckpoint tracks progress for a single conversation.
type ConvCheckpoint struct {
	ConversationID   string    `json:"conversation_id"`
	ConversationName string    `json:"conversation_name"`
	LastTimestamp    string    `json:"last_timestamp"` // Slack ts format
	MessageCount     int       `json:"message_count"`
	Completed        bool      `json:"completed"`
	LastUpdated      time.Time `json:"last_updated"`
}

// NewCheckpoint creates or loads a checkpoint file.
func NewCheckpoint(dir string) (*Checkpoint, error) {
	filePath := filepath.Join(dir, ".get-out-checkpoint.json")

	cp := &Checkpoint{
		filePath: filePath,
		data: CheckpointData{
			Version:       1,
			Conversations: make(map[string]ConvCheckpoint),
		},
	}

	// Try to load existing checkpoint
	if data, err := os.ReadFile(filePath); err == nil {
		if err := json.Unmarshal(data, &cp.data); err != nil {
			return nil, fmt.Errorf("failed to parse checkpoint file: %w", err)
		}
	}

	return cp, nil
}

// GetConversationCheckpoint returns the checkpoint for a conversation.
func (c *Checkpoint) GetConversationCheckpoint(convID string) *ConvCheckpoint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if cp, ok := c.data.Conversations[convID]; ok {
		return &cp
	}
	return nil
}

// UpdateConversation updates the checkpoint for a conversation.
func (c *Checkpoint) UpdateConversation(convID, convName, lastTS string, msgCount int, completed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing := c.data.Conversations[convID]
	c.data.Conversations[convID] = ConvCheckpoint{
		ConversationID:   convID,
		ConversationName: convName,
		LastTimestamp:    lastTS,
		MessageCount:     existing.MessageCount + msgCount,
		Completed:        completed,
		LastUpdated:      time.Now(),
	}

	if completed && !contains(c.data.CompletedConvs, convID) {
		c.data.CompletedConvs = append(c.data.CompletedConvs, convID)
	}

	c.data.ExportedMessages += msgCount
	c.dirty = true
}

// IsConversationCompleted checks if a conversation export is complete.
func (c *Checkpoint) IsConversationCompleted(convID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if cp, ok := c.data.Conversations[convID]; ok {
		return cp.Completed
	}
	return false
}

// SetTotalMessages sets the total message count for progress tracking.
func (c *Checkpoint) SetTotalMessages(total int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data.TotalMessages = total
	c.dirty = true
}

// GetProgress returns export progress as a percentage.
func (c *Checkpoint) GetProgress() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.data.TotalMessages == 0 {
		return 0
	}
	return float64(c.data.ExportedMessages) / float64(c.data.TotalMessages) * 100
}

// GetStats returns checkpoint statistics.
func (c *Checkpoint) GetStats() (completed, total int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data.CompletedConvs), len(c.data.Conversations)
}

// Save persists the checkpoint to disk.
func (c *Checkpoint) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.dirty {
		return nil
	}

	c.data.LastUpdated = time.Now()

	data, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	// Write atomically
	tmpPath := c.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint: %w", err)
	}

	if err := os.Rename(tmpPath, c.filePath); err != nil {
		return fmt.Errorf("failed to rename checkpoint: %w", err)
	}

	c.dirty = false
	return nil
}

// Reset clears the checkpoint data.
func (c *Checkpoint) Reset() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = CheckpointData{
		Version:       1,
		Conversations: make(map[string]ConvCheckpoint),
	}
	c.dirty = true

	return c.Save()
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
