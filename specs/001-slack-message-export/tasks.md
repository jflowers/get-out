# Tasks: Slack Message Export to Google Docs

**Input**: Design documents from `/specs/001-slack-message-export/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/slack-api.md, quickstart.md

**Tests**: Test tasks included for core functionality (parser, checkpoint, API clients).

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4, US5)

## Path Conventions

Project structure from plan.md:
- `cmd/get-out/` - CLI entry point
- `pkg/` - Reusable libraries (chrome, slackapi, gdrive, parser, exporter, models)
- `internal/cli/` - Cobra commands
- `tests/` - Unit and integration tests

---

## Phase 1: Setup (Project Initialization)

**Purpose**: Initialize Go project structure and dependencies

- [ ] T001 Create project directory structure per plan.md (cmd/, pkg/, internal/, tests/)
- [ ] T002 Initialize go.mod with module `github.com/jflowers/get-out` and Go 1.21+
- [ ] T003 [P] Add chromedp dependency `github.com/chromedp/chromedp v0.9.5`
- [ ] T004 [P] Add cobra dependency `github.com/spf13/cobra v1.8.0`
- [ ] T005 [P] Add errgroup dependency `golang.org/x/sync v0.6.0`
- [ ] T006 [P] Add Google API dependencies `google.golang.org/api v0.160.0` and `golang.org/x/oauth2 v0.16.0`
- [ ] T007 Create pkg/models/models.go with domain types from data-model.md (Conversation, Message, User, Reaction, Attachment, ExportSession, UserMapping, ExportIndex)
- [ ] T008 Create .gitignore for Go project (binaries, .env, credentials.json, token.json, user-mapping.json)

**Checkpoint**: Project compiles with `go build ./...`

---

## Phase 2: Foundational (Chrome + Slack + Google Auth)

**Purpose**: Core infrastructure that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

### Config Loading

- [ ] T009 Create pkg/config/types.go with config struct definitions:
  - Conversation struct (id, name, type, mode, export, share, shareMembers)
  - Person struct (slackId, email, displayName, googleEmail, noNotifications, noShare)
  - ConversationsConfig and PeopleConfig wrapper structs
- [ ] T010 Create pkg/config/config.go with config file loading:
  - LoadConversations(path) for conversations.json
  - LoadPeople(path) for people.json
  - Validation functions for config data
  - Filter functions (by export=true, by mode)

### Slack Infrastructure

- [ ] T011 Create pkg/chrome/chrome.go with RemoteAllocator connection to Chrome debugging port
- [ ] T012 Create pkg/chrome/token.go with ExtractToken function to get xoxc token from localStorage
- [ ] T013 Create pkg/slackapi/types.go with API response types (ConversationsListResponse, HistoryResponse, etc.)
- [ ] T014 Create pkg/slackapi/client.go with base HTTP client supporting both auth modes:
  - NewBrowserClient(xoxcToken, xoxdCookie) for browser mode
  - NewAPIClient(xoxbToken) for API mode
  - Common request/response handling
- [ ] T015 [P] Create pkg/slackapi/errors.go with error types (RateLimitError, AuthError, NotFoundError)

### Google Drive Infrastructure

- [ ] T014 Create pkg/gdrive/auth.go with OAuth 2.0 flow:
  - Load credentials.json from config directory
  - Browser-based consent flow for first auth
  - Save/load refresh token to token.json
  - Auto-refresh access token on expiry
- [ ] T015 Create pkg/gdrive/client.go with Google Drive/Docs service initialization
- [ ] T016 Create pkg/gdrive/folder.go with folder management:
  - CreateFolder function with parent folder support
  - FindOrCreateFolder for nested folder creation
  - GetFolderID by name and parent
- [ ] T017 Create pkg/gdrive/docs.go with Google Docs creation:
  - CreateDocument with title in specific folder
  - BatchUpdate for content insertion
  - Text formatting helpers (bold, italic, monospace, hyperlinks)
  - InsertLink helper for internal doc linking

### Export Index & Structure

- [ ] T018 Create pkg/exporter/index.go with ExportIndex management:
  - Load/save export-index.json from _metadata folder
  - Track conversation folders, thread docs, daily docs
  - Lookup methods for link replacement
- [ ] T019 Create pkg/exporter/structure.go with folder organization:
  - CreateConversationFolder (e.g., "DM - John Smith")
  - CreateThreadsSubfolder
  - CreateDailyDoc with date-based naming
  - CreateThreadDoc with topic preview naming

### CLI Base

- [ ] T020 Create internal/cli/root.go with Cobra root command and global flags (--debug, --chrome-port)
- [ ] T021 Create internal/cli/auth.go with `auth` command for Google OAuth setup
- [ ] T022 Create cmd/get-out/main.go entry point that calls internal/cli/root.go
- [ ] T023 Create internal/cli/discover.go with `discover` command:
  - Read conversations.json to get list of configured conversations
  - For each conversation, fetch member list from Slack API
  - Fetch user info (name, email, display name) for all members
  - Generate/update people.json with user mappings
  - Support both browser mode (xoxc token) and API mode (xoxb token)
  - --merge flag to merge with existing people.json (default: true)
  - Skip users already in people.json when merging

**Checkpoint**: `go run ./cmd/get-out auth` completes OAuth flow; `go run ./cmd/get-out discover` generates config files

---

## Phase 3: User Story 1 - Export Direct Messages (Priority: P1) MVP

**Goal**: Export a single DM conversation to Google Docs with daily chunking and proper folder structure

**Independent Test**: Run `get-out export D123ABC456` and verify:
- A folder "DM - John Smith" is created in Drive
- Daily Google Docs are created (e.g., "2026-02-01.gdoc", "2026-02-02.gdoc")
- Messages have sender names, timestamps, and formatted content

### Tests for User Story 1

- [ ] T023 [P] [US1] Create tests/unit/parser_test.go with mrkdwn to Docs conversion tests (mentions, links, formatting)
- [ ] T024 [P] [US1] Create tests/unit/resolver_test.go with user ID resolution tests
- [ ] T025 [P] [US1] Create tests/integration/slackapi_test.go with mock HTTP server for conversations.history
- [ ] T026 [P] [US1] Create tests/unit/gdrive_test.go with Docs formatting request tests
- [ ] T027 [P] [US1] Create tests/unit/index_test.go with export index save/load tests

### Implementation for User Story 1

- [ ] T028 [US1] Implement pkg/slackapi/conversations.go with ListConversations and GetHistory methods
- [ ] T029 [US1] Implement pkg/slackapi/users.go with GetUser and ListUsers methods
- [ ] T030 [US1] Create pkg/parser/resolver.go with UserResolver that builds ID→name map from users.list
- [ ] T031 [US1] Create pkg/parser/mrkdwn.go with ConvertToDocsRequests function:
  - Convert `<@U123>` to `@Real Name` (bold text)
  - Convert `<#C123|channel>` to `#channel`
  - Convert `<url|text>` to hyperlink
  - Convert code blocks to monospace font (Courier New)
  - Return slice of Google Docs API requests
