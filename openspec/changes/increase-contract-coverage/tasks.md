## 1. Google Docs Contract Assertions (`pkg/gdrive`)

- [x] 1.1 Add `TestInsertFormattedContent_Italic` — call `InsertFormattedContent` with a content block that has `Italic: true`; assert the BatchUpdate request contains an `UpdateTextStyle` with `TextStyle.Italic = true` and field mask includes `italic`
- [x] 1.2 Add `TestInsertFormattedContent_Monospace` — call with `Monospace: true`; assert `UpdateTextStyle` with `WeightedFontFamily.FontFamily = "Courier New"` and field mask includes `weightedFontFamily`
- [x] 1.3 Add `TestInsertFormattedContent_Combined` — call with `Bold: true`, `Italic: true`, and `LinkURL` set; assert a single `UpdateTextStyle` with all three fields and correct field mask
- [x] 1.4 Add `TestInsertFormattedContent_MultipleSegments` — call with 3 content blocks; assert `InsertText` and `UpdateTextStyle` requests are in reverse order with correct index ranges
- [x] 1.5 Add `TestInsertFormattedContent_APIError` — configure test server to return HTTP 500 for the batchUpdate endpoint; assert returned error is non-nil
- [x] 1.6 Add `TestBatchAppendMessages_WithImages` — call with a message containing an `ImageAnnotation`; assert the BatchUpdate request contains an `InsertInlineImage` request with the image URL and surrounding newlines
- [x] 1.7 Add `TestBatchAppendMessages_MultipleMessages` — call with 2 messages; assert both are inserted and the second message's indices account for the first message's content length
- [x] 1.8 Add `TestBatchAppendMessages_LinkTextNotInBody` — call with a `LinkAnnotation` whose `Text` does not appear in the message body; assert no `UpdateTextStyle` for that link and no error
- [x] 1.9 Add `TestAppendText_GetError` — configure test server to return HTTP 500 for the Documents.Get endpoint; assert `AppendText` returns a non-nil error wrapping the API failure
- [x] 1.10 Add `TestGetDocumentContent_APIError` — configure test server to return HTTP 500 for Documents.Get; assert error is non-nil and contains "failed to get document"
- [x] 1.11 Run `go test -race -count=1 ./pkg/gdrive/...` — all tests MUST pass
- [x] 1.12 Run `gaze crap --format=json ./pkg/gdrive` — verify `avg_contract_coverage` increased above 30% and Q3 count decreased

## 2. Google Drive Auth Contract Assertions (`pkg/gdrive`)

- [x] 2.1 Add `TestClientFromStore_BadCredentialsJSON` — store invalid JSON as credentials; assert `ClientFromStore` returns an error mentioning credential parsing
- [x] 2.2 Run `go test -race -count=1 ./pkg/gdrive/...` — all tests MUST pass

## 3. Slack API Contract Assertions (`pkg/slackapi`)

- [x] 3.1 Add `TestGetAllMessages_ForwardsOldestLatest` — call `GetAllMessages` with `oldest` and `latest` set; capture the HTTP request in the test handler; assert `oldest` and `latest` form parameters match the caller-provided values
- [x] 3.2 Add `TestGetAllMessages_EmptyPageSkipsCallback` — configure test server to return zero messages with `has_more: false`; assert the callback is never invoked
- [x] 3.3 Add `TestGetAllReplies_ForwardsThreadTS` — call `GetAllReplies` with a specific `threadTS`; capture the HTTP request; assert `ts` form parameter matches the provided `threadTS`
- [x] 3.4 Add `TestListConversations_ExcludeArchived` — call `ListConversations` with `ExcludeArchived: true`; assert the HTTP request includes `exclude_archived=true`
- [x] 3.5 Add `TestListConversations_NilOptions` — call `ListConversations` with `nil` options; assert no panic and a valid response is returned
- [x] 3.6 Add `TestListConversations_DefaultLimit` — call `ListConversations` with default options; assert the HTTP request includes `limit=200`
- [x] 3.7 Add `TestListConversations_ReturnValueAssertions` — call `ListConversations` with a test server returning 2 channels; assert `len(resp.Channels) == 2` and each channel has populated `ID` and `Name` fields
- [x] 3.8 Add `TestDownloadFile_SizeCap` — configure test server to return a response body larger than 50MB (use `io.LimitReader` or a fixed-size buffer); assert the returned `[]byte` has length ≤ 50 * 1024 * 1024
- [x] 3.9 Run `go test -race -count=1 ./pkg/slackapi/...` — all tests MUST pass
- [x] 3.10 Run `gaze crap --format=json ./pkg/slackapi` — verify `avg_contract_coverage` increased above 40% and Q3 count decreased

## 4. Parser Resolver Contract Assertions (`pkg/parser`)

- [x] 4.1 Add `TestLoadChannels_ContextCancellation` — create a `ChannelResolver` with a mock `SlackAPI` that returns `has_more: true` on the first page; cancel the context before the second page; assert `LoadChannels` returns `context.Canceled`
- [x] 4.2 Add `TestLoadChannels_TypesFilter` — create a `ChannelResolver` with a mock `SlackAPI` that captures the `ListConversationsOptions.Types`; call `LoadChannels`; assert the captured types equal `["public_channel", "private_channel"]`
- [x] 4.3 Run `go test -race -count=1 ./pkg/parser/...` — all tests MUST pass
- [x] 4.4 Run `gaze crap --format=json ./pkg/parser` — verify `avg_contract_coverage` increased above 35% and Q3 count decreased

## 5. Verification

- [x] 5.1 Run full test suite: `go test -race -count=1 ./...` — all tests MUST pass with no regressions
- [x] 5.2 Run per-package gaze analysis for all three packages and confirm overall Q3 count ≤ 16 (down from 24)
- [x] 5.3 Verify no production code was modified — only `_test.go` files should appear in `git diff --name-only`
- [x] 5.4 Verify constitution alignment: Testability principle (§IV) — all new tests verify observable side effects in isolation without requiring external services; Composability principle (§II) — each package's tests pass independently
