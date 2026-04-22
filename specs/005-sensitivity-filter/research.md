# Research: Sensitivity Filter for Local Markdown Export

**Feature**: `005-sensitivity-filter` | **Date**: 2026-04-21

## Decision 1: Ollama Client Design

### Context

The gcal-organizer project (PR #17) implemented a Granite Guardian integration for meeting transcript classification. The question is whether to port that client or build a new one.

### Decision: New minimal HTTP client

**Chosen**: Build a new `pkg/ollama/client.go` from scratch, tailored to get-out's needs.

**Rationale**:
- gcal-organizer uses a different Go module structure and dependency set
- get-out needs only the `/api/generate` endpoint (single-turn generation), not chat or embeddings
- The client is ~80 lines of code — simpler to write fresh than to extract and adapt
- get-out's error handling patterns (context wrapping, progress callbacks) differ from gcal-organizer
- Avoids introducing a cross-project dependency or copy-paste drift

**Rejected alternative**: Port from gcal-organizer
- Would require adapting to different config patterns, error handling, and testing conventions
- The gcal-organizer client has features we don't need (streaming, chat mode)

### Client API Design

```go
// pkg/ollama/client.go

// Client communicates with a local Ollama instance via its REST API.
type Client struct {
    endpoint   string        // e.g., "http://localhost:11434"
    model      string        // e.g., "granite-guardian:8b"
    httpClient *http.Client  // injectable for testing
}

// NewClient creates a Client with the given endpoint and model.
func NewClient(endpoint, model string, opts ...Option) *Client

// Generate sends a prompt and returns the model's text response.
// Returns an error if Ollama is unreachable, the model is not found,
// or the request times out.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error)

// Ping checks that Ollama is reachable and the configured model is available.
// Used by the doctor command and pre-export validation.
func (c *Client) Ping(ctx context.Context) error

// ModelAvailable checks if the configured model is pulled and ready.
func (c *Client) ModelAvailable(ctx context.Context) (bool, error)
```

### Ollama REST API Endpoints Used

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `POST /api/generate` | POST | Send classification prompt, receive JSON response |
| `GET /api/tags` | GET | List available models (for doctor check / `ModelAvailable`) |
| `GET /` | GET | Health check (for `Ping`) |

### HTTP Client Configuration

- **Timeout**: 60 seconds per request (SC-003 allows 5s overhead, but model inference can vary; 60s is the hard timeout per FR edge case)
- **No retries at HTTP level**: Retry logic is at the classification level (retry-once on malformed response per FR-009)
- **Connection**: HTTP/1.1 to localhost (Ollama default); no TLS needed for local
- **Injectable `http.Client`**: For testing with `httptest.Server`

---

## Decision 2: Batch Classification Prompt Design

### Context

Messages need to be classified individually, but sending one Ollama request per message would be too slow (5-20 messages × 2-5s each = 10-100s per daily doc, violating SC-003). Batch classification sends all messages in one prompt and expects a structured JSON response with per-message results.

### Decision: Single structured prompt with JSON array response

**Prompt structure**:
```
You are a content sensitivity classifier for workplace messages.

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

When uncertain, classify as SENSITIVE (err on the side of caution).

Messages to classify:
[1] "message text here"
[2] "message text here"
...

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

Respond ONLY with the JSON array. No other text.
```

**Rationale**:
- One Ollama call per daily doc keeps latency within SC-003 bounds
- JSON array response is parseable and maps directly to `SensitivityResult` structs
- Numbered indexing allows reliable mapping back to source messages
- The prompt explicitly distinguishes individual-specific content from general policy (FR-015)
- "When uncertain, classify as SENSITIVE" implements FR-014

**Rejected alternative**: Individual message classification
- Too slow for typical batch sizes (5-20 messages)
- Would require parallelizing Ollama calls or accepting >5s overhead

**Rejected alternative**: Free-text response with parsing
- Fragile parsing, harder to validate, more likely to produce malformed output

### Response Parsing Strategy

1. Extract JSON from response (handle potential markdown code fences)
2. Unmarshal into `[]SensitivityResult`
3. Validate: correct count, valid categories, index range
4. On parse failure: retry once with same prompt (FR-009)
5. On second failure: treat all messages in batch as sensitive

---

## Decision 3: Error Handling Strategy

### Context

The spec mandates a hard-gate posture (FR-007): exports must fail when Ollama is unavailable. But there are multiple failure modes with different appropriate responses.

### Decision: Three-tier error handling

| Failure Mode | Response | Rationale |
|-------------|----------|-----------|
| **Ollama unreachable** (connection refused, timeout on Ping) | Fail export immediately with actionable error | FR-007: hard gate. User must start Ollama or use `--no-sensitivity-filter` |
| **Model not available** (404 on generate, not in tags list) | Fail export immediately with model-specific error | FR-007: variant. Suggest `ollama pull <model>` |
| **Malformed response** (invalid JSON, wrong count, missing fields) | Retry once. On second failure, treat batch as all-sensitive | FR-009: err on side of caution. Don't block export for transient model quirks |
| **Ollama becomes unreachable mid-export** | Fail on next classification attempt. Report which conversations succeeded | US2-AS3: partial failure reporting |
| **Classification timeout** (>60s per batch) | Fail with timeout error | Edge case from spec: consistent with hard-gate posture |

### Pre-export Validation

Before starting the export loop, call `client.Ping(ctx)` to verify:
1. Ollama is reachable (HTTP GET to root endpoint)
2. Configured model is available (check `/api/tags` response)

This catches the most common failure modes before any work is done (SC-002: fail within 5 seconds).

### Error Message Format

```
Error: Ollama is not reachable at http://localhost:11434

Sensitivity filtering is enabled but the Ollama server is not running.

To fix:
  • Start Ollama:  ollama serve
  • Or bypass for this run:  get-out export --no-sensitivity-filter
```

```
Error: Model "granite-guardian:8b" is not available

The sensitivity filter requires this model but it is not installed.

To fix:
  • Pull the model:  ollama pull granite-guardian:8b
  • Or bypass for this run:  get-out export --no-sensitivity-filter
```

---

## Decision 4: Integration Point in the Export Pipeline

### Context

The sensitivity filter must intercept messages between fetch and markdown write, but only for the local markdown export path. Google Docs export must remain unaffected (FR-006).

### Decision: Filter between message grouping and RenderDailyDoc

**Integration point in `ExportConversation` (exporter.go)**:

```
Current flow:
  1. Fetch all messages
  2. Filter to main messages
  3. Group by date
  4. For each date:
     a. Create/find daily doc (Google Docs)
     b. Write messages to doc (Google Docs) ← ALL messages
     c. Write local markdown (if configured) ← ALL messages

New flow:
  1. Fetch all messages
  2. Filter to main messages
  3. Group by date
  4. For each date:
     a. Create/find daily doc (Google Docs)
     b. Write messages to doc (Google Docs) ← ALL messages (unchanged)
     c. If sensitivity filter configured AND localExport:
        i.  Run batch classification on date's messages
        ii. Filter to non-sensitive messages
        iii. If no messages remain, skip markdown file (FR-010)
        iv. Write filtered messages to markdown with sensitivity frontmatter
     d. Else: Write local markdown as before (unfiltered)
```

**Rationale**:
- Minimal change to existing flow — only the markdown write path is affected
- Google Docs export is completely untouched (FR-006)
- Classification happens per-date batch, matching the natural grouping
- The `MessageFilter` interface is injected into the `Exporter` struct, not hardcoded

**Rejected alternative**: Filter at the message fetch level
- Would affect both Google Docs and markdown exports
- Violates FR-006

**Rejected alternative**: Filter in `MarkdownWriter.RenderDailyDoc`
- Mixes classification (I/O to Ollama) with rendering (pure transformation)
- Harder to test, violates Single Responsibility

### MessageFilter Interface

```go
// pkg/exporter/sensitivity.go

// MessageFilter classifies messages and returns filtering results.
// Implementations may call external services (Ollama) or be no-ops (for testing).
type MessageFilter interface {
    // FilterMessages classifies the given messages and returns which ones
    // passed filtering, along with metadata about what was filtered.
    FilterMessages(ctx context.Context, messages []slackapi.Message) (*FilterResult, error)
}
```

This interface allows:
- `OllamaFilter` — real implementation calling Ollama
- `NoOpFilter` — passes all messages through (used when filtering disabled)
- Test doubles — for unit testing the export pipeline without Ollama

### Exporter Wiring

The `Exporter` struct gets a new `messageFilter MessageFilter` field, set during initialization:
- If `settings.Ollama.Enabled && !noSensitivityFilter` → create `OllamaFilter`
- If `noSensitivityFilter` flag set → `nil` (no filter, same as current behavior)
- If `settings.Ollama` is nil → `nil` (feature not configured)

---

## Decision 5: CLI Flag Design

### `--no-sensitivity-filter`

- **Type**: Boolean flag (default: false)
- **Behavior**: When set, disables sensitivity filtering for this run only, regardless of settings
- **No Ollama calls**: When this flag is set, the Ollama client is never created (US3-AS1)
- **Persistent settings unchanged**: The flag does not modify `settings.json` (US3-AS2)

### `--ollama-endpoint`

- **Type**: String flag (default: empty — uses settings or "http://localhost:11434")
- **Behavior**: Overrides the Ollama endpoint for this run
- **Use case**: Testing with a remote Ollama instance or non-default port
- **Priority**: CLI flag > settings.json > default

### Resolution Order for Ollama Endpoint

1. `--ollama-endpoint` flag value (if non-empty)
2. `settings.Ollama.Endpoint` (if configured)
3. Default: `http://localhost:11434`

---

## Decision 6: YAML Frontmatter Enrichment

### Current Frontmatter

```yaml
---
conversation: John Smith
type: dm
date: "2026-04-21"
participants:
  - John Smith
  - Jane Doe
---
```

### Enhanced Frontmatter (when filtering active)

```yaml
---
conversation: John Smith
type: dm
date: "2026-04-21"
participants:
  - John Smith
  - Jane Doe
sensitivity:
  filtered_count: 3
  categories:
    hr: 2
    financial: 1
---
```

### Design Notes

- `sensitivity:` block only appears when filtering is active (FR-005)
- `filtered_count: 0` is included when filtering ran but found nothing sensitive (provides audit trail)
- `categories:` only lists categories with count > 0
- When `filtered_count` equals total message count, no file is written (FR-010) — so this frontmatter is never seen with `filtered_count` equal to total

---

## Decision 7: Doctor Health Checks

### Conditional Display

Doctor checks for Ollama are only shown when sensitivity filtering is configured (`settings.Ollama != nil && settings.Ollama.Enabled`). When not configured, no Ollama checks appear (US4-AS3: "no noise for unconfigured features").

### New Checks

| Check | Condition | Pass | Fail |
|-------|-----------|------|------|
| Ollama reachable | `GET /` returns 200 | "Ollama: OK (http://localhost:11434)" | "Ollama: FAIL — not reachable at http://localhost:11434" + hint |
| Sensitivity model | Model in `/api/tags` response | "Sensitivity model: OK (granite-guardian:8b)" | "Sensitivity model: FAIL — model not found" + hint |

### Integration

New checks are added after the existing check 11 (background sync service) in `runDoctor`. They follow the same `pass_/warn_/fail_` counter pattern used by all other checks.
