## 1. Decompose CLI Command Handlers (internal/cli)

Extract testable business logic from monolithic cobra RunE handlers. Each handler becomes a thin flag-parsing wrapper that delegates to an extracted function accepting interfaces.

- [x] 1.1 Extract `runExport` (complexity 47, CRAP 2256) — pull config validation, conversation filtering, and export orchestration into `exportCore()` with injected dependencies; add tests for the extracted function covering primary success path, dry-run mode, date filtering, parallel mode, and error paths (missing config, invalid folder ID)
- [x] 1.2 Extract `runDiscover` (complexity 41, CRAP 1722) — pull user fetching, merge logic, and people.json writing into `discoverCore()` with injected Slack client and file writer; add tests covering discovery with merge, no-merge overwrite, empty conversation list, and API error handling
- [x] 1.3 Extract `runInit` (complexity 24, CRAP 600) — pull directory creation, migration call, and folder-ID prompt into `initCore()` with injected filesystem and prompter; add tests for fresh init, existing config dir, migration path, and invalid folder ID
- [ ] 1.4 Extract `runSetupBrowser` (complexity 22, CRAP 506) — extract each wizard step (chrome check, slack tab, credentials, team selection, token validation) into separate functions; add tests for each step with mock chrome session
- [x] 1.5 Extract `runDoctor` (complexity 10, CRAP 110) — extract check orchestration into `doctorCore()` with injected check functions; add tests verifying check aggregation and output formatting
- [x] 1.6 Extract `runList` (complexity 14, CRAP 210) — extract conversation loading and display formatting into `listCore()`; add tests for empty list, filtered list, and format output
- [x] 1.7 Extract `runStatus` (complexity 10, CRAP 110) — extract index loading and status formatting into `statusCore()`; add tests for no index, empty index, and populated index
- [x] 1.8 Increase coverage for `runAuthLogin` (complexity 9, CRAP 16, 56% coverage) — add tests for missing credentials, successful auth flow, and token refresh path to reach ≥ 80% coverage
- [x] 1.9 Increase coverage for `runAuthStatus` (complexity 10, CRAP 22, 50% coverage) — add tests for valid token, expired with refresh, and expired without refresh to reach ≥ 80% coverage

## 2. Decompose and Test pkg/exporter

- [x] 2.1 Decompose `(*DocWriter).messageToBlock` (complexity 27, CRAP 756) — split into `formatTextContent()`, `formatAttachments()`, `formatReactions()`, `formatThreadReference()` and a thin orchestrator; add tests for each extracted function with representative Slack message structures
- [ ] 2.2 Decompose `(*Exporter).ExportConversation` (complexity 21, CRAP 462) — extract message fetching, date grouping, and per-day doc creation into separate functions; add tests with mock Slack and Drive clients
- [ ] 2.3 Add tests for `(*Exporter).ExportAll` (complexity 11, CRAP 132) — test sequential export of multiple conversations, partial failure handling, and progress callback invocation using mock clients
- [ ] 2.4 Add tests for `(*Exporter).ExportAllParallel` (complexity 17, CRAP 306) — test parallel export with concurrency limit, error aggregation, and index checkpointing using mock clients
- [x] 2.5 Add tests for `(*Exporter).ResolveCrossLinks` (complexity 13, CRAP 182) — test link resolution across conversations, missing link handling, and Drive API error paths
- [x] 2.6 Add tests for `(*Exporter).resolveLinksInDoc` (complexity 7, CRAP 56) — test Slack URL replacement in doc content with various URL patterns
- [x] 2.7 Add tests for `(*Exporter).exportThread` (complexity 9, CRAP 90) — test thread export with replies, empty thread, and folder creation error
- [x] 2.8 Add tests for `(*DocWriter).WriteMessages` (complexity 7, CRAP 56) — test doc writing with multiple messages, empty message list, and formatting edge cases
- [ ] 2.9 Add tests for `(*Exporter).InitializeWithStore` (complexity 6, CRAP 42) — test initialization with valid store, missing credentials, and expired token
- [x] 2.10 Add tests for `(*Exporter).LoadUsersForConversations` (complexity 4, CRAP 20) — test user loading for multiple conversations, deduplication, and API errors
- [x] 2.11 Add tests for `(*FolderStructure).EnsureRootFolder` (complexity 5, CRAP 30), `EnsureConversationFolder` (complexity 6, CRAP 42), `EnsureThreadFolder` (complexity 6, CRAP 42), `EnsureThreadsFolder` (complexity 4, CRAP 20), `EnsureDailyDoc` (complexity 5, CRAP 30), `EnsureThreadDailyDoc` (complexity 6, CRAP 42) — test folder/doc creation, existing folder reuse, and Drive API error handling with mock Drive client