- [ ] T032 [US1] Create pkg/exporter/docwriter.go with:
  - WriteDailyDoc: create doc for a single day's messages
  - Group messages by sender with timestamps
  - Batch formatting requests for efficiency
- [ ] T033 [US1] Create pkg/exporter/exporter.go with ExportConversation orchestrator:
  - Extract Slack token via chrome package
  - Verify Google auth via gdrive package
  - Create conversation folder structure
  - Fetch user list for resolution
  - Fetch all messages via pagination
  - Group messages by date
  - Create daily docs for each date
  - Update export index with created docs
- [ ] T034 [US1] Create internal/cli/export.go with `export` command:
  - Accept conversation ID as argument
  - `--folder` flag for Drive root folder name (default: "Slack Exports")
  - Progress output during export (messages fetched, docs created)
  - Print folder URL on completion
- [ ] T035 [US1] Add basic error handling in exporter (auth failures, not found, network errors)

**Checkpoint**: Can export a DM conversation to a folder with daily Google Docs

---

## Phase 4: User Story 2 - Export Threads to Subfolder (Priority: P2)

**Goal**: Export threads to a "Threads" subfolder with daily-chunked docs per thread folder, linked from main daily docs

**Independent Test**: Run `get-out export G123ABC456` on a group with threads and verify:
- A "Threads" subfolder exists in the conversation folder
- Each thread has its own folder named by start date and topic preview
- Thread folders contain daily-chunked docs (same pattern as main conversation)
- Main daily docs link to thread folders where threads start
- Thread docs contain replies with proper formatting

