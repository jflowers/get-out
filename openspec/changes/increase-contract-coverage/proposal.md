## Why

The latest gaze quality report (2026-03-17) shows contract coverage averaging 20–27% across the three largest packages (`parser`, `slackapi`, `gdrive`), with 24 functions landing in the Q3 "Needs Tests" quadrant. The CI pipeline enforces a `--min-contract-coverage 50%` gate, and while it currently passes (due to module-level SSA degradation masking accurate measurement), per-package data shows the actual state is below threshold for three of six packages:

| Package | Avg Contract Coverage | Q3 Count |
|---------|----------------------|----------|
| gdrive | 20.0% | 9 |
| parser | 26.1% | 6 |
| slackapi | 26.9% | 8 |
| config | 77.8% | 1 |
| secrets | 62.5% | 0 |

The root cause is consistent: tests exercise the code paths (line coverage is 82–100%) but don't assert on the contractual side effects that gaze identifies — return value properties, error conditions, parameter forwarding, and state mutations. The tests check for `err == nil` and move on, leaving the function's design responsibilities unverified.

## What Changes

Add targeted contract assertions to existing tests and write new test cases for untested branches in `pkg/parser`, `pkg/slackapi`, and `pkg/gdrive`. Each assertion verifies a specific side effect that gaze classifies as contractual or ambiguous-likely-contractual:

- **Return value assertions**: Verify not just `err == nil` but the properties of returned values (field contents, slice lengths, non-nil nested fields).
- **Parameter forwarding assertions**: Verify that caller-provided parameters (`oldest`, `latest`, `threadTS`, `ExcludeArchived`) actually reach the underlying API calls via HTTP request inspection.
- **Error contract assertions**: Verify specific error messages, error types, and error wrapping behavior for failure paths.
- **Branch coverage for untested paths**: Add tests for branches with 0% coverage that represent real design responsibilities (image annotations, formatting variants, size caps).

No production code changes. Only `_test.go` files are modified.

## Capabilities

### New Capabilities
- None (test-only change)

### Modified Capabilities
- `pkg/gdrive/docs_test.go`: New test cases for `InsertFormattedContent` (italic, monospace, combined formatting, multi-segment, API error), `BatchAppendMessages` (image annotations, multiple messages, link-not-found), `AppendText` (API error paths), `GetDocumentContent` (API error path)
- `pkg/gdrive/gdrive_test.go`: New test cases for `ClientFromStore` (bad credentials JSON)
- `pkg/slackapi/client_test.go`: New test cases for `ListConversations` (ExcludeArchived, nil opts, default limit), `GetAllMessages` (oldest/latest forwarding, empty page), `GetAllReplies` (threadTS forwarding), `DownloadFile` (50MB size cap)
- `pkg/parser/resolver_test.go`: New test case for `LoadChannels` (context cancellation during pagination, types filter assertion)

### Removed Capabilities
- None

## Impact

- **Files changed**: 4 test files (`*_test.go` only). Zero production code changes.
- **Risk**: Extremely low. Tests can only break if the existing production code doesn't match its design contract — which is information worth discovering.
- **CI impact**: Contract coverage should rise above the 50% threshold for all packages, satisfying the `--min-contract-coverage 50%` CI gate at the per-package level.
- **GazeCRAPload**: Q3 count should decrease as newly-asserted side effects shift from "ambiguous" to "contractual with coverage," moving functions from Q3 into Q1.

## Constitution Alignment

Assessed against the get-out project constitution.

### I. Session-Driven Extraction

**Assessment**: N/A

This change adds test assertions only. No extraction behavior is modified.

### II. Go-First Architecture

**Assessment**: PASS

All tests are written in Go using the standard library `testing` package and existing test helpers (`httptest.Server`, `testClient`). No new dependencies are introduced.

### III. Stealth & Reliability

**Assessment**: N/A

No runtime behavior changes. Rate limiting, header mimicry, and CDP usage are unaffected.

### IV. Testability

**Assessment**: PASS

This change directly improves testability by verifying observable side effects in isolation. Every new assertion targets a specific contractual behavior of the function under test, following the constitution's mandate for test-driven development and the gaze quality model's contract coverage metric.
