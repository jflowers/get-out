# Research: Slack Message Export

**Feature**: 001-slack-message-export  
**Date**: 2026-02-03

## Browser Automation Framework

### Decision: Chromedp

**Rationale**: Chromedp provides direct Chrome DevTools Protocol (CDP) access in pure Go, enabling:
- Native Go integration without external dependencies (no Selenium, no WebDriver)
- Direct localStorage access via JavaScript evaluation
- Low-level control for stealth operations (avoiding headless detection)
- Ability to connect to existing browser sessions

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| Rod | Similar capabilities but smaller community; Chromedp has more examples for auth extraction |
| Selenium | Requires external WebDriver binary; heavier footprint; more detectable |
| Puppeteer (via Go bindings) | Adds Node.js dependency; not native Go |
| Playwright | No official Go SDK; would require subprocess calls |

### Key Chromedp Patterns for This Project

```go
// Connect to existing Chrome instance (user's Zen browser)
allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), "ws://localhost:9222")
defer cancel()
ctx, cancel := chromedp.NewContext(allocCtx)

// Extract localStorage token
var token string
chromedp.Run(ctx,
    chromedp.Evaluate(`JSON.parse(localStorage.getItem('localConfig_v2')).teams[0].token`, &token),
)
```

## Token Extraction Strategy

### Decision: Extract `xoxc` token from localStorage

**Rationale**: Slack stores session tokens in `localStorage.localConfig_v2`. This approach:
- Leverages the user's existing authenticated session
- Avoids MFA re-authentication
- Works with any Slack workspace the user has access to

**Token Location**:
```javascript
// Primary location
localStorage.getItem('localConfig_v2')
// Structure: { teams: [{ token: "xoxc-...", ... }] }
```

**Cookie Requirement**: The `d` cookie (`xoxd-...`) must also be captured and sent with API requests for full authentication.

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| OAuth app token | Requires app installation; limited scopes; can't access DMs without user tokens |
| Manual token entry | Poor UX; security risk of copying tokens |
| Slack Export (official) | Only available to workspace admins; doesn't include DMs |

## API Mimicry Approach

### Decision: Use Slack's internal REST endpoints with captured auth

**Rationale**: Slack's web client uses documented REST endpoints that accept user tokens:
- `conversations.history` - Paginated message retrieval
- `conversations.replies` - Thread reply retrieval  
- `users.info` - User ID to name resolution

**Request Pattern**:
```
POST https://slack.com/api/conversations.history
Content-Type: application/x-www-form-urlencoded
Authorization: Bearer xoxc-...
Cookie: d=xoxd-...

channel=C123ABC456&limit=100&cursor=...
```

**Rate Limiting**: Slack enforces Tier 3 rate limits (~50 requests/minute). Must implement exponential backoff when receiving `429` or `ratelimited` error responses.

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| DOM scraping only | Slower; fragile to UI changes; harder to get complete data |
| GraphQL endpoints | Not publicly documented; may change without notice |
| Undocumented batch endpoints | High risk of breaking; no error documentation |

## Checkpoint & Resume Strategy

### Decision: Timestamp-based checkpointing with JSON state file

**Rationale**: 
- Slack messages have unique `ts` (timestamp) identifiers
- Saving the last processed `ts` allows precise resume points
- JSON state file is human-readable and easily debuggable

**State File Structure**:
```json
{
  "conversation_id": "D123ABC456",
  "last_ts": "1704067200.000100",
  "messages_exported": 547,
  "started_at": "2026-02-03T10:00:00Z",
  "status": "in_progress"
}
```

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| SQLite state DB | Overkill for simple state; adds dependency |
| Message ID tracking | Slack uses `ts` as primary identifier; IDs would be redundant |
| No checkpointing | Unacceptable for large exports; data loss on interruption |

## Content Conversion (mrkdwn → Google Docs)

### Decision: Custom mrkdwn parser outputting Google Docs API requests

**Rationale**: Slack's mrkdwn needs conversion to Google Docs formatting:
- User mentions: `<@U12345>` → `@John Smith` (bold text)
- Channel links: `<#C12345|general>` → `#general` (link or text)
- URLs: `<https://example.com|Example>` → Hyperlink in Google Docs
- Bold: `*text*` → Bold formatting
- Italic: `_text_` → Italic formatting
- Code: `` `code` `` → Monospace font (Courier New)
- Code blocks: → Indented paragraph with monospace font

**Conversion Pipeline**:
1. Pre-fetch all users in workspace → build ID-to-name map
2. Process each message:
   - Replace `<@U...>` with resolved names
   - Convert channel references
   - Build Google Docs API request with proper formatting
   - Handle emoji shortcodes (`:emoji:` → Unicode emoji)

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| Markdown file upload | Google Drive doesn't render Markdown; would need conversion anyway |
| HTML upload | More complex; Google Docs import can lose formatting |
| Plain text | Loses all formatting; poor readability |

## Concurrency Model

### Decision: errgroup with semaphore for parallel fetching

