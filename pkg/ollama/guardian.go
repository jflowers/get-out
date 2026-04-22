package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Sensitivity category constants define the types of sensitive content
// the classifier can detect. These map to the categories in the batch
// classification prompt (FR-004).
const (
	CategoryHR          = "hr"
	CategoryLegal       = "legal"
	CategoryFinancial   = "financial"
	CategoryHealth      = "health"
	CategoryTermination = "termination"
	CategoryNone        = "none"
)

// ValidCategories is the set of accepted category values for validation.
var ValidCategories = map[string]bool{
	CategoryHR:          true,
	CategoryLegal:       true,
	CategoryFinancial:   true,
	CategoryHealth:      true,
	CategoryTermination: true,
	CategoryNone:        true,
}

// SensitivityResult represents the classification outcome for a single message.
// Each message in a batch classification receives its own SensitivityResult.
type SensitivityResult struct {
	// Index is the 1-based position of the message in the classification batch.
	Index int `json:"index"`

	// Sensitive indicates whether the message was classified as sensitive.
	Sensitive bool `json:"sensitive"`

	// Category is the type of sensitivity detected.
	// One of: "hr", "legal", "financial", "health", "termination", "none".
	Category string `json:"category"`

	// Confidence is the classifier's confidence score (0.0 to 1.0).
	Confidence float64 `json:"confidence"`

	// Reasoning is a brief explanation of the classification decision.
	Reasoning string `json:"reasoning"`
}

// Guardian classifies messages for sensitivity using an Ollama-backed
// language model. It builds batch prompts, sends them to the model,
// and parses structured JSON responses.
//
// Design: Guardian is a thin orchestrator that owns prompt construction,
// response parsing, and retry logic. The actual HTTP communication is
// delegated to the injected Client (Separation of Concerns / SRP).
type Guardian struct {
	client *Client
}

// NewGuardian creates a Guardian backed by the given Ollama client.
func NewGuardian(client *Client) *Guardian {
	return &Guardian{client: client}
}

// Classify sends a batch of messages to the Ollama model for sensitivity
// classification. It returns one SensitivityResult per input message.
//
// Behavior:
//   - Empty input returns an empty slice without calling Ollama.
//   - Empty-text messages are returned as non-sensitive with CategoryNone (FR-013).
//   - On parse/validation failure, retries once with the same prompt (FR-009).
//   - On second failure, returns all messages as sensitive with CategoryNone
//     and Confidence 0.0 (err on caution, FR-014).
func (g *Guardian) Classify(ctx context.Context, messages []string) ([]SensitivityResult, error) {
	if len(messages) == 0 {
		return []SensitivityResult{}, nil
	}

	// Identify which messages have text content and which are empty.
	// Empty messages are pre-classified as non-sensitive (FR-013).
	var textsToClassify []indexedMessage
	results := make([]SensitivityResult, len(messages))

	for i, msg := range messages {
		if strings.TrimSpace(msg) == "" {
			// FR-013: Messages with no text content are treated as non-sensitive.
			results[i] = SensitivityResult{
				Index:      i + 1,
				Sensitive:  false,
				Category:   CategoryNone,
				Confidence: 1.0,
				Reasoning:  "Empty message — no content to classify.",
			}
		} else {
			textsToClassify = append(textsToClassify, indexedMessage{
				originalIndex: i,
				text:          msg,
			})
		}
	}

	// If all messages were empty, no Ollama call needed.
	if len(textsToClassify) == 0 {
		return results, nil
	}

	prompt := buildClassificationPrompt(textsToClassify)

	// First attempt.
	classified, err := g.classifyWithPrompt(ctx, prompt, len(textsToClassify))
	if err != nil {
		// Retry once on parse/validation failure (FR-009).
		classified, err = g.classifyWithPrompt(ctx, prompt, len(textsToClassify))
		if err != nil {
			// Second failure: err on caution — treat all as sensitive.
			return g.fallbackAllSensitive(messages), nil
		}
	}

	// Map classified results back to their original positions.
	for i, im := range textsToClassify {
		result := classified[i]
		result.Index = im.originalIndex + 1
		results[im.originalIndex] = result
	}

	return results, nil
}

// indexedMessage tracks a non-empty message and its original position
// in the input slice, so results can be mapped back after classification.
type indexedMessage struct {
	originalIndex int
	text          string
}

