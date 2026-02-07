# Tasks: Slack Message Export to Google Docs

**Input**: Design documents from `/specs/001-slack-message-export/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/slack-api.md, quickstart.md  
**Last Updated**: 2026-02-07

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

- [x] T001 Create project directory structure per plan.md (cmd/, pkg/, internal/, tests/)
- [x] T002 Initialize go.mod with module `github.com/jflowers/get-out` and Go 1.21+
- [x] T003 [P] Add chromedp dependency `github.com/chromedp/chromedp v0.9.5`
- [x] T004 [P] Add cobra dependency `github.com/spf13/cobra v1.8.0`
- [x] T005 [P] Add errgroup dependency `golang.org/x/sync v0.6.0`
- [x] T006 [P] Add Google API dependencies `google.golang.org/api v0.160.0` and `golang.org/x/oauth2 v0.16.0`
- [x] T007 Create pkg/models/models.go with domain types from data-model.md (Conversation, Message, User, Reaction, Attachment, ExportSession, UserMapping, ExportIndex)
- [x] T008 Create .gitignore for Go project (binaries, .env, credentials.json, token.json, user-mapping.json)

**Checkpoint**: Project compiles with `go build ./...`

---

## Phase 2: Foundational (Chrome + Slack + Google Auth)

**Purpose**: Core infrastructure that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

### Config Loading

- [x] T009 Create pkg/config/types.go with config struct definitions:
  - Conversation struct (id, name, type, mode, export, share, shareMembers)
  - Person struct (slackId, email, displayName, googleEmail, noNotifications, noShare)
  - ConversationsConfig and PeopleConfig wrapper structs
- [x] T010 Create pkg/config/config.go with config file loading:
  - LoadConversations(path) for conversations.json
  - LoadPeople(path) for people.json
  - Validation functions for config data
  - Filter functions (by export=true, by mode)

### Slack Infrastructure

- [x] T011 Create pkg/chrome/chrome.go with RemoteAllocator connection to Chrome debugging port
- [x] T012 Create pkg/chrome/token.go with ExtractToken function to get xoxc token from localStorage
- [x] T013 Create pkg/slackapi/types.go with API response types (ConversationsListResponse, HistoryResponse, etc.)
- [x] T014 Create pkg/slackapi/client.go with base HTTP client supporting both auth modes:
  - NewBrowserClient(xoxcToken, xoxdCookie) for browser mode
  - NewAPIClient(xoxbToken) for API mode
  - Common request/response handling
- [x] T015 [P] Create pkg/slackapi/errors.go with error types (RateLimitError, AuthError, NotFoundError)

### Google Drive Infrastructure

- [x] T014 Create pkg/gdrive/auth.go with OAuth 2.0 flow:
  - Load credentials.json from config directory
  - Browser-based consent flow for first auth
  - Save/load refresh token to token.json
  - Auto-refresh access token on expiry
- [x] T015 Create pkg/gdrive/client.go with Google Drive/Docs service initialization
- [x] T016 Create pkg/gdrive/folder.go with folder management:
  - CreateFolder function with parent folder support
  - FindOrCreateFolder for nested folder creation
  - GetFolderID by name and parent
- [x] T017 Create pkg/gdrive/docs.go with Google Docs creation:
  - CreateDocument with title in specific folder
  - BatchUpdate for content insertion
  - Text formatting helpers (bold, italic, monospace, hyperlinks)
  - InsertLink helper for internal doc linking

### Export Index & Structure

- [x] T018 Create pkg/exporter/index.go with ExportIndex management:
  - Load/save export-index.json from _metadata folder
  - Track conversation folders, thread docs, daily docs
  - Lookup methods for link replacement
- [x] T019 Create pkg/exporter/structure.go with folder organization:
  - CreateConversationFolder (e.g., "DM - John Smith")
  - CreateThreadsSubfolder
  - CreateDailyDoc with date-based naming
  - CreateThreadDoc with topic preview naming

### CLI Base

- [x] T020 Create internal/cli/root.go with Cobra root command and global flags (--debug, --chrome-port)
- [x] T021 Create internal/cli/auth.go with `auth` command for Google OAuth setup
- [x] T022 Create cmd/get-out/main.go entry point that calls internal/cli/root.go
- [x] T023 Create internal/cli/discover.go with `discover` command:
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

- [x] T023 [P] [US1] Create tests/unit/parser_test.go with mrkdwn to Docs conversion tests (mentions, links, formatting)
- [x] T024 [P] [US1] Create tests/unit/resolver_test.go with user ID resolution tests
- [x] T025 [P] [US1] Create tests/integration/slackapi_test.go with mock HTTP server for conversations.history
- [ ] T026 [P] [US1] Create tests/unit/gdrive_test.go with Docs formatting request tests
- [x] T027 [P] [US1] Create tests/unit/index_test.go with export index save/load tests

### Implementation for User Story 1

- [x] T028 [US1] Implement pkg/slackapi/conversations.go with ListConversations and GetHistory methods
- [x] T029 [US1] Implement pkg/slackapi/users.go with GetUser and ListUsers methods
- [x] T030 [US1] Create pkg/parser/resolver.go with UserResolver that builds ID→name map from users.list
- [x] T031 [US1] Create pkg/parser/mrkdwn.go with ConvertToDocsRequests function:
  - Convert `<@U123>` to `@Real Name` (bold text)
  - Convert `<#C123|channel>` to `#channel`
  - Convert `<url|text>` to hyperlink
  - Convert code blocks to monospace font (Courier New)
  - Return slice of Google Docs API requests
