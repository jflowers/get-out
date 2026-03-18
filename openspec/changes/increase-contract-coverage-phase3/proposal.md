## Why

After two phases of contract coverage work (50 new test functions), the gaze report shows Q3 count stable at 24 and per-package contract coverage unchanged at 20-27% for `slackapi`, `parser`, and `gdrive`. The root cause is confirmed: gaze's mechanical classifier uses code-structural signals (visibility, caller count, effect type), not test assertion patterns, to determine confidence.

Two levers remain to improve the quality metrics:

**Lever 1 — Line coverage for `pkg/gdrive` (51.3%)**
12 of 35 gdrive functions have 0% line coverage. These include `GetFolder`, `DeleteFolder`, `ShareFolder`, `UploadFile`, `FindOrCreateDocument`, `CreateNestedFolders`, `MakePublic`, `DeleteFile`, and others. All 12 are testable using the existing `testClient()` httptest helper — no real Google credentials required. Adding tests for these functions will raise gdrive line coverage from 51% to ~80%, directly reducing the package CRAPload and improving the overall CRAP profile.

**Lever 2 — Push 5 confidence-79 functions over the threshold**
Five functions sit at confidence 79, just 1 point below the contractual threshold (80). These need targeted error-path tests and contract assertions to cross over:

| Function | Package | Gap |
|----------|---------|-----|
| LoadConversations | config | Corrupt JSON + file-not-found error paths untested |
| DefaultSettings | config | No distinct-instance mutation-isolation test |
| ResolveWithFallback | parser | No concurrent-access test |
| RecordRateLimit | slackapi | `no_effects_detected` — gaze can't trace mutex-guarded mutations |
| ChannelResolver.Resolve | parser | Missing explicit non-empty return contract assertion |

## What Changes

**Workstream A**: Add tests for 12 zero-coverage gdrive functions using the existing `testClient()` httptest helper. Each test verifies the function's primary success path and at least one error path. Tests capture HTTP request details (method, path, body) to verify API contract compliance.

**Workstream B**: Add 5 targeted tests for the confidence-79 Q3 functions: error-path coverage for `LoadConversations`, distinct-instance test for `DefaultSettings`, concurrent-access test for `ResolveWithFallback`, observable-effect test for `RecordRateLimit`, and explicit contract assertions for `ChannelResolver.Resolve`.

No production code changes. Only `_test.go` files are modified.

## Capabilities

### New Capabilities
- None (test-only change)

### Modified Capabilities
- `pkg/gdrive/gdrive_test.go`: New tests for `DefaultConfig`, `GetFolder`, `FindOrCreateDocument`, `FindOrCreateFolder`, `CreateNestedFolders`, `DeleteFolder`, `ShareFolder`, `ShareFolderWithWriter`, `UploadFile`, `GetWebContentLink`, `MakePublic`, `DeleteFile`
- `pkg/config/config_test.go`: New tests for `LoadConversations` (corrupt JSON, file-not-found) and `DefaultSettings` (distinct instances)
- `pkg/parser/resolver_api_test.go`: New tests for `ResolveWithFallback` (concurrent access) and `ChannelResolver.Resolve` (contract assertions)
- `pkg/slackapi/ratelimit_test.go`: New test for `RecordRateLimit` (observable effect via `Wait` duration)

### Removed Capabilities
- None

## Impact

- **Files changed**: 4 test files (`*_test.go` only). Zero production code changes.
- **Risk**: Low. Tests verify existing behavior. The gdrive tests use httptest with the existing `testClient()` pattern.
- **Expected outcome**: gdrive line coverage from 51% → ~80%. gdrive CRAPload from 1 → 0. Q3 count should decrease by up to 5 (the confidence-79 functions). Total Q3 should drop from 24 to ≤ 19.
- **CI impact**: Higher line coverage reduces CRAP scores for previously-uncovered functions; confidence-79 functions crossing to Q1 reduces GazeCRAPload.

## Constitution Alignment

Assessed against the get-out project constitution.

### I. Session-Driven Extraction

**Assessment**: N/A

Test-only change. No extraction behavior modified.

### II. Go-First Architecture

**Assessment**: PASS

All tests are in Go using the standard library `testing` package and existing test helpers (`httptest.Server`, `testClient`). No new dependencies.

### III. Stealth & Reliability

**Assessment**: N/A

No runtime behavior changes.

### IV. Testability

**Assessment**: PASS

This change covers 12 previously untested gdrive functions using the existing httptest infrastructure, demonstrating that the functions ARE testable in isolation without external services — consistent with the constitution's testability mandate.
