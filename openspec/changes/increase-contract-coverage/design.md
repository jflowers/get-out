## Context

The gaze quality report identifies 24 functions in the Q3 "Needs Tests" quadrant across `pkg/parser`, `pkg/slackapi`, and `pkg/gdrive`. These functions have adequate line coverage (82–100%) but their side effects are classified as "ambiguous" because existing tests verify that functions execute without errors but don't assert on the values returned, the parameters forwarded, or the state mutations performed. The CI pipeline's `--min-contract-coverage 50%` gate needs per-package averages above threshold.

This is a test-only change. No production code is modified.

## Goals / Non-Goals

### Goals
- Increase per-package contract coverage above 50% for `parser`, `slackapi`, and `gdrive`
- Reduce Q3 function count from 24 to ≤ 8 across all packages
- Add assertions that verify each function's design responsibilities, not just its execution path
- Cover untested branches that represent real contractual behavior (image annotations, formatting variants, size caps, parameter forwarding)
- Follow existing test patterns (httptest.Server, table-driven tests, white-box testing in the same package)

### Non-Goals
- Achieving 100% contract coverage (some functions like `getTokenFromWeb` require network interaction and are untestable by design)
- Refactoring production code to improve testability (that's a separate change)
- Adding integration tests or end-to-end tests
- Fixing the SSA panics that prevent `internal/cli` and `pkg/chrome` from being analyzed (separate toolchain issue)
- Increasing line coverage (already adequate at 82%+; the gap is assertion quality, not path coverage)

## Decisions

### 1. Assert return value properties, not just error status

The primary pattern in existing tests is:

```go
if err != nil {
    t.Fatalf("unexpected error: %v", err)
}
```

This verifies the function didn't fail but doesn't verify what it returned. For contract coverage, assertions must verify the structure and content of return values. For example, `ListConversations` returns a `*ConversationsListResponse` — tests must assert on the `Channels` slice length and field values, not just `err == nil`.

Each new test case will include explicit assertions on return value fields, following the `// Contract assertion:` comment convention already used in `pkg/parser` tests.

### 2. Use HTTP request inspection for parameter forwarding

Several Q3 functions have ambiguous side effects because their parameters are forwarded to underlying HTTP calls but no test verifies the forwarding. The existing `httptest.Server` pattern in both `slackapi` and `gdrive` test suites supports handler-level inspection of request bodies and query parameters.

For each parameter forwarding contract:
1. The test handler captures the incoming HTTP request parameters
2. After the function returns, the test asserts that the expected parameter values appeared in the captured request

This pattern is already used in `slackapi/client_test.go` for `doRequest` mode detection and in `gdrive/gdrive_test.go` for query string validation.

### 3. Add new test functions rather than modifying existing ones

New test cases are added as separate test functions (not inserted into existing table-driven tests) to keep the diff reviewable and avoid accidental regressions in passing tests. The naming convention follows the existing pattern: `Test<Function>_<Scenario>`.

Exception: where an existing test function already tests the same function and adding a sub-case is more natural (e.g., adding an italic row to an existing formatting table test), the existing test is extended.

### 4. Package-by-package implementation order

Work proceeds in this order, based on gap severity and the number of Q3 functions:

1. **`pkg/gdrive`** (9 Q3 functions, 20% contract coverage) — largest gap, highest impact
2. **`pkg/slackapi`** (8 Q3 functions, 26.9% contract coverage) — second largest
3. **`pkg/parser`** (6 Q3 functions, 26.1% contract coverage) — well-tested already; gaps are smaller

Each package is completed and verified independently before moving to the next, consistent with the Composability First principle.

### 5. Verify with per-package gaze runs

After each package is complete, verify improvement with:
```bash
gaze crap --format=json ./pkg/<package>
```

The expected signal is:
- `avg_contract_coverage` increases above 50%
- `quadrant_counts.Q3_SimpleButUnderspecified` decreases
- `gaze_crapload` decreases

### 6. Scope exclusions for untestable code

The following functions are excluded from contract coverage improvement because they require real external services:

- `gdrive.getTokenFromWeb` — requires browser-based OAuth flow (CRAP 182, but untestable by design)
- `gdrive.NewClient` / `gdrive.NewClientFromStore` — require real Google API credentials
- Network-dependent token refresh paths in `AuthenticateWithStore` and `EnsureTokenFreshWithStore`

These exclusions align with the constitution's Testability principle (§IV): "Every component MUST be testable in isolation without requiring external services." Functions that inherently require external services are exempt.

## Risks / Trade-offs

### Test maintenance burden

Adding 30+ new test assertions increases the maintenance surface. If production code changes the return value structure or error message wording, tests will fail.

**Trade-off accepted**: This is the point. The tests should fail when contractual behavior changes — that's what contract coverage means. The assertions document the design contract and protect against unintentional drift.

### Flaky test risk from HTTP request inspection

Tests that inspect HTTP request bodies (parameter forwarding assertions) are sensitive to serialization order and encoding details.

**Mitigation**: Use `r.FormValue()` for form-encoded parameters (stable) and `json.Unmarshal` into maps for JSON bodies (order-independent). Both patterns are already in use in the existing test suite.

### Q3 functions may remain Q3 if gaze classifies their effects as ambiguous

Gaze's mechanical classification may keep some side effects as "ambiguous" even after adding assertions, because the confidence score depends on signals like visibility, caller count, and godoc — not on test assertions alone. Contract coverage only improves for effects that gaze classifies as contractual.

**Trade-off accepted**: The assertions are valuable regardless of gaze's classification. If a side effect is truly contractual but gaze scores it as ambiguous, the document-enhanced scoring in full-mode analysis (using project README and specs) can promote it. The test assertions are necessary either way.