- [x] T032 [US1] Create pkg/exporter/docwriter.go with:
  - WriteDailyDoc: create doc for a single day's messages
  - Group messages by sender with timestamps
  - Batch formatting requests for efficiency
- [x] T033 [US1] Create pkg/exporter/exporter.go with ExportConversation orchestrator:
  - Extract Slack token via chrome package
  - Verify Google auth via gdrive package
  - Create conversation folder structure
  - Fetch user list for resolution
  - Fetch all messages via pagination
  - Group messages by date
  - Create daily docs for each date
  - Update export index with created docs
- [x] T034 [US1] Create internal/cli/export.go with `export` command:
  - Accept conversation ID as argument
  - `--folder` flag for Drive root folder name (default: "Slack Exports")
  - Progress output during export (messages fetched, docs created)
  - Print folder URL on completion
- [x] T035 [US1] Add basic error handling in exporter (auth failures, not found, network errors)

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

- [x] T038 [US2] Implement pkg/slackapi/replies.go with GetReplies method for conversations.replies endpoint
- [x] T039 [US2] Update pkg/exporter/exporter.go to detect threaded messages (thread_ts field present)
- [x] T040 [US2] Add thread fetching logic: for each parent message with replies, fetch via GetReplies
- [x] T041 [US2] Create pkg/exporter/threadwriter.go with:
  - ExtractTopicPreview: first 30 chars of parent message, sanitized for folder name
  - CreateThreadFolder: create folder in Threads subfolder named `{date} - {topic}`
  - GroupRepliesByDate: organize thread replies by date
  - WriteThreadDailyDocs: create daily docs within thread folder (same pattern as main)
- [x] T042 [US2] Update pkg/exporter/docwriter.go to add thread links:
  - When parent message has replies, add "[View thread: N replies]" link
  - Link points to the thread folder in Threads subfolder
- [x] T043 [US2] Update export index with thread folder and doc entries (conversation_id:thread_ts → folder URL, daily doc URLs)
- [x] T044 [US2] Handle pagination in thread replies (has_more, cursor)

**Checkpoint**: Can export conversations with threads properly organized in subfolders with cross-links

---

## Phase 5: User Story 3 - Google User Mapping (Priority: P3)

**Goal**: Replace Slack @mentions with Google account links where mapping exists

**Independent Test**: Export a conversation with user-mapping.json configured, verify mentions link to Google profiles/emails

### Tests for User Story 3

- [ ] T045 [P] [US3] Create tests/unit/usermapping_test.go with mapping load and lookup tests

### Implementation for User Story 3

- [x] T046 [US3] Create person→email resolver (implemented as `pkg/parser/personresolver.go`):
  - Loads from people.json (populated by `discover` command)
  - Maps Slack user ID → Google email
- [x] T047 [US3] Update mrkdwn conversion for Google account linking:
  - `ConvertMrkdwnWithLinks` returns `LinkAnnotation` for mentions with Google emails
  - `BatchAppendMessages` applies `UpdateTextStyle` with `mailto:` `Link`
- [x] T048 [US3] Wire PersonResolver through Exporter → DocWriter → messageToBlock
- [x] T049 [US3] Graceful fallback: no people.json = display names only, no links
- [x] T050 [US3] Handle bot users: [bot] indicator for IsBot/IsAppUser
- [x] T051 [US3] Handle deleted users: [deactivated] indicator for Deleted flag
- [x] T052 [US3] Batch user loading: LoadUsersForConversations fetches only relevant users
- [ ] T053 [US3] Add `--user-mapping` flag to export command to specify mapping file path

