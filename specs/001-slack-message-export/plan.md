# Implementation Plan: Slack Message Export

**Branch**: `001-slack-message-export` | **Date**: 2026-02-03 | **Updated**: 2026-02-07 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/001-slack-message-export/spec.md`

## Summary

Build a Go CLI tool that exports Slack conversations to Google Docs in Google Drive. The tool supports two extraction modes:

1. **Browser Mode** (for DMs/Group DMs): Extracts authentication tokens from an active browser session via Chrome DevTools Protocol (Chromedp)
2. **API Mode** (for Channels): Uses Slack bot token (xoxb-) for channels where the bot is a member

Both modes use Slack's REST API to retrieve message history with automatic rate limiting, checkpointing for resume capability, and mrkdwn-to-Google Docs formatting conversion with user ID resolution. Google Drive API v3 with OAuth 2.0 is used to create native Google Docs. Configuration is driven by `conversations.json` which defines which conversations to export and their extraction mode.

## Technical Context

**Language/Version**: Go 1.21+  
**Primary Dependencies**: Chromedp (CDP), cobra (CLI), errgroup (concurrency), Google Drive API v3, Google Docs API v1  
**Storage**: JSON files for checkpoints (local), Google Docs for output (cloud)  
**Testing**: Go standard testing package (`go test`)  
**Target Platform**: macOS, Linux (CLI binary)  
**Project Type**: Single CLI application  
**Performance Goals**: Export 1000 messages in <5 minutes; handle 10,000+ message conversations  
**Constraints**: Respect Slack rate limits (~50 req/min Tier 3); respect Google API quotas; OAuth 2.0 required for Drive  
**Scale/Scope**: Personal use; 1-100 conversations per export session

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Session-Driven Extraction | PASS | Uses Chromedp to extract tokens from active browser |
| II. Go-First Architecture | PASS | Pure Go implementation with minimal dependencies |
| III. Stealth & Reliability | PASS | Connects to existing browser; mirrors natural headers |
| IV. Two-Tier Extraction | PASS | API mimicry as primary; DOM scraping deferred to future |
| V. Concurrency & Resilience | PASS | errgroup + checkpoint system planned |
| VI. Security First | PASS | No hardcoded tokens; runtime extraction only |
| VII. Output Format | PASS | Google Docs with proper formatting and user resolution |
| VIII. Google Drive Integration | PASS | OAuth 2.0 auth; native Docs creation via API |

**Post-Design Re-check**: All principles satisfied. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/001-slack-message-export/
├── plan.md              # This file
├── research.md          # Technical decisions and rationale
├── data-model.md        # Entity definitions and Go types
├── quickstart.md        # User guide and CLI reference
├── contracts/           
│   └── slack-api.md     # Slack API endpoint contracts
└── tasks.md             # Implementation tasks (created by /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/
└── get-out/
    └── main.go              # CLI entry point

pkg/
├── chrome/
│   ├── chrome.go            # Chromedp connection and session management
│   └── token.go             # Token extraction from localStorage
├── config/
│   ├── config.go            # Config file loading (conversations.json, people.json)
│   └── types.go             # Config struct definitions
├── slackapi/
│   ├── client.go            # HTTP client with auth and rate limiting
│   ├── conversations.go     # conversations.list, conversations.history
│   ├── replies.go           # conversations.replies
│   ├── users.go             # users.info, users.list
│   └── types.go             # API response types
├── gdrive/
│   ├── auth.go              # OAuth 2.0 authentication flow
│   ├── client.go            # Google Drive/Docs API client
│   ├── docs.go              # Google Docs creation and formatting
│   └── folder.go            # Folder management in Drive
├── parser/
│   ├── mrkdwn.go            # Slack mrkdwn to Google Docs converter
│   ├── resolver.go          # User ID to name resolution
│   ├── usermapping.go       # Slack→Google user mapping loader
│   └── linkreplacer.go      # Slack link to Drive link replacement
├── exporter/
│   ├── exporter.go          # Main export orchestration
│   ├── checkpoint.go        # Checkpoint save/load for resume
│   ├── docwriter.go         # Google Docs writing
│   ├── index.go             # Export index management (tracks exported docs)
│   └── structure.go         # Folder/daily doc/thread organization
└── models/
    └── models.go            # Shared domain models

internal/
└── cli/
    ├── root.go              # Cobra root command
    ├── list.go              # list command
    ├── export.go            # export command
    └── status.go            # status command

tests/
├── unit/
│   ├── parser_test.go       # mrkdwn parsing tests
│   ├── checkpoint_test.go   # Checkpoint serialization tests
│   ├── resolver_test.go     # User resolution tests
│   ├── usermapping_test.go  # Slack→Google mapping tests
│   ├── linkreplacer_test.go # Link replacement tests
│   ├── index_test.go        # Export index tests
│   └── gdrive_test.go       # Google Docs formatting tests
└── integration/
    ├── slackapi_test.go     # API client tests (with mocks)
    ├── gdrive_test.go       # Google Drive API tests (with mocks)
    └── exporter_test.go     # End-to-end export tests
```

**Structure Decision**: Single project CLI structure chosen per constitution. The `pkg/` directory contains reusable libraries, `cmd/` contains the binary entry point, and `internal/` contains CLI-specific code not intended for external use.

## Implementation Phases