**Rationale**: 
- `errgroup` provides clean error propagation from goroutines
- Semaphore (channel-based) limits concurrent API calls to respect rate limits
- Parallel user info fetching significantly speeds up large exports

**Pattern**:
```go
g, ctx := errgroup.WithContext(ctx)
sem := make(chan struct{}, 5) // Max 5 concurrent requests

for _, userID := range userIDs {
    userID := userID
    g.Go(func() error {
        sem <- struct{}{}
        defer func() { <-sem }()
        return fetchUserInfo(ctx, userID)
    })
}
return g.Wait()
```

## Stealth Considerations

### Decision: Use existing browser profile, mirror realistic headers

**Rationale**: Slack may detect and block automated access. Mitigations:
- Connect to user's existing Zen browser (already authenticated, has cookies)
- Use realistic User-Agent and headers from the browser session
- Add random delays between requests (100-500ms)
- Avoid headless mode flags

**Headers to Mirror**:
```
User-Agent: [from browser]
Accept: application/json
Accept-Language: en-US,en;q=0.9
Sec-Fetch-Dest: empty
Sec-Fetch-Mode: cors
Sec-Fetch-Site: same-origin
```

## Google Drive Integration

### Decision: Google Drive API v3 with OAuth 2.0

**Rationale**: 
- Official Google API with comprehensive Go SDK
- OAuth 2.0 provides secure, revocable access
- Can create native Google Docs (not just uploaded files)
- Supports folder organization and document metadata

**Go Library**: `google.golang.org/api/drive/v3` and `google.golang.org/api/docs/v1`

**Authentication Flow**:
1. First run: Open browser for OAuth consent
2. User grants Drive file access scope
3. Store refresh token locally (encrypted or in OS keychain)
4. Subsequent runs: Use refresh token for access

**Required Scopes**:
- `https://www.googleapis.com/auth/drive.file` - Create/modify files created by this app
- `https://www.googleapis.com/auth/docs` - Create and edit Google Docs

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| Service account | Requires Google Workspace admin setup; overkill for personal use |
| API key only | Can't create files; read-only access |
| Upload as .docx | Loses Google Docs native features; requires conversion step |

### Document Creation Strategy

**Decision**: Use Google Docs API to create formatted documents directly

**Rationale**:
- Native Google Docs are editable, searchable, and shareable
- Docs API allows precise formatting control (bold, italic, fonts, links)
- Better than uploading converted files

**API Pattern**:
```go
// Create empty doc
doc, err := docsService.Documents.Create(&docs.Document{
    Title: "DM - John Smith - 2026-02-03",
}).Do()

// Insert formatted content
requests := []*docs.Request{
    {InsertText: &docs.InsertTextRequest{
        Location: &docs.Location{Index: 1},
        Text:     "Message content here",
    }},
    {UpdateTextStyle: &docs.UpdateTextStyleRequest{
        Range: &docs.Range{StartIndex: 1, EndIndex: 10},
        TextStyle: &docs.TextStyle{Bold: true},
        Fields: "bold",
    }},
}
docsService.Documents.BatchUpdate(doc.DocumentId, &docs.BatchUpdateDocumentRequest{
    Requests: requests,
}).Do()
```

## Output Organization

### Decision: Hierarchical folder structure with daily docs and thread subfolders

**Structure**:
```
Google Drive/
└── Slack Exports/                           # Configurable root folder
    ├── DM - John Smith/                     # Folder per DM conversation
    │   ├── 2026-02-01.gdoc                  # Daily message doc
    │   ├── 2026-02-02.gdoc                  # Daily message doc
    │   ├── 2026-02-03.gdoc                  # Daily message doc
    │   └── Threads/                         # Subfolder for threads
    │       ├── 2026-02-01 - Project update/     # Thread folder
    │       │   ├── 2026-02-01.gdoc              # Thread replies for day 1
    │       │   └── 2026-02-02.gdoc              # Thread replies for day 2
    │       └── 2026-02-02 - Quick question/     # Short thread (single day)
    │           └── 2026-02-02.gdoc
    ├── Group - project-alpha/               # Folder per group/channel
    │   ├── 2026-01-15.gdoc
    │   ├── 2026-01-16.gdoc
    │   └── Threads/
    │       ├── 2026-01-15 - Sprint planning/    # Long thread spanning 3 days
    │       │   ├── 2026-01-15.gdoc
    │       │   ├── 2026-01-16.gdoc
    │       │   └── 2026-01-17.gdoc
    │       ├── 2026-01-15 - Bug discussion/     # Single-day thread
    │       │   └── 2026-01-15.gdoc
    │       └── 2026-01-16 - Release notes/
    │           └── 2026-01-16.gdoc
    └── _metadata/                           # Subfolder for state
        ├── users.json                       # Cached Slack user map
        ├── user-mapping.json                # Slack → Google account mapping
        ├── export-index.json                # Index of all exported docs with IDs
        └── checkpoints/                     # Resume state files
```