**Checkpoint**: @mentions resolve to Google mailto: links where people.json mapping exists ✅

---

## Phase 6: User Story 4 - Replace Slack Links with Export Links (Priority: P3)

**Goal**: Replace links to other Slack messages/threads/channels with links to exported Google Docs

**Independent Test**: Export multiple conversations, verify cross-references link to correct Google Docs

### Tests for User Story 4

- [ ] T054 [P] [US4] Create tests/unit/linkreplacer_test.go with Slack URL parsing and replacement tests

### Implementation for User Story 4

- [x] T055 [US4] Slack link parsing and replacement (already existed in `mrkdwn.go`):
  - `FindSlackLinks` + `ReplaceSlackLinks` parse slack.com/archives/ URLs
  - `SlackLinkResolver` callback type for resolution
- [x] T056 [US4] Integrated link replacement into `ConvertMrkdwnWithLinks`:
  - `ReplaceSlackLinks` called before other mrkdwn conversions
  - Resolver callback wired through `DocWriter` → `ExportIndex.LookupDocURL`
- [x] T057 [US4] Link replacement logic:
  - Message link → daily doc URL via `LookupDocURL` (timestamp → date → doc)
  - Conversation link → folder URL via `LookupConversationURL`
  - Thread link → thread folder via `LookupThreadURL`
  - Unresolved links kept as-is
- [x] T058 [US4] Export index lookups support all link types (already existed):
  - `LookupDocURL`, `LookupThreadURL`, `LookupConversationURL`
- [ ] T059 [US4] Second-pass link replacement for cross-conversation references (optional)

**Checkpoint**: Slack archive URLs in exported content point to Google Docs ✅

---

## Phase 7: User Story 5 - Handle Rate Limiting Gracefully (Priority: P3)

**Goal**: Automatic retry with exponential backoff for both Slack and Google APIs, checkpointing for resume

**Independent Test**: Simulate rate limit during large export and verify it completes without data loss

### Tests for User Story 5

- [ ] T060 [P] [US5] Create tests/unit/checkpoint_test.go with checkpoint save/load tests
- [ ] T061 [P] [US5] Create tests/unit/ratelimit_test.go with backoff calculation tests

### Implementation for User Story 5

- [x] T062 [US5] Update pkg/slackapi/client.go with rate limit detection (429 status, ratelimited error)
- [x] T063 [US5] Implement exponential backoff in Slack client: initial 1s, max 60s, respect Retry-After header
- [x] T064 [US5] Update pkg/gdrive/docs.go with rate limit handling for Google APIs (429)
- [x] T065 [US5] Create checkpoint system in pkg/exporter:
  - Status field tracks in_progress/complete per conversation
  - Index saved after each daily doc for granular checkpointing
- [x] T066 [US5] Update pkg/exporter/exporter.go to check for existing checkpoint on start
- [x] T067 [US5] Add `--resume` flag to export command that skips completed conversations
- [x] T068 [US5] Add progress reporting: show message count, docs created, rate limit waits, ETA

**Checkpoint**: Large exports survive rate limits and interruptions; can resume seamlessly

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: CLI completeness, documentation, and production readiness

- [x] T069 Create internal/cli/list.go with `list` command:
  - Show all DMs, groups, channels user has access to
  - Format: ID, name, type, member count
  - `--type` filter flag (dm, group, channel)
- [x] T070 Create internal/cli/status.go with `status` command:
  - Show exported conversations from index
  - Display status, message count, doc count, thread count, last updated
- [ ] T071 Add `--all-dms` flag to export command for batch DM export
- [ ] T072 Add `--all-groups` flag to export command for batch group export
- [ ] T073 Implement parallel conversation export with semaphore (max 3 concurrent)
- [ ] T074 Add session expiry detection in chrome package (clear error message)
- [ ] T075 Add Google token refresh check before each export
- [x] T076 [P] Update README.md with features, commands, project structure, and usage examples
- [x] T077 [P] Update quickstart.md with full workflow, threads, user mapping, and all current features
- [x] T078 Add `--from` and `--to` date filters and `--sync` incremental export to export command
- [x] T079 Final code cleanup: gofmt applied to all Go files

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
4. **STOP and VALIDATE**: Export a real DM conversation to Google Docs folder with daily docs ✅ (1,094 msgs → 149 docs)
5. **MVP READY**: Basic DM export with folder structure works ✅

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