## 3. Test pkg/gdrive

- [x] 3.1 Add tests for `getTokenFromWeb` (complexity 13, CRAP 182) — test OAuth flow with mock HTTP server, callback handling, and error paths
- [x] 3.2 Add tests for `(*Client).InsertFormattedContent` (complexity 13, CRAP 182) — test formatted content insertion with mock Docs API, batch request building, and error handling
- [x] 3.3 Add tests for `(*Client).BatchAppendMessages` (complexity 10, CRAP 110) — test batch append with multiple messages, empty batch, and API rate limiting
- [x] 3.4 Add tests for `retryOnRateLimit` (complexity 8, CRAP 72) — test retry logic with immediate success, retry-after header, max retries exceeded
- [x] 3.5 Add tests for `(*Client).GetDocumentContent` (complexity 7, CRAP 56), `GetDocumentEndIndex` (complexity 4, CRAP 20), `FindDocument` (complexity 4, CRAP 20), `AppendText` (complexity 5, CRAP 30), `ReplaceText` (complexity 6, CRAP 42) — test each Docs API operation with mock HTTP responses
- [x] 3.6 Add tests for `(*Client).FindFolder` (complexity 4, CRAP 20), `ListFolders` (complexity 7, CRAP 56) — test folder operations with mock Drive API responses

## 4. Test pkg/slackapi

- [x] 4.1 Add tests for `(*Client).doRequest` (complexity 11, CRAP 132) — test request execution, rate limit handling (429 + Retry-After), auth errors, and timeout behavior with mock HTTP server
- [x] 4.2 Add tests for `(*Client).request` (complexity 7, CRAP 56) — test request building, header injection, and cookie attachment
- [x] 4.3 Add tests for `(*Client).GetConversationHistory` (complexity 9, CRAP 90), `GetConversationReplies` (complexity 9, CRAP 90) — test pagination, cursor handling, and empty response
- [x] 4.4 Add tests for `(*Client).GetAllMessages` (complexity 10, CRAP 110), `GetAllReplies` (complexity 10, CRAP 110) — test full pagination loop, message accumulation, and error mid-pagination
- [x] 4.5 Add tests for `(*Client).GetUsers` (complexity 4, CRAP 20), `GetConversationMembers` (complexity 4, CRAP 20), `ListConversations` (complexity 7, CRAP 56) — test user/member listing with pagination and filtering
- [x] 4.6 Add tests for `(*Client).DownloadFile` (complexity 6, CRAP 42) — test file download with mock HTTP response, redirect handling, and error paths

## 5. Test pkg/chrome

- [ ] 5.1 Add tests for `(*Session).ExtractCredentials` (complexity 11, CRAP 132) — test credential extraction with mock CDP session returning valid token and cookie, missing token, and missing cookie
- [ ] 5.2 Add tests for `(*Session).ExtractCredentialsForTeam` (complexity 10, CRAP 110) — test team-specific extraction with mock CDP returning multi-team config
- [ ] 5.3 Add tests for `(*Session).ListAvailableTeams` (complexity 7, CRAP 56) — test team listing from localStorage with mock CDP
- [x] 5.4 Add tests for `(*Session).ListTargets` (complexity 6, CRAP 42), `FindSlackTarget` (complexity 5, CRAP 30) — test target enumeration and Slack URL matching with mock CDP
- [ ] 5.5 Add tests for `(*Session).extractSlackCookie` (complexity 5, CRAP 30) — test cookie extraction with mock CDP cookie jar

