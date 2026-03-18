## 1. Slack API — Nil/Boundary Tests (`pkg/slackapi`)

- [x] 1.1 Add `TestIsAuthError_NilInput` — assert `IsAuthError(nil)` returns `false`
- [x] 1.2 Add `TestIsAuthError_WrappedError` — wrap `*AuthError` with `fmt.Errorf("%w", ...)`, assert `IsAuthError` returns `false` (no `errors.As`)
- [x] 1.3 Add `TestIsNotFoundError_NilInput` — assert `IsNotFoundError(nil)` returns `false`
- [x] 1.4 Add `TestIsNotFoundError_WrappedError` — wrap `*NotFoundError` with `fmt.Errorf`, assert returns `false`
- [x] 1.5 Add `TestTSToTime_EmptyString` — assert `TSToTime("").Unix() == 0`
- [x] 1.6 Add `TestTSToTime_MalformedInput` — assert `TSToTime("not.a.number")` doesn't panic and returns Unix epoch

## 2. Slack API — Parameter Forwarding and Branch Tests (`pkg/slackapi`)

- [x] 2.1 Add `TestGetAllMessages_ForwardsChannelID` — capture HTTP form param, assert `channel=C_TARGET`
- [x] 2.2 Add `TestGetAllMessages_HasMoreWithEmptyCursor` — server returns `has_more:true, next_cursor:""`, assert callback invoked exactly once (no infinite loop)
- [x] 2.3 Add `TestGetAllReplies_ForwardsChannelID` — capture HTTP form param, assert `channel=C_REPLIES`
- [x] 2.4 Add `TestListConversations_EmptyTypesSlice` — pass `Types: []string{}`, assert no `types` param in request (or empty)
- [x] 2.5 Add `TestDownloadFile_APIModeNoCookie` — use `newAPITestClient`, assert no Cookie header, assert auth header present
- [x] 2.6 Add `TestRateLimiter_Wait_DebugPath` — call `SetDebug(true)`, call `Wait` twice, assert nil return
- [x] 2.7 Run `go test -race -count=1 ./pkg/slackapi/...` — all tests MUST pass

## 3. Parser — Boundary and Overwrite Tests (`pkg/parser`)

- [x] 3.1 Add `TestLoadUsers_EmptyResponse` — mock returns zero users, assert `Count() == 0` and no error
- [x] 3.2 Add `TestLoadUsers_OverwriteOnReload` — load U001 with "old-name", reload with "new-name", assert `Resolve("U001") == "new-name"`
- [x] 3.3 Add `TestLoadUsersForConversations_NilChannelIDs` — pass nil (not empty slice), assert no API calls made and no error
- [x] 3.4 Add `TestLoadChannels_EmptyResponse` — mock returns zero channels, assert `Resolve("C999") == "C999"`
- [x] 3.5 Add `TestLoadChannels_OverwriteOnReload` — load C1="old", reload C1="new", assert `Resolve("C1") == "new"`
- [x] 3.6 Add `TestNewPersonResolver_DuplicateSlackID` — two entries with same SlackID, assert last-writer-wins for ResolveName and Count
- [x] 3.7 Add `TestFindSlackLinks_EmptyString` — assert `len(FindSlackLinks("")) == 0`
- [x] 3.8 Add `TestReplaceSlackLinks_EmptyText` — assert `ReplaceSlackLinks("", resolver) == ""`
- [x] 3.9 Add `TestReplaceSlackLinks_NoSlackURLs` — text without URLs, assert returned string equals input unchanged
- [x] 3.10 Add `TestReplaceSlackLinks_ResolverArguments` — capture channelID and messageTS passed to resolver, assert correct values
- [x] 3.11 Run `go test -race -count=1 ./pkg/parser/...` — all tests MUST pass

## 4. Google Drive — Boundary and Edge Case Tests (`pkg/gdrive`)

- [x] 4.1 Add `TestInsertFormattedContent_NilContent` — pass nil content, assert no API call and nil error
- [x] 4.2 Add `TestInsertFormattedContent_UnicodeTextRange` — bold emoji text "😀", assert UpdateTextStyle endIndex uses UTF-16 length (startIndex + 2, not + 4)
- [x] 4.3 Add `TestAppendText_NilBody` — document response with no body field, assert insert at index 0
- [x] 4.4 Add `TestAppendText_EmptyText` — append empty string, assert InsertText request sent with `""`
- [x] 4.5 Add `TestFindDocument_NoFolderID` — empty folderID, assert query does not contain `in parents`
- [x] 4.6 Add `TestFindDocument_TitleWithQuotes` — title with single quote, assert query escapes it
- [x] 4.7 Add `TestClientFromStore_ExpiredWithRefreshToken` — expired token with refresh_token set, assert returns non-nil client (not error)
- [x] 4.8 Run `go test -race -count=1 ./pkg/gdrive/...` — all tests MUST pass

## 5. Verification

- [x] 5.1 Run full test suite: `go test -race -count=1 ./...` — all tests MUST pass with no regressions
- [x] 5.2 Run `gaze crap --format=json` per-package for slackapi, parser, gdrive — verify Q3 counts decreased
- [x] 5.3 Verify no production code was modified — only `_test.go` files in `git diff --name-only`
- [x] 5.4 Verify constitution alignment: Testability (§IV) — all tests verify boundary behaviors in isolation without external services
