## Context

Phases 1-3 added 71 test functions and raised line coverage from 82% to 88.5%, Q1 from 57 to 70. Contract coverage percentages didn't budge because gaze's mechanical classifier determines confidence scores from code-structural signals, not test assertion patterns. The key missing signal is **godoc documentation** that explicitly describes return contracts.

All 24 Q3 functions have godoc that describes purpose ("loads users", "finds a document") but omits return contracts ("returns nil if not found", "returns (nil, error) on failure"). Gaze's classifier rewards documentation that matches return-value patterns because it provides strong evidence that a side effect is contractual (intentionally promised to callers) rather than incidental (implementation detail).

## Goals / Non-Goals

### Goals
- Add explicit return-contract, error-contract, and mutation-description godoc to all 24 Q3 functions
- Push each function's mechanical confidence above 80 (the contractual threshold)
- Reduce Q3 count from 24 to ≤ 5
- Raise per-package contract coverage above 50% for `slackapi`, `parser`, and `gdrive`
- Follow Go documentation conventions (comment starts with function name, describes behavior not implementation)

### Non-Goals
- Adding tests (phases 1-3 already cover these functions thoroughly)
- Changing function signatures or behavior
- Fixing the `getTokenFromWeb` CRAP score (untestable by design)
- Documenting internal/unexported helper functions
- Resolving SSA panics for `internal/cli`, `pkg/chrome`, or `pkg/exporter`

## Decisions

### 1. Replace, don't append

Current godoc comments are typically one-line descriptions. Rather than appending a second paragraph to a one-liner (which reads awkwardly), each function gets a fully rewritten godoc comment that:
- Starts with the function name (Go convention)
- Opens with a concise purpose statement (preserves the original meaning)
- Follows with return-contract paragraphs
- Closes with error behavior

### 2. Five contract signal types to document

Each godoc replacement includes as many of these signal types as apply:

| Signal Type | Example Pattern | When to Include |
|-------------|----------------|-----------------|
| Return-nil contract | "Returns (nil, error) if..." | Functions returning (T, error) |
| Return-non-nil contract | "Returns a non-nil *Client on success" | Functions returning pointers |
| Return-nil-nil contract | "Returns (nil, nil) if not found" | Functions where nil is a valid success return |
| Mutation description | "Mutates the receiver by populating r.users" | Methods that modify internal state |
| Callback contract | "The callback is invoked once per batch" | Functions taking func parameters |

### 3. Package-by-package order: slackapi → parser → gdrive → config

Start with `slackapi` (8 Q3 functions, most diverse signal types), then `parser` (6 functions with mutation patterns), then `gdrive` (9 functions with the critical `FindDocument (nil, nil)` contract), and finally `config` (1 function).

### 4. Verify with gaze after each package

After updating each package's godoc, run `gaze crap --format=json ./pkg/<package>` to verify:
- Function confidence scores increased above 80
- Q3 count decreased
- Contract coverage percentage increased

If a function's confidence doesn't cross 80, the godoc may need additional signal-bearing phrases.

### 5. Document only what the code actually does

Every godoc claim must be verifiable against the implementation. No aspirational documentation. If the function has a bug (e.g., doesn't handle nil correctly), document the actual behavior, not the ideal behavior. The tests already verify the behavior exists.

## Risks / Trade-offs

### Godoc accuracy

Enhanced godoc describes contracts that the code may not perfectly implement. If a godoc says "returns (nil, error) on failure" but a code path actually panics, the documentation is misleading.

**Mitigation**: Every contract statement was verified against the function body during the analysis phase. The 71 tests from phases 1-3 independently verify these behaviors.

### Godoc maintenance burden

Detailed godoc comments must be updated when function behavior changes. A future change to `FindDocument` that returns an empty `*DocInfo` instead of `nil` would require updating the godoc.

**Trade-off accepted**: This is the standard Go approach. The godoc IS the contract specification. Keeping it accurate is part of the development workflow.

### Confidence model uncertainty

Gaze's confidence scoring is a heuristic. Adding godoc signals may not push every function above 80 if the model weights documentation differently than expected.

**Mitigation**: Run gaze per-package after each group to measure impact. If specific functions don't respond, investigate whether additional signals (caller count, effect type) are the binding constraint.