## 6. Decompose and Test pkg/parser

- [ ] 6.1 Decompose `ConvertMrkdwnWithLinks` (complexity 24, GazeCRAP 96, Q4 Dangerous) — split into pipeline stages: `resolveMentions()`, `convertLinks()`, `applyFormatting()`, `resolveChannels()`; add tests for each stage independently
- [ ] 6.2 Add tests for `(*UserResolver).LoadUsersForConversations` (complexity 20, CRAP 420), `(*UserResolver).LoadUsers` (complexity 9, CRAP 90) — test user resolution with mock Slack client, empty user list, and API error
- [ ] 6.3 Add tests for `(*ChannelResolver).LoadChannels` (complexity 5, CRAP 30) — test channel loading with mock Slack client
- [x] 6.4 Add tests for `FindSlackLinks` (GazeCRAP 20), `ReplaceSlackLinks` (GazeCRAP 20) — test Slack URL pattern matching and replacement with contract assertions on return values

## 7. Test pkg/config and Remaining Gaps

- [x] 7.1 Add tests for `(*PeopleConfig).BuildEmailMap` (complexity 4, CRAP 20) — test email map building with various people configurations
- [x] 7.2 Add tests for `(*ConversationsConfig).FilterByType` (complexity 3, CRAP 12), `FilterByMode` (complexity 3, CRAP 12) — test filtering by conversation type and export mode

## 8. Add Contract Assertions to Existing Tests (GazeCRAPload)

Add explicit assertions on contractual return values in tests that currently execute functions without verifying outputs.

- [x] 8.1 Add contract assertions for `config.LoadConversations` (GazeCRAP 30, 0% contract coverage) — assert on returned config struct contents, not just error
- [x] 8.2 Add contract assertions for `gdrive.ClientFromStore` (GazeCRAP 42, 0% contract coverage) — assert returned client is non-nil and configured correctly
- [x] 8.3 Add contract assertions for `gdrive.AuthenticateWithStore` (GazeCRAP 72, 0% contract coverage) — assert on returned token properties
- [x] 8.4 Add contract assertions for `gdrive.EnsureTokenFreshWithStore` (GazeCRAP 72, 0% contract coverage) — assert on token freshness and error messages
- [x] 8.5 Add contract assertions for `parser.NewPersonResolver` (GazeCRAP 30, 0% contract coverage) — assert on resolver state after construction
- [x] 8.6 Add contract assertions for `slackapi.IsAuthError`, `IsNotFoundError` (GazeCRAP 20 each, 0% contract coverage) — assert on boolean return values for various error inputs
- [x] 8.7 Add contract assertions for `slackapi.TSToTime` (GazeCRAP 20, 0% contract coverage) — assert on returned time value, not just error

## 9. Verification

- [ ] 9.1 Run full test suite: `go test -race -count=1 -coverprofile=coverage.out ./...` — all tests MUST pass
- [ ] 9.2 Run gaze CRAPload check: `gaze crap --coverprofile=coverage.out --max-crapload=10 ./...` — MUST exit 0
- [ ] 9.3 Run gaze GazeCRAPload check: `gaze crap --coverprofile=coverage.out --max-gaze-crapload=5 ./...` — MUST exit 0
- [ ] 9.4 Run gaze contract coverage check: `gaze quality --min-contract-coverage=50 ./...` — MUST exit 0
- [ ] 9.5 Verify all existing tests unchanged: `git diff --name-only` should show no modifications to existing test assertions (new files and new test functions only, unless explicitly adding contract assertions to existing tests per task group 8)
- [ ] 9.6 Verify constitution alignment: no new external dependencies introduced, all tests run in isolation without network access, all public API signatures preserved