// classifyWithPrompt sends the prompt to Ollama and parses/validates the response.
// Returns an error if parsing or validation fails (caller handles retry).
func (g *Guardian) classifyWithPrompt(ctx context.Context, prompt string, expectedCount int) ([]SensitivityResult, error) {
	response, err := g.client.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("guardian: classify: %w", err)
	}

	results, err := parseSensitivityResponse(response)
	if err != nil {
		return nil, fmt.Errorf("guardian: parse response: %w", err)
	}

	if len(results) != expectedCount {
		return nil, fmt.Errorf("guardian: expected %d results, got %d", expectedCount, len(results))
	}

	return results, nil
}

// fallbackAllSensitive returns all messages as sensitive with CategoryNone
// and Confidence 0.0. This is the safe fallback when classification fails
// twice (FR-009, FR-014: err on the side of caution).
func (g *Guardian) fallbackAllSensitive(messages []string) []SensitivityResult {
	results := make([]SensitivityResult, len(messages))
	for i := range messages {
		results[i] = SensitivityResult{
			Index:      i + 1,
			Sensitive:  true,
			Category:   CategoryNone,
			Confidence: 0.0,
			Reasoning:  "Classification failed — treating as sensitive (err on caution).",
		}
	}
	return results
}

// buildClassificationPrompt constructs the batch prompt per research.md Decision 2.
// Messages are numbered starting from 1 for reliable mapping back to results.
// The prompt includes the individual-vs-policy distinction (FR-015) and
// err-on-caution instruction (FR-014).
func buildClassificationPrompt(messages []indexedMessage) string {
	var b strings.Builder

	b.WriteString(`You are a content sensitivity classifier for workplace messages.

Classify each message below as either SENSITIVE or NOT_SENSITIVE.

A message is SENSITIVE if it contains:
- HR matters about specific individuals (performance issues, complaints, disciplinary actions)
- Salary, compensation, or bonus details for specific people
- Legal matters (lawsuits, legal holds, attorney communications)
- Health information about specific individuals
- Termination or layoff discussions about specific people
- Financial data that is not publicly available

A message is NOT SENSITIVE if it contains:
- General policy discussions ("updating the PTO policy")
- Public announcements or team updates
- Technical discussions, code reviews, project planning
- General business operations without individual-specific details
- Social conversations, jokes, or casual chat

IMPORTANT: Distinguish between content about specific individuals and general policy discussions. For example, "updating the PTO policy" is NOT sensitive because it discusses policy in general. However, "Sarah's excessive absences" IS sensitive because it references a specific individual's behavior. Always ask: does this message identify or single out a specific person in a sensitive context?

When uncertain, classify as SENSITIVE (err on the side of caution).

Messages to classify:
`)

	for i, msg := range messages {
		fmt.Fprintf(&b, "[%d] %q\n", i+1, msg.text)
	}

	b.WriteString(`
Respond with a JSON array. Each element must have:
- "index": the message number (1-based)
- "sensitive": true or false
- "category": one of "hr", "legal", "financial", "health", "termination", "none"
- "confidence": a number between 0.0 and 1.0
- "reasoning": a brief explanation (one sentence)

Example response:
[
  {"index": 1, "sensitive": false, "category": "none", "confidence": 0.95, "reasoning": "General project update with no personal information."},
  {"index": 2, "sensitive": true, "category": "hr", "confidence": 0.88, "reasoning": "Discusses a specific employee's performance review."}
]

Respond ONLY with the JSON array. No other text.`)

	return b.String()
}

// parseSensitivityResponse extracts and validates a JSON array of
// SensitivityResult from the model's response text. It handles
// markdown code fences (```json ... ```) that models sometimes wrap
// around JSON output.
func parseSensitivityResponse(response string) ([]SensitivityResult, error) {
	cleaned := stripMarkdownCodeFence(response)

	var results []SensitivityResult
	if err := json.Unmarshal([]byte(cleaned), &results); err != nil {
		return nil, fmt.Errorf("unmarshal sensitivity response: %w", err)
	}

	// Validate each result's category and clamp confidence to [0, 1].
	for i := range results {
		if !ValidCategories[results[i].Category] {
			return nil, fmt.Errorf("invalid category %q at index %d", results[i].Category, i)
		}
		results[i].Confidence = math.Max(0.0, math.Min(1.0, results[i].Confidence))
	}

	return results, nil
}

// stripMarkdownCodeFence removes markdown code fences from around JSON.
// Models sometimes respond with ```json\n...\n``` wrapping.
func stripMarkdownCodeFence(s string) string {
	s = strings.TrimSpace(s)

	// Handle ```json ... ``` or ``` ... ```
	if strings.HasPrefix(s, "```") {
		// Remove opening fence line.
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove closing fence.
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}

	return s
}