**Naming Conventions**:
- **Conversation folders**: `{type} - {name}` (e.g., "DM - John Smith", "Group - project-alpha")
- **Daily docs**: `{YYYY-MM-DD}.gdoc` (e.g., "2026-02-03.gdoc")
- **Thread folders**: `{YYYY-MM-DD} - {topic-preview}/` (e.g., "2026-02-01 - Project update/")
  - Topic preview: first 30 chars of thread parent message, sanitized
- **Thread daily docs**: `{YYYY-MM-DD}.gdoc` inside thread folder

**Rationale for Daily Chunking (both main and threads)**:
- Daily chunking prevents massive single docs that are slow to load
- Consistent structure: threads follow same pattern as main conversation
- Long-running threads (days/weeks) stay manageable
- Thread subfolder keeps conversations organized without cluttering main folder
- Enables cross-linking between daily docs and thread folders
- Makes it easy to find conversations by date or topic

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| Single doc per conversation | Massive docs for active conversations; slow to load/edit |
| Flat thread structure | Threads mixed with daily docs; hard to distinguish |
| Weekly/monthly chunking | Too coarse; daily provides better granularity |

## Slack to Google User Mapping

### Decision: External JSON mapping file with optional Google account links

**Rationale**: 
- Users may want to link Slack mentions to Google accounts for @-mentions
- Not all Slack users will have Google accounts
- Mapping should be optional and user-provided

**User Mapping File** (`user-mapping.json`):
```json
{
  "U123ABC456": {
    "google_email": "john.smith@company.com",
    "google_name": "John Smith"
  },
  "U789DEF012": {
    "google_email": "jane.doe@gmail.com",
    "google_name": "Jane Doe"
  }
}
```

**Behavior**:
1. If mapping exists for user → Create Google Docs mention/link to that user
2. If no mapping → Use Slack display name (no link)
3. If user not found at all → Show `@Unknown (U123ABC456)`

**Google Docs Mention Format**:
- With Google account: `@John Smith` as a link to `mailto:john.smith@company.com` or Google profile
- Without: `@John Smith` (bold text, no link)

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| Automatic email matching | Privacy concerns; Slack emails may differ from Google |
| Google Workspace directory lookup | Requires admin access; not available for personal accounts |
| No Google linking | Loses opportunity to make mentions actionable |

## Slack Link Replacement

### Decision: Replace Slack links with Google Drive links using export index

**Rationale**:
- Slack message/thread/channel links become useless outside Slack
- Replacing with Google Drive links maintains navigability
- Export index tracks which conversations have been exported

**Export Index** (`export-index.json`):
```json
{
  "conversations": {
    "D123ABC456": {
      "type": "dm",
      "name": "John Smith",
      "folder_id": "1abc...",
      "folder_url": "https://drive.google.com/drive/folders/1abc..."
    },
    "C789DEF012": {
      "type": "channel",
      "name": "project-alpha",
      "folder_id": "2def...",
      "folder_url": "https://drive.google.com/drive/folders/2def..."
    }
  },
  "threads": {
    "C789DEF012:1704067200.000100": {
      "doc_id": "3ghi...",
      "doc_url": "https://docs.google.com/document/d/3ghi...",
      "title": "2026-01-01 - Sprint planning"
    }
  },
  "daily_docs": {
    "D123ABC456:2026-02-03": {
      "doc_id": "4jkl...",
      "doc_url": "https://docs.google.com/document/d/4jkl..."
    }
  }
}
```

**Link Replacement Rules**:
| Slack Link Type | Replacement |
|-----------------|-------------|
| `slack.com/archives/C123/p1234567890` (message) | Link to daily doc at that date |
| `slack.com/archives/C123/p1234567890?thread_ts=...` (thread) | Link to thread doc in Threads subfolder |
| `slack.com/archives/C123` (channel) | Link to conversation folder |
| Link to unexported conversation | Keep original Slack link + `[not exported]` note |

**Implementation**:
1. Parse Slack links in messages using regex
2. Extract conversation ID and timestamp
3. Look up in export index
4. Replace with Google Drive/Docs URL or mark as external

**Alternatives Considered**:
| Alternative | Rejected Because |
|-------------|------------------|
| Keep all Slack links | Links become dead ends outside Slack |
| Remove Slack links | Loses context; user may want to know there was a reference |
| Two-pass export (export all, then link) | More complex; index approach works in single pass |

## Rate Limiting (Google Drive)

### Decision: Respect Google API quotas with exponential backoff

**Rationale**: Google Drive API has quotas:
- 12,000 requests per minute per user (generous)
- But batch operations count as multiple requests
- Document creation + formatting can hit limits on large exports

**Implementation**:
- Track request count per minute
- Exponential backoff on 429/503 responses
- Batch formatting requests where possible (Docs API supports batch updates)

**Pattern**:
```go
// Batch multiple formatting requests into single API call
requests := make([]*docs.Request, 0, 100)
for _, msg := range messages {
    requests = append(requests, formatMessage(msg)...)
}
// Single batch update instead of 100 individual calls
docsService.Documents.BatchUpdate(docId, &docs.BatchUpdateDocumentRequest{
    Requests: requests,
}).Do()
```