### Tests for User Story 2

- [ ] T036 [P] [US2] Add tests/unit/thread_test.go with thread detection, topic extraction, and daily grouping tests
- [ ] T037 [P] [US2] Add tests/integration/replies_test.go with mock for conversations.replies

### Implementation for User Story 2

- [ ] T038 [US2] Implement pkg/slackapi/replies.go with GetReplies method for conversations.replies endpoint
- [ ] T039 [US2] Update pkg/exporter/exporter.go to detect threaded messages (thread_ts field present)
- [ ] T040 [US2] Add thread fetching logic: for each parent message with replies, fetch via GetReplies
- [ ] T041 [US2] Create pkg/exporter/threadwriter.go with:
  - ExtractTopicPreview: first 30 chars of parent message, sanitized for folder name
  - CreateThreadFolder: create folder in Threads subfolder named `{date} - {topic}`
  - GroupRepliesByDate: organize thread replies by date
  - WriteThreadDailyDocs: create daily docs within thread folder (same pattern as main)
- [ ] T042 [US2] Update pkg/exporter/docwriter.go to add thread links:
  - When parent message has replies, add "[View thread: N replies]" link
  - Link points to the thread folder in Threads subfolder
- [ ] T043 [US2] Update export index with thread folder and doc entries (conversation_id:thread_ts → folder URL, daily doc URLs)
- [ ] T044 [US2] Handle pagination in thread replies (has_more, cursor)

**Checkpoint**: Can export conversations with threads properly organized in subfolders with cross-links

---

## Phase 5: User Story 3 - Google User Mapping (Priority: P3)

**Goal**: Replace Slack @mentions with Google account links where mapping exists

**Independent Test**: Export a conversation with user-mapping.json configured, verify mentions link to Google profiles/emails

### Tests for User Story 3

- [ ] T045 [P] [US3] Create tests/unit/usermapping_test.go with mapping load and lookup tests

### Implementation for User Story 3

- [ ] T046 [US3] Create pkg/parser/usermapping.go with:
  - LoadUserMapping from user-mapping.json
  - GetGoogleAccount(slackUserID) → (email, name, found)
  - UserMapping struct with Slack ID → Google email/name
- [ ] T047 [US3] Update pkg/parser/mrkdwn.go to check user mapping:
  - If Google account exists: create mailto: link or Google profile link
  - If no mapping: use Slack display name (bold, no link)
- [ ] T048 [US3] Update pkg/parser/resolver.go to cache user map to file (users.json in _metadata)
- [ ] T049 [US3] Add fallback for unresolvable users: `@Unknown (U123ABC456)` format
- [ ] T050 [US3] Handle bot users with is_bot flag (show [bot] indicator)
- [ ] T051 [US3] Handle deleted users with deleted flag (show [deactivated] indicator)
- [ ] T052 [US3] Add batch user fetching optimization: collect all user IDs first, fetch in parallel with errgroup
- [ ] T053 [US3] Add `--user-mapping` flag to export command to specify mapping file path

**Checkpoint**: Mentions resolve to Google links where mapping exists, graceful fallback otherwise

---

## Phase 6: User Story 4 - Replace Slack Links with Export Links (Priority: P3)

**Goal**: Replace links to other Slack messages/threads/channels with links to exported Google Docs

**Independent Test**: Export multiple conversations, verify cross-references link to correct Google Docs

### Tests for User Story 4

- [ ] T054 [P] [US4] Create tests/unit/linkreplacer_test.go with Slack URL parsing and replacement tests

### Implementation for User Story 4

- [ ] T055 [US4] Create pkg/parser/linkreplacer.go with:
  - ParseSlackLink: extract conversation ID, timestamp, thread_ts from Slack URLs
  - ReplaceSlackLinks: find all Slack links in message, replace with Drive links
  - Regex patterns for slack.com/archives/... URLs
- [ ] T056 [US4] Update pkg/parser/mrkdwn.go to call linkreplacer during conversion
- [ ] T057 [US4] Implement link replacement logic:
  - Channel link (slack.com/archives/C123) → conversation folder URL
  - Message link (slack.com/archives/C123/p1234) → daily doc URL (with anchor if possible)
  - Thread link (slack.com/archives/C123/p1234?thread_ts=...) → thread doc URL
  - If target not exported: keep original link + "[not in export]" note
