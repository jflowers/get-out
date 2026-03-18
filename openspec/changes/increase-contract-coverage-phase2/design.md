## Context

Phase 1 added 21 test functions asserting on design responsibilities (formatting variants, parameter forwarding, size caps, error paths). The gaze report confirmed these tests pass and exercise the right code paths, but the Q3 count held at 24 because gaze's mechanical classifier evaluates **confidence of side-effect contractuality** — not just whether assertions exist, but whether **all observable behaviors** at boundaries are verified.

The deep analysis of each Q3 function reveals that gaze penalizes functions when:
1. **Nil inputs** are never passed (the function accepts `error`, `[]string`, `*Config` but `nil` is never tested)
2. **Empty collections** are never passed (empty slice vs nil slice, empty string vs non-empty)
3. **Identity/passthrough** behavior is unverified (function returns input unchanged when no transformation applies)
4. **Branch paths** are unexercised (debug logging, `HasMore=true` with empty cursor, nil body)
5. **Parameter capture** is incomplete (channelID forwarded but never verified at the HTTP level)
6. **Overwrite semantics** are untested (calling Load twice doesn't verify second-call-wins)

## Goals / Non-Goals

### Goals
- Push each Q3 function's confidence above 80 (the contractual threshold)
- Add boundary-case tests that target the specific confidence gap for each function
- Reduce Q3 count from 24 to ≤ 6 across `slackapi`, `parser`, `gdrive`
- Raise per-package contract coverage above 50% for `slackapi` and `parser`
- Follow existing test patterns (httptest.Server, mockSlackAPI, docsMux, table-driven tests)

### Non-Goals
- Achieving 100% contract coverage (some functions like `getTokenFromWeb` require network)
- Refactoring production code to improve testability
- Adding tests for untestable-by-design paths (browser OAuth flow, real token refresh)
- Fixing SSA panics in `internal/cli`, `pkg/chrome`, or `pkg/exporter`

## Decisions

### 1. Target boundary cases, not more happy-path assertions

Phase 1 proved that adding more happy-path assertions doesn't shift confidence scores. The fix is boundary-case tests: nil inputs, empty strings, wrapped errors, overwrite-on-reload. These test the **edges** of the function's contract, which is exactly what gaze's model rewards.

### 2. One test per confidence gap, not one test per function

Some functions have a single gap (e.g., `RateLimiter.Wait` just needs a debug-path test to go from 79 → 83). Others have multiple gaps (e.g., `ReplaceSlackLinks` at 58 needs nil/empty/passthrough/resolver-argument tests). The task list is organized by gap type, not by function.

### 3. Test categories by gap type

| Category | Tests | Target Functions |
|----------|------:|-----------------|
| Nil/empty input boundaries | ~12 | IsAuthError, IsNotFoundError, TSToTime, FindSlackLinks, ReplaceSlackLinks, InsertFormattedContent, LoadUsers, LoadUsersForConversations, LoadChannels |
| Parameter forwarding verification | ~4 | GetAllMessages, GetAllReplies (channelID); FindDocument (no-folderID, quote escaping) |
| Branch-path exercise | ~4 | RateLimiter.Wait (debug), DownloadFile (API mode), AppendText (nil body), GetAllMessages (HasMore+empty cursor) |
| Overwrite/reload semantics | ~3 | LoadUsers, LoadChannels, NewPersonResolver (duplicate SlackID) |
| Identity passthrough | ~3 | ReplaceSlackLinks (no URLs), FindSlackLinks (nil return), ReplaceSlackLinks (resolver arguments) |
| Auth edge cases | ~3 | ClientFromStore (expired+refresh), AuthenticateWithStore (corrupt token), EnsureTokenFreshWithStore (error content) |

### 4. Package-by-package order: slackapi → parser → gdrive

Slackapi has the most Q3 functions (8) and includes the lowest-confidence functions (IsAuthError/IsNotFoundError at 58). Parser is next (6 Q3). Gdrive last (9 Q3, but some are auth functions with hard-to-test refresh paths).

## Risks / Trade-offs

### Confidence model uncertainty

Gaze's confidence scoring is a heuristic. Adding the exact boundary tests identified may not push every function above 80 if the model weighs signals we haven't identified. However, each test is independently valuable as a contract verification.

**Mitigation**: Run `gaze crap --format=json` per-package after each group to measure impact. Adjust strategy if specific functions don't respond.

### Nil-input tests may reveal panics

Some boundary tests (e.g., `ReplaceSlackLinks(text, nil)`) may expose actual panics in production code. This is valuable information, not a problem — it reveals missing nil guards in the contract.

**Mitigation**: If a nil-input test panics, document the panic and skip the test with a `// TODO: add nil guard` comment rather than blocking the change.
