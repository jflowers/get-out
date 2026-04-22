# Data Model: Sensitivity Filter for Local Markdown Export

**Feature**: `005-sensitivity-filter` | **Date**: 2026-04-21

## Entity Definitions

### SensitivityResult

Represents the classification outcome for a single message.

**Package**: `pkg/ollama`

```go
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
```

**Validation rules**:
- `Index` must be ≥ 1 and ≤ batch size
- `Category` must be one of the defined constants (see below)
- `Confidence` must be in [0.0, 1.0]
- If `Sensitive` is true, `Category` must not be "none"
- If `Sensitive` is false, `Category` must be "none"

**Category constants** (defined in `pkg/ollama/guardian.go`):

```go
const (
    CategoryHR          = "hr"
    CategoryLegal       = "legal"
    CategoryFinancial   = "financial"
    CategoryHealth      = "health"
    CategoryTermination = "termination"
    CategoryNone        = "none"
)

// ValidCategories is the set of accepted category values.
var ValidCategories = map[string]bool{
    CategoryHR:          true,
    CategoryLegal:       true,
    CategoryFinancial:   true,
    CategoryHealth:      true,
    CategoryTermination: true,
    CategoryNone:        true,
}
```

---

### FilterResult

Aggregates filtering outcomes for a batch of messages in a single daily doc.

**Package**: `pkg/exporter`

```go
// FilterResult aggregates the sensitivity filtering outcome for a batch of
// messages (typically one daily doc). It contains the messages that passed
// filtering and metadata about what was filtered out.
type FilterResult struct {
    // PassedMessages contains the messages that were not classified as sensitive.
    // These are the messages that will be written to the markdown file.
    PassedMessages []slackapi.Message

    // FilteredCount is the number of messages that were classified as sensitive
    // and excluded from the markdown output.
    FilteredCount int

    // CategoryBreakdown maps sensitivity category to count of filtered messages.
    // Only categories with count > 0 are included.
    // Example: {"hr": 2, "financial": 1}
    CategoryBreakdown map[string]int

    // TotalCount is the total number of messages in the batch before filtering.
    TotalCount int

    // Results contains the per-message classification results from the classifier.
    // Useful for debugging and audit logging.
    Results []ollama.SensitivityResult
}

// AllFiltered returns true if every message in the batch was classified as
// sensitive, meaning no markdown file should be written (FR-010).
func (f *FilterResult) AllFiltered() bool {
    return len(f.PassedMessages) == 0
}
```

**Relationships**:
- `PassedMessages` is a subset of the input `[]slackapi.Message`
- `FilteredCount` + `len(PassedMessages)` == `TotalCount`
- `CategoryBreakdown` values sum to `FilteredCount`
- `Results` has exactly `TotalCount` entries (one per input message)

---

### OllamaConfig

Represents the Ollama connection settings in `settings.json`.

**Package**: `pkg/config`

```go
// OllamaConfig holds configuration for the Ollama-based sensitivity classifier.
// When present and Enabled is true, sensitivity filtering is active for local
// markdown exports.
type OllamaConfig struct {
    // Enabled controls whether sensitivity filtering is active.
    // Default: false (feature must be explicitly opted into).
    Enabled bool `json:"enabled"`

    // Endpoint is the Ollama REST API base URL.
    // Default: "http://localhost:11434"
    Endpoint string `json:"endpoint,omitempty"`

    // Model is the Ollama model to use for classification.
    // Default: "granite-guardian:8b"
    Model string `json:"model,omitempty"`
}

// DefaultOllamaEndpoint is the default Ollama REST API endpoint.
const DefaultOllamaEndpoint = "http://localhost:11434"

// DefaultOllamaModel is the default model for sensitivity classification.
const DefaultOllamaModel = "granite-guardian:8b"
```

**Integration with Settings** (in `pkg/config/types.go`):

```go
type Settings struct {
    // ... existing fields ...

    // Ollama configuration for sensitivity filtering (optional).
    // When nil or Enabled is false, sensitivity filtering is disabled.
    Ollama *OllamaConfig `json:"ollama,omitempty"`
}
```

**Example `settings.json`**:

```json
{
  "slackWorkspaceUrl": "https://app.slack.com",
  "logLevel": "INFO",
  "localExportOutputDir": "~/.get-out/export",
  "ollama": {
    "enabled": true,
    "endpoint": "http://localhost:11434",
    "model": "granite-guardian:8b"
  }
}
```

---

### BatchClassificationRequest (internal)