- [ ] T058 [US4] Update export index lookups to support all link types
- [ ] T059 [US4] Add second-pass link replacement for cross-conversation references:
  - After all conversations exported, re-scan for links to newly exported content
  - Update docs with resolved links (or mark as optional enhancement)

**Checkpoint**: Slack links within exported content point to corresponding Google Docs

---

## Phase 7: User Story 5 - Handle Rate Limiting Gracefully (Priority: P3)

**Goal**: Automatic retry with exponential backoff for both Slack and Google APIs, checkpointing for resume

**Independent Test**: Simulate rate limit during large export and verify it completes without data loss

### Tests for User Story 5

- [ ] T060 [P] [US5] Create tests/unit/checkpoint_test.go with checkpoint save/load tests
- [ ] T061 [P] [US5] Create tests/unit/ratelimit_test.go with backoff calculation tests

### Implementation for User Story 5

- [ ] T062 [US5] Update pkg/slackapi/client.go with rate limit detection (429 status, ratelimited error)
- [ ] T063 [US5] Implement exponential backoff in Slack client: initial 1s, max 60s, respect Retry-After header
- [ ] T064 [US5] Update pkg/gdrive/client.go with rate limit handling for Google APIs (429/503)
- [ ] T065 [US5] Create pkg/exporter/checkpoint.go with SaveCheckpoint and LoadCheckpoint:
  - State: conversation_id, last_ts, messages_exported, current_date, folder_id, status
  - Save after each daily doc is created
  - File: `.checkpoint.json` in config directory
- [ ] T066 [US5] Update pkg/exporter/exporter.go to check for existing checkpoint on start
- [ ] T067 [US5] Add `--resume` flag to export command that loads checkpoint and continues
- [ ] T068 [US5] Add progress reporting: show message count, docs created, rate limit waits, ETA

**Checkpoint**: Large exports survive rate limits and interruptions; can resume seamlessly

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: CLI completeness, documentation, and production readiness

- [ ] T069 Create internal/cli/list.go with `list` command:
  - Show all DMs, groups, channels user has access to
  - Format: ID, name, type, member count
  - `--type` filter flag (dm, group, channel)
- [ ] T070 Create internal/cli/status.go with `status` command:
  - Show active/paused exports from checkpoints
  - Display progress, folder URL, and last update time
- [ ] T071 Add `--all-dms` flag to export command for batch DM export
- [ ] T072 Add `--all-groups` flag to export command for batch group export
- [ ] T073 Implement parallel conversation export with semaphore (max 3 concurrent)
- [ ] T074 Add session expiry detection in chrome package (clear error message)
- [ ] T075 Add Google token refresh check before each export
- [ ] T076 [P] Update README.md with installation, Google Cloud setup, user-mapping.json format, and usage
- [ ] T077 [P] Update quickstart.md with full workflow including threads and user mapping
- [ ] T078 Add `--since` and `--until` date filters to export command
- [ ] T079 Final code cleanup: remove debug prints, add comments, format with gofmt

**Checkpoint**: Production-ready CLI tool with all documented features working

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - start immediately
- **Foundational (Phase 2)**: Depends on Setup - BLOCKS all user stories
- **User Stories (Phase 3-7)**: All depend on Foundational completion
  - US1 (P1): No dependencies on other stories - **MVP**
  - US2 (P2): Depends on US1 folder structure
  - US3 (P3): Independent, enhances US1/US2 output
  - US4 (P3): Depends on export index from US1/US2
  - US5 (P3): Independent, adds resilience
- **Polish (Phase 8)**: Depends on US1 minimum; full features need US1-US5

### User Story Dependencies

```
Phase 1 (Setup)
    ↓
Phase 2 (Foundational: Slack + Google + Index) ─── BLOCKS ALL ───┐
    ↓                                                             ↓
┌────────────────────────────────────────────────────────────────────────────────┐
│  US1 (Daily Docs)  →  US2 (Threads)  →  US4 (Link Replace)                    │
│       ↓                    ↓                    ↓                              │
│   [MVP Ready]         [Threads]          [Cross-links]                         │
│                                                                                │
│  US3 (Google Users)        US5 (Rate Limits)                                  │
│       ↓                         ↓                                             │
│   [Mentions]              [Resilience]                                         │
└────────────────────────────────────────────────────────────────────────────────┘
    ↓
Phase 8 (Polish)
```

