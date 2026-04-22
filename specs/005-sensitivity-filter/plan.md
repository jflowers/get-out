# Implementation Plan: Sensitivity Filter for Local Markdown Export

**Branch**: `005-sensitivity-filter` | **Date**: 2026-04-21 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/005-sensitivity-filter/spec.md`

## Summary

Add an Ollama-based sensitivity classifier that filters per-message content from local markdown exports. Messages classified as sensitive (HR, legal, financial, health, termination) are excluded from markdown files written for Dewey indexing, while Google Docs exports remain unaffected. The system uses a hard-gate posture: exports fail when Ollama is unavailable rather than silently exporting unfiltered content. Classification operates in batch mode (one Ollama call per daily doc) using the Granite Guardian model with a purpose-built prompt for Slack message classification.

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: Ollama REST API (HTTP client, `net/http`), existing `pkg/exporter/`, `pkg/config/`, `internal/cli/`, cobra v1.10.2  
**Storage**: YAML frontmatter enrichment in markdown files; `OllamaConfig` struct added to `settings.json` via `pkg/config/types.go`  
**Testing**: `go test -race -count=1 ./...`  
**Target Platform**: macOS (primary), Linux  
**Project Type**: Single Go binary (CLI)  
**Performance Goals**: ≤5 seconds overhead per daily doc for sensitivity classification (SC-003)  
**Constraints**: All classification local-only via Ollama (SC-006); no cloud API calls for classification; hard-gate on Ollama availability (FR-007)  
**Scale/Scope**: Typical sync: 5-20 new messages per conversation per daily doc; batch fits within Granite Guardian context window

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Session-Driven Extraction | ✅ Pass | Sensitivity filter does not affect browser session extraction. Slack data is still fetched via CDP/xoxc. |
| II. Go-First Architecture | ✅ Pass | New `pkg/ollama/` package implemented in Go. HTTP client to Ollama REST API. No external language dependencies. |
| III. Stealth & Reliability | ✅ Pass | No change to browser automation or Slack API interaction. Ollama runs locally. |
| IV. Two-Tier Extraction Strategy | ✅ Pass | No change to extraction tiers. Sensitivity filter operates post-extraction, before markdown write. |
| V. Concurrency & Resilience | ✅ Pass | Classification is per-daily-doc, fits naturally into existing sequential daily doc loop. Checkpoint system unaffected. |
| VI. Security First | ✅ Pass | No credentials stored for Ollama (local HTTP). Message content stays local (FR-002, SC-006). No cloud classification. |
| VII. Output Format | ✅ Pass | Google Docs export unaffected (FR-006). Markdown output enhanced with sensitivity frontmatter (FR-005). |
| VIII. Google Drive Integration | ✅ Pass | No change to Google Drive integration. |
| IX. Documentation Maintenance | ✅ Pass | README.md, AGENTS.md updates required for new flags and config. Tracked in tasks. |

**No violations. Proceeding to Phase 0.**

## Project Structure

### Documentation (this feature)

```text
specs/005-sensitivity-filter/
├── plan.md              # This file
├── research.md          # Phase 0 output — design decisions
├── data-model.md        # Phase 1 output — entity definitions
├── quickstart.md        # Phase 1 output — setup/verification steps
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
get-out/
├── cmd/get-out/main.go           # No changes needed
├── internal/cli/
│   ├── export.go                 # Add --no-sensitivity-filter, --ollama-endpoint flags
│   └── selfservice.go            # Add Ollama + model doctor checks (conditional)
├── pkg/
│   ├── config/
│   │   └── types.go              # Add OllamaConfig struct to Settings
│   ├── ollama/                   # NEW PACKAGE
│   │   ├── client.go             # Ollama REST API HTTP client
│   │   └── guardian.go           # Granite Guardian prompt + batch classification
│   └── exporter/
│       ├── sensitivity.go        # NEW: MessageFilter interface + OllamaFilter impl
│       ├── mdwriter.go           # Modify RenderDailyDoc to accept FilterResult for frontmatter
│       └── exporter.go           # Wire sensitivity filter into markdown export path
```

**Structure Decision**: Follows existing Go package layout. New `pkg/ollama/` package encapsulates all Ollama interaction (client + prompt). `pkg/exporter/sensitivity.go` provides the `MessageFilter` interface that bridges the Ollama classifier into the export pipeline. This keeps the Ollama dependency isolated and the exporter testable via interface injection.

## Phase 0: Research

**Status**: Complete  
**Output**: [research.md](./research.md)

Key decisions:
1. **Ollama client**: New minimal HTTP client in `pkg/ollama/client.go` (not ported from gcal-organizer — different project, different dependencies)
2. **Batch classification**: Single Ollama call per daily doc with structured JSON response
3. **Error handling**: Hard-gate with retry-once on malformed response; message treated as sensitive on second failure
4. **Integration point**: Between message fetch and `RenderDailyDoc` call in `ExportConversation`

## Phase 1: Design & Contracts

**Status**: Complete  
**Output**: [data-model.md](./data-model.md), [quickstart.md](./quickstart.md)

Key design decisions:
1. **MessageFilter interface**: `FilterMessages(ctx, []Message) (*FilterResult, error)` — injectable, testable
2. **OllamaConfig in Settings**: Extends existing `Settings` struct with `Ollama *OllamaConfig` field
3. **Frontmatter enrichment**: `RenderDailyDoc` accepts optional `*FilterResult` to add `sensitivity:` YAML block
4. **Doctor checks**: Conditional on `settings.Ollama != nil && settings.Ollama.Enabled`

## Phase 2: Tasks

**Status**: Complete  
**Output**: [tasks.md](./tasks.md)

## Testing Strategy

### Coverage Floor

- **≥80%** line coverage for `pkg/ollama/` and new/modified code in `pkg/exporter/` (`sensitivity.go`, `mdwriter.go` changes)
- Measurement command: `go test -race -count=1 -coverprofile=coverage.out ./pkg/ollama/... ./pkg/exporter/...`

### Test Tiers

| Tier | Scope | Technique |
|------|-------|-----------|
| Unit | `pkg/ollama/client.go`, `guardian.go`, `pkg/exporter/sensitivity.go`, `mdwriter.go` | `net/http/httptest` mock servers; no real Ollama |
| Integration | Full export pipeline with sensitivity filter wired in | Mock `httptest.Server` as Ollama backend through `ExportConversation` |

### Exclusions

- Real Ollama integration tests are excluded from CI. They are manual benchmarking exercises for prompt tuning and model evaluation.

### Security Note

`SensitivityResult.Reasoning` MUST NOT be written to disk or logged above debug level. The reasoning field may echo sensitive content from the classified message. It is used only for developer debugging at `debug` log level and must never appear in markdown output, frontmatter, or standard log output.

## Complexity Tracking

> No constitution violations to justify. All new code follows existing patterns.