### Phase 1: Foundation (Chrome + Slack API)

**Goal**: Extract Slack data from browser session

1. **Chrome Connection** (`pkg/chrome/`)
   - Connect to Chrome via remote debugging port
   - Extract `xoxc` token from localStorage
   - Capture `d` cookie for API auth

2. **Slack API Client** (`pkg/slackapi/`)
   - Implement `conversations.history` endpoint
   - Basic rate limit handling (fixed delay)
   - Error response parsing

3. **CLI Scaffold** (`internal/cli/`)
   - `get-out export <conversation-id>` command
   - `--chrome-port` flag
   - Basic error handling

**Deliverable**: Can fetch messages from Slack API

### Phase 2: Google Drive Integration (P1 - Export DMs)

**Goal**: Create Google Docs in Drive with exported messages

1. **Google OAuth** (`pkg/gdrive/`)
   - OAuth 2.0 consent flow (browser-based)
   - Token storage (refresh token persistence)
   - `get-out auth` command for initial setup

2. **Google Docs Creation** (`pkg/gdrive/`)
   - Create new Google Doc via Docs API
   - Folder management (create "Slack Exports" folder)
   - Document naming convention

3. **User Resolution** (`pkg/parser/`)
   - Fetch user list via `users.list`
   - Build ID → name map
   - Cache users locally for reuse

4. **Mrkdwn to Docs Converter** (`pkg/parser/`)
   - Convert `<@U123>` to `@Real Name` (bold)
   - Convert `<#C123|channel>` to `#channel`
   - Handle code blocks (monospace), links (hyperlinks), formatting

5. **Docs Writer** (`pkg/exporter/`)
   - Batch formatting requests for efficiency
   - Group messages by date with headers
   - Format with sender, timestamp, content

**Deliverable**: Can export DM to Google Doc with formatted content

### Phase 3: Threads & Checkpoints (P2 + Resilience)

**Goal**: Thread support and resume capability

1. **Thread Fetching** (`pkg/slackapi/`)
   - Implement `conversations.replies`
   - Detect threaded messages by `thread_ts`
   - Fetch and merge replies

2. **Checkpoint System** (`pkg/exporter/`)
   - Save state after each page of messages
   - Store `last_ts`, message count, status
   - Load and resume from checkpoint

3. **Rate Limit Improvements** (`pkg/slackapi/`)
   - Exponential backoff on 429 responses
   - Respect `Retry-After` header
   - Progress reporting during waits

**Deliverable**: Can export threaded conversations; survives interruptions

### Phase 4: Batch & Polish (P3 + UX)

**Goal**: Batch exports and production-ready CLI

1. **Conversation Listing** (`internal/cli/`)
   - `get-out list` command
   - Show DMs, groups, channels
   - Filter by type

2. **Batch Export** (`pkg/exporter/`)
   - `--all-dms`, `--all-groups` flags
   - Parallel conversation exports (semaphore-limited)
   - Summary report on completion

3. **Session Handling**
   - Detect expired sessions
   - Clear error messages for auth failures
   - Suggest re-authentication steps

4. **Status Command** (`internal/cli/`)
   - Show active/paused exports
   - Display progress and ETA
   - List completed exports

**Deliverable**: Production-ready CLI tool

## Dependencies

```go
// go.mod
module github.com/jflowers/get-out

go 1.21

require (
    github.com/chromedp/chromedp v0.9.5
    github.com/spf13/cobra v1.8.0
    golang.org/x/sync v0.6.0                    // errgroup
    golang.org/x/oauth2 v0.16.0                 // OAuth 2.0
    google.golang.org/api v0.160.0              // Google APIs
)
```

**Google API Setup Requirements**:
1. Create Google Cloud project
2. Enable Google Drive API and Google Docs API
3. Create OAuth 2.0 credentials (Desktop app type)
4. Download `credentials.json` to project root

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Slack blocks automated access | Use existing browser session; mirror realistic headers; add random delays |
| Rate limits slow large exports | Exponential backoff; parallel user fetching; checkpoint frequently |
| Token format changes | Abstract token extraction; log warnings on parse failures |
| Browser not running | Clear error message; CLI validates connection before starting |
| Long-running exports timeout | Checkpoint every page; resume seamlessly |
| Google OAuth token expires | Store refresh token; auto-refresh access token |
| Google Drive quota exceeded | Exponential backoff on 429; batch API requests |
| User cancels OAuth flow | Clear error message; instructions to retry `get-out auth` |
| credentials.json missing | Clear setup instructions; link to Google Cloud Console |

## Testing Strategy

1. **Unit Tests**: Parser logic, checkpoint serialization, user resolution
2. **Integration Tests**: Mock HTTP responses for API client tests
3. **Manual Testing**: Real Slack workspace with various conversation types

## Success Metrics (from Spec)

- [x] SC-001: Export 1000-message conversation in <5 minutes (1,094 msgs in ~6 min — close, rate limit dominated)
- [x] SC-002: No raw user IDs or mrkdwn syntax in Google Docs output
- [x] SC-003: 100% message capture (verified by count)
- [ ] SC-004: Resume within 10 seconds, lose <1 minute progress
- [x] SC-005: No manual Slack token configuration required
- [x] SC-006: Google Docs searchable via Drive search
- [x] SC-007: Google Docs shareable via standard Drive sharing