### Parallel Opportunities

**Phase 1**: T003, T004, T005, T006 (dependencies)
**Phase 2**: T013 (errors.go) parallel with T014-T17 (gdrive)
**Phase 3**: T023, T024, T025, T026, T027 (all tests)
**Phase 4**: T036, T037 (tests)
**Phase 5**: T045 (tests)
**Phase 6**: T054 (tests)
**Phase 7**: T060, T061 (tests)
**Phase 8**: T076, T077 (docs)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T008)
2. Complete Phase 2: Foundational (T009-T022)
3. Complete Phase 3: User Story 1 (T023-T035)
4. **STOP and VALIDATE**: Export a real DM conversation to Google Docs folder with daily docs
5. **MVP READY**: Basic DM export with folder structure works

### Incremental Delivery

| Milestone | Stories Complete | Capability |
|-----------|------------------|------------|
| MVP | US1 | Export DMs to folders with daily docs |
| v0.2 | US1 + US2 | + Thread subfolders with cross-links |
| v0.3 | US1 + US2 + US3 | + Google user linking |
| v0.4 | US1 + US2 + US3 + US4 | + Slack link replacement |
| v0.5 | US1 + US2 + US3 + US4 + US5 | + Rate limiting & resume |
| v1.0 | All + Polish | Production ready |

---

## Task Summary

| Phase | Tasks | Parallel | Description |
|-------|-------|----------|-------------|
| 1. Setup | 8 | 4 | Project initialization |
| 2. Foundational | 14 | 1 | Chrome + Slack + Google + Index |
| 3. US1 (Daily Docs) | 13 | 5 | MVP - folder structure + daily docs |
| 4. US2 (Threads) | 9 | 2 | Thread subfolders |
| 5. US3 (Google Users) | 9 | 1 | User mapping |
| 6. US4 (Link Replace) | 6 | 1 | Slack → Drive links |
| 7. US5 (Rate Limit) | 9 | 2 | Resilience |
| 8. Polish | 11 | 2 | CLI completion |
| **Total** | **79** | **18** | |

---

## Configuration Files

### user-mapping.json (Optional)

Place in `~/.config/get-out/` or specify with `--user-mapping`:

```json
{
  "U123ABC456": {
    "google_email": "john.smith@company.com",
    "google_name": "John Smith"
  },
  "U789DEF012": {
    "google_email": "jane.doe@gmail.com"
  }
}
```

### Google Cloud Setup (Prerequisites)

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project (e.g., "get-out-slack-export")
3. Enable APIs: Google Drive API, Google Docs API
4. Create OAuth 2.0 credentials (Desktop app type)
5. Download as `credentials.json` to `~/.config/get-out/`

---

## Output Structure Example

```
Google Drive/
└── Slack Exports/
    ├── DM - John Smith/
    │   ├── 2026-02-01.gdoc                      # Main daily messages
    │   ├── 2026-02-02.gdoc
    │   └── Threads/
    │       └── 2026-02-01 - Project update/     # Thread folder
    │           ├── 2026-02-01.gdoc              # Thread replies day 1
    │           └── 2026-02-02.gdoc              # Thread replies day 2
    ├── Group - project-alpha/
    │   ├── 2026-01-15.gdoc
    │   ├── 2026-01-16.gdoc
    │   └── Threads/
    │       ├── 2026-01-15 - Sprint planning/    # Multi-day thread
    │       │   ├── 2026-01-15.gdoc
    │       │   ├── 2026-01-16.gdoc
    │       │   └── 2026-01-17.gdoc
    │       └── 2026-01-16 - Bug discussion/     # Single-day thread
    │           └── 2026-01-16.gdoc
    └── _metadata/
        ├── users.json
        ├── user-mapping.json
        └── export-index.json
```

---

## Notes

- [P] tasks can run in parallel (different files, no dependencies)
- [Story] label maps task to user story for traceability
- Each checkpoint validates that phase works independently
- Commit after each task or logical group
- MVP achievable with Phases 1-3 (35 tasks)
- Thread support adds 9 tasks (Phase 4)
- Google user mapping adds 9 tasks (Phase 5)
- Link replacement adds 6 tasks (Phase 6)