Internal structure for building the Ollama API request. Not exported — used only within `pkg/ollama/guardian.go`.

**Package**: `pkg/ollama` (unexported)

```go
// ollamaGenerateRequest is the request body for POST /api/generate.
type ollamaGenerateRequest struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
    Stream bool   `json:"stream"`
    Format string `json:"format,omitempty"`
}
```

**Notes**:
- `Stream` is always `false` — we want the complete response in one HTTP response
- `Format` is set to `"json"` to hint Ollama to produce JSON output
- The `Prompt` field contains the full batch classification prompt with all messages embedded

---

### BatchClassificationResponse (internal)

Internal structure for parsing the Ollama API response.

**Package**: `pkg/ollama` (unexported)

```go
// ollamaGenerateResponse is the response body from POST /api/generate.
type ollamaGenerateResponse struct {
    Model     string `json:"model"`
    Response  string `json:"response"`
    Done      bool   `json:"done"`
    CreatedAt string `json:"created_at"`

    // Timing metadata (useful for performance monitoring)
    TotalDuration    int64 `json:"total_duration"`
    LoadDuration     int64 `json:"load_duration"`
    PromptEvalCount  int   `json:"prompt_eval_count"`
    EvalCount        int   `json:"eval_count"`
    EvalDuration     int64 `json:"eval_duration"`
}
```

**Notes**:
- `Response` contains the raw text output from the model (the JSON array of `SensitivityResult`)
- `Done` should be `true` for non-streaming responses
- Timing fields are logged at debug level for performance monitoring

---

### MessageFilter Interface

The interface that bridges the Ollama classifier into the export pipeline.

**Package**: `pkg/exporter`

```go
// MessageFilter classifies messages and returns filtering results.
// Implementations may call external services (Ollama) or be no-ops.
//
// The interface enables dependency injection for testing: the export
// pipeline can be tested with a mock filter that returns predetermined
// results without requiring a running Ollama instance.
type MessageFilter interface {
    // FilterMessages classifies the given messages and returns which ones
    // passed filtering, along with metadata about what was filtered.
    //
    // Returns an error if the classification service is unreachable or
    // returns an unrecoverable error. Transient errors (malformed response)
    // are retried once internally before returning an error.
    FilterMessages(ctx context.Context, messages []slackapi.Message) (*FilterResult, error)
}
```

**Implementations**:

| Implementation | Package | Purpose |
|---------------|---------|---------|
| `OllamaFilter` | `pkg/exporter` | Production: calls `pkg/ollama` client for classification |
| (test double) | `pkg/exporter` (test file) | Testing: returns configurable results without Ollama |

---

## Entity Relationship Diagram

```
settings.json
  └── OllamaConfig (optional)
        ├── endpoint → Ollama REST API
        ├── model → granite-guardian:8b
        └── enabled → controls feature activation

Exporter
  ├── messageFilter: MessageFilter (interface)
  │     └── OllamaFilter
  │           └── ollama.Client
  │                 ├── Generate(prompt) → ollamaGenerateResponse
  │                 │     └── Response → []SensitivityResult (parsed)
  │                 ├── Ping() → health check
  │                 └── ModelAvailable() → model check
  │
  └── ExportConversation()
        └── for each date:
              ├── Write Google Doc (ALL messages — unfiltered)
              └── If filter configured:
                    ├── FilterMessages(messages) → FilterResult
                    │     ├── PassedMessages → RenderDailyDoc
                    │     ├── FilteredCount → frontmatter
                    │     └── CategoryBreakdown → frontmatter
                    └── RenderDailyDoc(passedMessages, filterResult)
                          └── YAML frontmatter includes sensitivity section
```

## Data Flow

```
Messages (from Slack API)
    │
    ├──→ Google Docs export (ALL messages, unchanged)
    │
    └──→ Local Markdown export
           │
           ├── [filter disabled] → RenderDailyDoc(all messages) → WriteMarkdownFile
           │
           └── [filter enabled]
                  │
                  ├── MessageFilter.FilterMessages(messages)
                  │     │
                  │     ├── Build batch prompt with all message texts
                  │     ├── POST /api/generate to Ollama
                  │     ├── Parse JSON response → []SensitivityResult
                  │     ├── (on parse failure: retry once)
                  │     ├── (on second failure: treat all as sensitive)
                  │     └── Return FilterResult
                  │
                  ├── If FilterResult.AllFiltered() → skip file (FR-010)
                  │
                  └── RenderDailyDoc(passedMessages, filterResult)
                        └── WriteMarkdownFile (with sensitivity frontmatter)
```
