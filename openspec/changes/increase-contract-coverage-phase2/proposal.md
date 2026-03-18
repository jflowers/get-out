## Why

The phase 1 contract coverage change (`increase-contract-coverage`) added 21 test functions asserting on design responsibilities. The follow-up gaze report confirmed that all 24 Q3 functions remain Q3 because gaze's mechanical confidence scores stayed in the 58-79 range. The root cause is now understood: gaze classifies side effects as ambiguous when tests exercise the function but miss specific **boundary cases, nil inputs, parameter forwarding, and branch-path assertions** that would push confidence above 80.

Deep analysis of each Q3 function reveals a precise gap pattern:

| Confidence Band | Root Cause | Q3 Count | Fix Pattern |
|----------------|------------|----------|-------------|
| 58 (lowest) | Nil input never tested, wrapped-error behavior untested | 3 | Add nil/boundary tests |
| 63-64 | Empty/nil body paths, empty input identity, overwrite behavior | 7 | Add edge-case path tests |
| 69-70 | Parameter forwarding gaps, channelID not verified, empty-types | 11 | Add parameter capture tests |
| 79 (near threshold) | Debug path unexercised, explicit nil-return assertion | 3 | Add minor branch coverage |

This change targets all 24 Q3 functions with tests designed to push each function's confidence above 80.

## What Changes

Add targeted boundary-case and edge-case tests to existing test files across `pkg/slackapi`, `pkg/parser`, and `pkg/gdrive`. Each test targets a specific gap that gaze's confidence model penalizes: nil inputs, empty collections, parameter forwarding verification, identity-passthrough for no-op cases, overwrite-on-reload behavior, and untested branch paths.

No production code changes. Only `_test.go` files are modified.

## Capabilities

### New Capabilities
- None (test-only change)

### Modified Capabilities
- `pkg/slackapi/client_test.go`: New tests for channelID forwarding, nil/empty edge cases, API mode auth behavior, HasMore+empty-cursor termination
- `pkg/slackapi/errors_test.go`: New tests for nil and wrapped-error inputs to IsAuthError/IsNotFoundError
- `pkg/slackapi/ratelimit_test.go`: New test exercising debug logging path
- `pkg/slackapi/types_test.go`: New tests for empty/malformed TSToTime inputs
- `pkg/parser/resolver_api_test.go`: New tests for empty responses, overwrite-on-reload, nil channelIDs, duplicate SlackID handling
- `pkg/parser/parser_test.go`: New tests for empty-string inputs, nil-return assertions, resolver argument verification
- `pkg/gdrive/gdrive_test.go`: New tests for nil content, nil body paths, empty text, Unicode range computation, no-folderID queries, title escaping
- `pkg/gdrive/auth_test.go`: New tests for expired-with-refresh-token path, corrupt token handling

### Removed Capabilities
- None

## Impact

- **Files changed**: 6-8 test files (`*_test.go` only). Zero production code changes.
- **Risk**: Extremely low. Tests verify existing behavior at boundaries.
- **Expected outcome**: Q3 count should drop from 24 to ≤ 6. Per-package contract coverage should exceed 50% for `slackapi` and `parser`, and approach 40% for `gdrive`.
- **CI impact**: Moving functions from Q3 to Q1 reduces GazeCRAPload, strengthening the `--min-contract-coverage` gate.

## Constitution Alignment

Assessed against the get-out project constitution.

### I. Session-Driven Extraction

**Assessment**: N/A

Test-only change. No extraction behavior modified.

### II. Go-First Architecture

**Assessment**: PASS

All tests are in Go using the standard library `testing` package and existing test helpers. No new dependencies.

### III. Stealth & Reliability

**Assessment**: N/A

No runtime behavior changes.

### IV. Testability

**Assessment**: PASS

This change directly addresses the constitution's mandate for test-driven development by verifying observable side effects at boundaries. Each new test targets a specific contractual behavior — nil safety, identity passthrough, parameter forwarding — that the design promises but previously lacked explicit verification.
