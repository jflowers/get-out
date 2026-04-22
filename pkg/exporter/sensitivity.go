package exporter

import (
	"context"
	"fmt"

	"github.com/jflowers/get-out/pkg/ollama"
	"github.com/jflowers/get-out/pkg/slackapi"
)

// MessageFilter classifies messages for sensitivity and returns a
// FilterResult separating passed (non-sensitive) messages from filtered
// (sensitive) ones. Implementations may use local models, remote APIs,
// or rule-based heuristics.
//
// Design: Interface abstraction enables dependency injection for testing
// and future alternative filter backends (SOLID Interface Segregation).
type MessageFilter interface {
	FilterMessages(ctx context.Context, messages []slackapi.Message) (*FilterResult, error)
}

// FilterResult holds the outcome of a sensitivity filtering pass over
// a batch of messages.
type FilterResult struct {
	// PassedMessages contains messages that were NOT classified as sensitive.
	PassedMessages []slackapi.Message

	// FilteredCount is the number of messages that were classified as sensitive.
	FilteredCount int

	// CategoryBreakdown maps sensitivity categories (e.g., "hr", "financial")
	// to the count of messages filtered for that category.
	CategoryBreakdown map[string]int

	// TotalCount is the total number of messages evaluated.
	TotalCount int
}

// AllFiltered returns true when every message in the batch was classified
// as sensitive. This signals the caller to skip writing the daily doc
// entirely (no non-sensitive content remains).
func (r *FilterResult) AllFiltered() bool {
	return r.FilteredCount == r.TotalCount && r.TotalCount > 0
}

// OllamaFilter implements MessageFilter using an Ollama-backed Guardian
// classifier. It extracts message text, delegates classification to the
// Guardian, and partitions messages into passed/filtered buckets.
type OllamaFilter struct {
	guardian *ollama.Guardian
}

// NewOllamaFilter creates an OllamaFilter backed by the given Guardian.
func NewOllamaFilter(guardian *ollama.Guardian) *OllamaFilter {
	return &OllamaFilter{guardian: guardian}
}

// FilterMessages classifies each message's text via the Guardian and
// partitions the batch into passed (non-sensitive) and filtered (sensitive)
// messages. Empty-text messages pass through per FR-013 (the Guardian
// already handles this, but the filter respects the result).
//
// On classification error, the error is returned directly — the caller
// decides whether to treat this as a hard gate (FR-007).
func (f *OllamaFilter) FilterMessages(ctx context.Context, messages []slackapi.Message) (*FilterResult, error) {
	result := &FilterResult{
		CategoryBreakdown: make(map[string]int),
		TotalCount:        len(messages),
	}

	if len(messages) == 0 {
		return result, nil
	}

	// Extract text from each message for classification.
	texts := make([]string, len(messages))
	for i, msg := range messages {
		texts[i] = msg.Text
	}

	classifications, err := f.guardian.Classify(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("sensitivity filter: %w", err)
	}

	// Partition messages based on classification results.
	for i, classification := range classifications {
		if classification.Sensitive {
			result.FilteredCount++
			result.CategoryBreakdown[classification.Category]++
		} else {
			result.PassedMessages = append(result.PassedMessages, messages[i])
		}
	}

	return result, nil
}
