## 1. Slack API Godoc Contracts (`pkg/slackapi`)

- [x] 1.1 Update `ListConversations` godoc (client.go) — add return non-nil/nil contracts, nil-opts behavior, error classification description
- [x] 1.2 Update `GetAllMessages` godoc (client.go) — add callback invocation contract (when, error propagation), return nil on success, empty-parameter semantics
- [x] 1.3 Update `GetAllReplies` godoc (client.go) — add callback invocation contract, return nil on success, thread parent note
- [x] 1.4 Update `DownloadFile` godoc (client.go) — add return nil on error, 50MB cap mention, error wrapping
- [x] 1.5 Update `RateLimiter.Wait` godoc (ratelimit.go) — add mutation description (lastRequest timestamp), return nil vs ctx.Err() contracts, non-blocking fast-path
- [x] 1.6 Update `IsAuthError` godoc (errors.go) — add return true/false contracts, nil-input behavior, no-unwrap caveat, exhaustive error code list
- [x] 1.7 Update `IsNotFoundError` godoc (errors.go) — add return true/false contracts, nil-input behavior, no-unwrap caveat, exhaustive code list
- [x] 1.8 Update `TSToTime` godoc (types.go) — add return value for all input cases, no-error guarantee, zero-epoch for malformed input
- [x] 1.9 Run `go vet ./pkg/slackapi/...` — verify godoc format compliance
- [x] 1.10 Run `gaze crap --format=json ./pkg/slackapi` — verify Q3 count decreased and contract coverage increased

## 2. Parser Godoc Contracts (`pkg/parser`)

- [x] 2.1 Update `LoadUsers` godoc (resolver.go) — add mutation description (r.users overwrite), callback contract, variadic semantics, error return
- [x] 2.2 Update `LoadUsersForConversations` godoc (resolver.go) — add mutation description (r.users), callback invocation details, silent-skip behavior, error return
- [x] 2.3 Update `LoadChannels` godoc (resolver.go) — add mutation description (r.channels), return nil contract, overwrite semantics
- [x] 2.4 Update `NewPersonResolver` godoc (personresolver.go) — add always non-nil return, nil-input behavior, empty-mapping semantics
- [x] 2.5 Update `FindSlackLinks` godoc (mrkdwn.go) — add nil/empty-slice return contract, malformed-timestamp skip behavior
- [x] 2.6 Update `ReplaceSlackLinks` godoc (mrkdwn.go) — add callback invocation contract (when, with what args), empty-return passthrough, return value description
- [x] 2.7 Run `go vet ./pkg/parser/...` — verify godoc format compliance
- [x] 2.8 Run `gaze crap --format=json ./pkg/parser` — verify Q3 count decreased and contract coverage increased

## 3. Google Drive Godoc Contracts (`pkg/gdrive`)

- [x] 3.1 Update `ClientFromStore` godoc (auth.go) — add return nil/non-nil contracts, exhaustive error conditions, error wrapping
- [x] 3.2 Update `AuthenticateWithStore` godoc (auth.go) — add return nil/non-nil contracts, mutation (token saved), blocking behavior, warning side-effect
- [x] 3.3 Update `EnsureTokenFreshWithStore` godoc (auth.go) — add return nil contract (including warning-but-nil case), mutation (token), exhaustive error conditions
- [x] 3.4 Update `FindDocument` godoc (docs.go) — add critical (nil, nil) return contract, non-nil on found, error on API failure, empty folderID behavior
- [x] 3.5 Update `AppendText` godoc (docs.go) — add mutation description (remote doc), return nil on success, retry behavior, two-phase operation
- [x] 3.6 Update `InsertFormattedContent` godoc (docs.go) — add mutation (remote doc), empty-input fast-path, reverse-order insertion, return nil, retry
- [x] 3.7 Update `GetDocumentContent` godoc (docs.go) — add empty-string return contract, nil-body handling, error return
- [x] 3.8 Update `BatchAppendMessages` godoc (docs.go) — add mutation (remote doc), empty-input fast-path, two failure points, return nil
- [x] 3.9 Update `ListFolders` godoc (folder.go) — add nil/empty-slice not-an-error contract, empty parentID behavior, pagination, error wrapping
- [x] 3.10 Run `go vet ./pkg/gdrive/...` — verify godoc format compliance
- [x] 3.11 Run `gaze crap --format=json ./pkg/gdrive` — verify Q3 count decreased and contract coverage increased

## 4. Config Godoc Contracts (`pkg/config`)

- [x] 4.1 Update `LoadConversations` godoc (config.go) — add return nil on error, non-nil on success, error wrapping, validation side-effect
- [x] 4.2 Run `go vet ./pkg/config/...` — verify godoc format compliance
- [x] 4.3 Run `gaze crap --format=json ./pkg/config` — verify Q3 count decreased

## 5. Verification

- [x] 5.1 Run full test suite: `go test -race -count=1 ./...` — all tests MUST pass (no behavioral changes)
- [x] 5.2 Verify only `.go` production files were modified — `git diff --name-only` should show no `_test.go` files
- [x] 5.3 Run `go vet ./...` — no vet warnings
- [x] 5.4 Aggregate Q3 counts across all packages — target ≤ 5 total (down from 24)
