## Why

The CI pipeline enforces three hard quality gates via `gaze report` that currently block every PR targeting `main`:

| Gate | Threshold | Current Value | Gap |
|------|-----------|---------------|-----|
| `--max-crapload` | ≤ 10 | 59 | 49 functions over |
| `--max-gaze-crapload` | ≤ 5 | 11 | 6 functions over |
| `--min-contract-coverage` | ≥ 50% | 31.4% | ~19 points short |

The root causes are:

1. **109 of 231 functions have 0% line coverage** — 5 CLI command handlers alone (runExport, runDiscover, runInit, runSetupBrowser, runList) have complexity 14–47 with zero tests, producing CRAP scores from 210 to 2,256.
2. **10 Q3 functions have tests that execute code but don't assert on contractual side effects** — line coverage exists but contract coverage is 0%, inflating GazeCRAPload.
3. **1 Q4 function (ConvertMrkdwnWithLinks, complexity 24)** has only 50% contract coverage and needs decomposition.
4. **pkg/chrome (4.4%) and pkg/gdrive (15.6%)** have the lowest package-level coverage, dragging down project averages.

Until these gaps are closed, no code can merge to `main`.

## What Changes

This change adds tests and refactors code across the project to pass all three CI quality gates. No user-facing behavior changes. No new dependencies. No configuration changes.

The work falls into four categories:

1. **Add unit tests** for the highest-CRAP zero-coverage functions to bring CRAPload from 59 to ≤ 10
2. **Add contract assertions** to existing tests that exercise code but don't verify return values, bringing contract coverage from 31.4% to ≥ 50%
3. **Decompose complex functions** where cyclomatic complexity alone keeps CRAP above threshold even with coverage
4. **Reduce GazeCRAPload** by asserting contractual side effects in the 11 functions currently above the GazeCRAP threshold

## Capabilities

### New Capabilities
- None — this is a quality improvement, not a feature change

### Modified Capabilities
- `internal/cli`: CLI command handlers refactored to extract testable logic from cobra RunE functions
- `pkg/exporter`: messageToBlock and ExportConversation decomposed into smaller, independently testable functions
- `pkg/parser`: ConvertMrkdwnWithLinks split into composable conversion stages
- `pkg/chrome`: Test coverage added for session management and credential extraction
- `pkg/gdrive`: Test coverage added for auth flow and token management

### Removed Capabilities
- None

## Impact

- **Test files**: New `_test.go` files and expanded existing test files across all 8 packages
- **Source files**: Refactored functions in `internal/cli`, `pkg/exporter`, and `pkg/parser` — extracted logic into smaller functions with the same public API
- **No behavioral changes**: All refactoring preserves existing public interfaces and behavior
- **CI pipeline**: Will unblock PRs to `main` by passing all three quality gates
- **Build time**: Test suite execution time will increase (more tests), but each individual test should be fast (no external services)

## Constitution Alignment

Assessed against the get-out project constitution.

### I. Session-Driven Extraction

**Assessment**: N/A

This change does not alter the extraction strategy. Browser session handling, token extraction, and API mimicry behavior remain unchanged. Tests use mocks and stubs — no real browser sessions or Slack connections required.

### II. Go-First Architecture

**Assessment**: PASS

All changes are pure Go. No new external dependencies are introduced. Refactored functions maintain the single-binary deployment model. Test helpers use only the standard library and existing test dependencies.

### III. Stealth & Reliability

**Assessment**: N/A

No changes to CDP communication, header mimicry, or rate limiting behavior. Tests validate the correctness of existing stealth logic without altering it.

### IV. Testability

**Assessment**: PASS

This change directly advances testability — the core goal is making previously untestable monolithic functions testable in isolation. Extracted functions accept interfaces and return values that can be verified without external services. Every new test follows the isolation principle: no network calls, no filesystem side effects beyond temp dirs, no real Google/Slack API calls.
