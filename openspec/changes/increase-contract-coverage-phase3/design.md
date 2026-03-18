## Context

Phases 1 and 2 added 50 test functions (assertions, boundary cases, parameter forwarding, nil inputs) across `slackapi`, `parser`, and `gdrive`. The Q3 count held at 24 because gaze's mechanical classifier is driven by code-structural signals. Two actionable levers remain: (1) covering the 12 zero-coverage gdrive functions to raise line coverage, and (2) targeting the 5 confidence-79 functions with specific gap-filling tests.

## Goals / Non-Goals

### Goals
- Raise `pkg/gdrive` line coverage from 51% to ~80% by testing 12 zero-coverage functions
- Push ≤ 5 confidence-79 Q3 functions over the threshold to Q1
- Reduce gdrive CRAPload from 1 to 0
- Demonstrate that folder/file operations are testable with httptest (constitution compliance)
- Verify `RecordRateLimit`'s mutex-guarded mutations through observable public API behavior

### Non-Goals
- Testing `NewClient` and `NewClientFromStore` (require mock Google discovery documents; deferred to a refactoring change)
- Achieving 100% line coverage for any package
- Changing production code to improve testability (test-only change)
- Targeting functions below confidence 70 (these need document-enhanced scoring, not more tests)

## Decisions

### 1. Use existing `testClient()` helper for all gdrive tests

The `gdrive_test.go` file already has a `testClient(t, mux)` function that creates a `*Client` backed by `httptest.NewServer`. All 12 zero-coverage functions can be tested through this pattern — the Google Drive/Docs API calls go to a local HTTP server. This is the same pattern used by all existing gdrive tests (50+ tests).

For each test:
1. Register handlers on `http.ServeMux` for the specific Google API endpoints
2. Create a client via `testClient(t, mux)`
3. Call the function under test
4. Assert the HTTP request (method, path, body) and the function's return values

### 2. Drive API endpoint patterns

The Google Drive API uses these URL patterns, which the test mux must handle:

| Operation | Method | URL Pattern |
|-----------|--------|-------------|
| Get file/folder | GET | `/files/{id}` |
| List files | GET | `/files?q=...` |
| Create file | POST | `/files` |
| Update file | PATCH | `/files/{id}` |
| Delete file | DELETE | `/files/{id}` |
| Create permission | POST | `/files/{id}/permissions` |
| Upload (multipart) | POST | `/upload/files` |

The existing `docsMux()` helper handles `GET /v1/documents/` and `:batchUpdate` for Docs API. For Drive API, handlers go directly on `/files/` and `/upload/files`.

### 3. Confidence-79 tests target the specific gap

Each confidence-79 function has a specific, identified gap:

| Function | Gap | Fix |
|----------|-----|-----|
| LoadConversations | Error paths untested | Add corrupt-JSON and file-not-found tests |
| DefaultSettings | No mutation isolation | Add distinct-instance test |
| ResolveWithFallback | No concurrency test | Add goroutine-safety test with WaitGroup |
| RecordRateLimit | `no_effects_detected` | Test observable effect via `Wait()` duration change |
| ChannelResolver.Resolve | Missing edge cases | Add empty-string ID and explicit non-empty assertions |

### 4. Exclude `NewClient` and `NewClientFromStore`

These two functions chain through Google's discovery document resolution (`drive.NewService()`, `docs.NewService()`), which makes HTTP calls to `googleapis.com/.../$discovery/rest`. Mocking this requires serving valid OpenAPI/Discovery JSON documents, which is fragile and complex. These are better addressed by a separate refactoring change that adds `...option.ClientOption` parameters.

### 5. Test error paths, not just success paths

Every new gdrive test covers both the success path and at least one error path (HTTP 500, HTTP 404, or malformed response). This ensures the tests verify the full return contract, not just the happy path.

## Risks / Trade-offs

### Google API JSON structure changes

Tests that parse Google API response JSON may break if the API response structure changes. However, this risk is already accepted by all existing gdrive tests, and the `google-api-go-client` library handles serialization — tests only need to provide minimal valid JSON.

### Drive API routing complexity

The Drive API uses `/files/{id}` for multiple operations (GET, PATCH, DELETE). Test muxes must route by HTTP method. The existing mux pattern in `gdrive_test.go` already handles this for the Docs API (`/v1/documents/` routes by `:batchUpdate` suffix).

**Mitigation**: Each test registers only the handlers it needs, avoiding cross-test interference.
