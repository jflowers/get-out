# get-out Constitution

## Core Principles

### I. Session-Driven Extraction

All data extraction MUST leverage an active browser session (via Zen/OpenCode environment) to bypass traditional MFA hurdles. The tool SHALL NOT require users to configure OAuth tokens or API keys manually when a valid browser session exists.

### II. Go-First Architecture

- Language: Go (Golang) — chosen for superior concurrency (goroutines) and single static binary output
- All core functionality MUST be implemented in Go
- External dependencies SHOULD be minimized to maintain single-binary deployment

### III. Stealth & Reliability

- Browser automation MUST use Chrome DevTools Protocol (CDP) via Chromedp or Rod
- Implementation MUST mirror natural user headers to avoid detection
- Slack monitors for "Headless" flags; use headed profile state from Zen environment
- Rate limiting MUST be handled with exponential backoff for 429 responses

### IV. Two-Tier Extraction Strategy

**Tier 1: API Mimicry (Preferred)**
- Extract `xoxc` token from `localStorage.getItem('localConfig_v2')`
- Use authenticated POST requests to internal Slack endpoints:
  - `/api/conversations.history` for message streams
  - `/api/conversations.replies` for nested threads
  - `/api/users.info` to resolve IDs to real names

**Tier 2: DOM Scraping (Fallback)**
- If API access is restricted, use DOM selectors targeting virtual list
- Key selectors: `[role="message"]` or `[data-qa="message_container"]`
- Scroll logic: set `scrollTop = 0` on `.c-scrollbar__hider` container to lazy load history

### V. Concurrency & Resilience

- Use Go's `errgroup` for parallel fetching of user profiles and message history
- Implement checkpoint system: save last `ts` (timestamp) so export can resume if interrupted
- Handle rate limits (`ratelimited` error) gracefully without data loss

### VI. Security First

- NEVER hardcode `xoxc` tokens or session cookies in source code
- Tokens MUST be read from active browser session or environment variables at runtime
- No credentials SHALL be persisted to disk unless explicitly encrypted

### VII. Output Format

- All exported messages MUST be uploaded as Google Docs to Google Drive
- Slack's `mrkdwn` format MUST be parsed and converted to Google Docs formatting:
  - User mentions (`<@U12345>`) resolved to readable names using pre-fetched user map
  - Channel references, links, and formatting preserved in Google Docs equivalents
  - Code blocks formatted with monospace font
  - Bold, italic, and strikethrough preserved

### VIII. Google Drive Integration

- Google Drive API MUST be used to create and upload documents
- OAuth 2.0 authentication flow required for Drive access
- Credentials SHOULD be stored securely using OS keychain/credential manager when possible
- Exported docs MUST be organized in a user-configurable Drive folder
- Document naming convention: `{type} - {name} - {date}` (e.g., "DM - John Smith - 2026-02-03")
- Rate limiting for Google Drive API MUST be handled with exponential backoff

## Technical Constraints

### Technology Stack
- **Primary Language**: Go 1.21+
- **Browser Automation**: Chromedp or Rod (CDP-based)
- **Cloud Integration**: Google Drive API v3 for document creation
- **Environment**: OpenCode.ai with Zen subscription
- **Output Format**: Google Docs in Google Drive, organized by folder

### Project Structure
```
/get-out
├── main.go          # Entry point and CLI flags
├── pkg/
│   ├── chrome/      # Chromedp/Rod initialization logic
│   ├── slackapi/    # API request structures & mimicry
│   ├── gdrive/      # Google Drive API client & OAuth
│   ├── parser/      # Content conversion logic
│   └── util/        # Rate limiting and helpers
├── go.mod
└── README.md
```

## Development Workflow

1. **Test-Driven**: Write tests for API response parsing before implementation
2. **Incremental**: Build Tier 1 (API) first, Tier 2 (DOM) as fallback
3. **Checkpoint-First**: Implement resume capability early to handle long exports
4. **Security Review**: All credential handling reviewed before merge

## Governance

- Constitution supersedes all other implementation decisions
- Changes to extraction strategy require documentation and testing
- Security-related changes require explicit review

**Version**: 1.1.0 | **Ratified**: 2026-02-03 | **Last Amended**: 2026-02-03
