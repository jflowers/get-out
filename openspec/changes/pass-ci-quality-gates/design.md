## Context

The CI pipeline enforces three quality gates that currently fail:

| Gate | Required | Current | Strategy |
|------|----------|---------|----------|
| CRAPload ≤ 10 | ≤ 10 | 59 | Add tests + decompose complex functions |
| GazeCRAPload ≤ 5 | ≤ 5 | 11 | Add contract assertions to existing tests |
| Contract coverage ≥ 50% | ≥ 50% | 31.4% | Assert on return values and error paths |

The 59 CRAPload functions break down by root cause:

- **21 functions** have 0% coverage + complexity ≥ 10 (the "big monoliths")
- **35 functions** have 0% coverage + complexity 4–9 (testable without refactoring)
- **3 functions** have partial coverage but CRAP still > 15 (need more coverage or decomposition)

The key insight: we don't need to fix all 59. We need to bring 49 below the CRAP threshold of 15. The most efficient path is:

1. **Decompose the top 5 monoliths** (complexity 20–47) into smaller functions — this removes them from CRAPload AND produces smaller functions that are cheaper to test
2. **Add tests for 35 medium-complexity functions** (complexity 4–9) — these are testable as-is with interface mocking
3. **Increase coverage for 3 partially-covered functions** — small test additions

## Goals / Non-Goals

### Goals
- Reduce CRAPload from 59 to ≤ 10
- Reduce GazeCRAPload from 11 to ≤ 5
- Increase average contract coverage from 31.4% to ≥ 50%
- Preserve all existing public APIs and behavior
- All tests run in isolation (no network, no real browser, no real Google/Slack)

### Non-Goals
- 100% line coverage — not required, not pursued
- Refactoring for architectural beauty — only decompose where complexity directly blocks the quality gates
- Adding integration or end-to-end tests — unit tests only
- Changing the CI gate thresholds — the gates are the target, not the variable
- Testing `cmd/get-out/main.go` — it's a thin wrapper, not worth the `covdata` workaround

## Decisions

### 1. Extract-and-Test pattern for CLI command handlers

The 7 CLI `run*` functions (runExport, runDiscover, runInit, etc.) are monolithic cobra RunE handlers that mix argument parsing, dependency construction, and business logic. Refactoring approach:

- **Extract the business logic** into a separate unexported function that accepts interfaces (e.g., `exportCore(cfg ExportConfig, slack SlackClient, drive DriveClient) error`)
- **Keep the cobra handler thin**: parse flags, construct dependencies, call the extracted function
- **Test the extracted function** with mock implementations of each interface
- **Don't test the cobra handler itself** — it's just glue code

This follows the constitution's Testability principle: components testable in isolation without external services.

### 2. Interface-based mocking for external dependencies

Functions in `pkg/chrome`, `pkg/gdrive`, and `pkg/slackapi` make real network calls. Testing approach:

- Define interfaces for the external operations each function depends on (HTTP client, browser session, Drive API)
- Use constructor injection or function parameters to accept interfaces
- Tests provide stub implementations that return canned responses
- No mocking frameworks — plain Go interface implementations in `_test.go` files

Where interfaces already exist (e.g., `SecretStore`), use them directly. Where they don't, add the minimal interface needed.

### 3. Decompose functions with complexity > 15

Five functions have cyclomatic complexity 20+, making them nearly impossible to test adequately:

| Function | Complexity | Decomposition Strategy |
|----------|-----------|----------------------|
| `runExport` (47) | Extract config validation, conversation filtering, export orchestration into separate functions |
| `runDiscover` (41) | Extract user fetching, merging logic, file writing into separate functions |
| `messageToBlock` (27) | Split by message element type: text formatting, attachments, reactions, thread references |
| `runInit` (24) | Extract directory creation, migration, and folder-ID prompt into separate functions |
| `runSetupBrowser` (22) | Extract each wizard step into its own function |
| `ConvertMrkdwnWithLinks` (24) | Split into pipeline stages: mention resolution, link conversion, formatting |
| `ExportConversation` (21) | Extract message fetching, date grouping, doc creation into separate functions |
| `LoadUsersForConversations` (20) | Extract single-conversation user loading, deduplication |

Each decomposed function gets its own tests. The parent function becomes a thin orchestrator that's either testable with the new smaller pieces or drops below threshold via reduced complexity.

### 4. Contract assertion strategy for GazeCRAPload

The 11 GazeCRAPload functions have tests that call them but don't assert on contractual return values. Fix approach:

- **Return value assertions**: Every test that calls a function with a non-void return MUST assert on the return value
- **Error path assertions**: Tests for error cases MUST verify the specific error content, not just `err != nil`
- **State mutation assertions**: Where a function modifies a struct, assert on the mutated state

Target: move all 11 GazeCRAPload functions to Q1 (Safe) or at minimum reduce the count to ≤ 5.

### 5. Prioritization by CRAP-reduction efficiency

Not all functions contribute equally to the gates. The work is ordered by "CRAP points removed per unit of effort":

**Phase 1 — Decompose monoliths** (removes ~10 functions from CRAPload, unlocks testability for extracted pieces):
- `runExport`, `runDiscover`, `messageToBlock`, `runInit`, `runSetupBrowser`, `ConvertMrkdwnWithLinks`, `ExportConversation`, `LoadUsersForConversations`

**Phase 2 — Test medium-complexity functions** (removes ~30 functions from CRAPload, highest ROI):
- All 35 zero-coverage functions with complexity 4–9 across `pkg/gdrive`, `pkg/slackapi`, `pkg/chrome`, `pkg/exporter`

**Phase 3 — Contract assertions** (fixes GazeCRAPload and contract coverage):
- Add assertions to the 11 GazeCRAPload functions
- Review all existing tests for missing return-value assertions

**Phase 4 — Remaining coverage gaps** (mop up the last few CRAPload entries):
- Increase coverage for `runAuthLogin` (56% → 80%+), `runAuthStatus` (50% → 80%+)
- Test `ExportAllParallel` and `ExportAll` via the decomposed sub-functions

### 6. Test file organization

- One `_test.go` file per source file (standard Go convention)
- Mock/stub types defined in `testutil_test.go` within each package (not exported, not shared across packages)
- Table-driven tests where a function has > 3 test cases
- Test names follow `Test<FunctionName>_<Scenario>` convention

## Risks / Trade-offs

### Refactoring risk to existing behavior

Decomposing functions risks introducing bugs. Mitigations:
- Existing tests provide a regression safety net (all existing tests must continue to pass)
- Decomposition is purely structural — extract, don't rewrite
- Each decomposition is a separate task so it can be reviewed independently

### Test maintenance burden

Adding ~100+ new tests increases maintenance cost. Mitigations:
- Tests use table-driven patterns to minimize boilerplate
- Mock implementations are minimal (only implement the methods actually called)
- Tests assert on contracts (what), not implementation (how), reducing brittleness

### SSA panic blocking accurate measurement

Gaze SSA construction panics on 3 packages (`internal/cli`, `pkg/chrome`, `pkg/exporter`), preventing accurate quality/classification analysis locally. The CI pipeline may behave differently (Go 1.25 on Ubuntu). Mitigations:
- Focus on line coverage and CRAP scores which don't require SSA
- Run the full CI pipeline after changes to verify gate passage
- The contract coverage gate is measured by the CI's `gaze report` command, which may have different SSA behavior

### Diminishing returns near the threshold

The last few CRAPload entries (those with CRAP 15–20) are the cheapest to fix but provide the smallest safety margin. Trade-off accepted: aim for CRAPload ≤ 8 to provide a buffer, not exactly 10.
