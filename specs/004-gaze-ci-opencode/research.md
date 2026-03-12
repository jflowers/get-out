# Research: Add Gaze Quality Analysis to GitHub CI via OpenCode and Zen

**Feature**: 004-gaze-ci-opencode  
**Date**: 2026-03-12  
**Status**: Complete — no NEEDS CLARIFICATION items remain

## Summary

All decisions for this feature were pre-resolved through direct examination of the gaze source code and OpenCode documentation during the specification phase. No unknowns required external research agents. This document records the key decisions and their rationale.

---

## Decision 1: AI Adapter — `--ai=opencode`

**Decision**: Use `gaze report --ai=opencode` (not `--ai=claude` or `--ai=gemini`)

**Rationale**: The `opencode` adapter was merged to gaze `main` in commit `3c7d430`. It shells out to the `opencode` CLI binary (installed via npm) rather than requiring a raw Anthropic or Gemini API key. This allows the CI secret to be a single `OPENCODE_API_KEY` (Zen) rather than a provider-specific key, which aligns with FR-006 (use Zen model tier, no direct provider keys).

**Mechanism**: `gaze report --ai=opencode` writes the `gaze-reporter.md` agent prompt to a temp directory under `.opencode/agents/`, then invokes `opencode run --dir <tmpDir> --agent gaze-reporter --format default -m <model> ""` with the analysis JSON on stdin. The temp directory is cleaned up on exit.

**Alternatives considered**:
- `--ai=claude`: Requires `ANTHROPIC_API_KEY` and the `claude` CLI binary (npm). Two dependencies, direct provider billing.
- `--ai=gemini`: Requires `GEMINI_API_KEY` and the `gemini` CLI binary. Same drawback.
- `--ai=ollama`: No API key, but requires a running Ollama server. Not viable on ephemeral CI runners.

---

## Decision 2: Model — `opencode/claude-sonnet-4-6`

**Decision**: Use `--model=opencode/claude-sonnet-4-6`

**Rationale**: Claude Sonnet 4.6 is available on the OpenCode Zen tier, proven for coding tasks, and cost-effective relative to Opus. The `opencode/` prefix is the Zen provider format. The gaze adapter passes the model string verbatim as `-m <value>` to the `opencode run` subprocess.

**Alternatives considered**:
- `opencode/claude-opus-4-6`: Higher quality but significantly higher cost per token for a CI gate that runs on every PR.
- `opencode/claude-haiku-4-5`: Cheaper but lower reasoning quality for complex CRAP + contract coverage interpretation.

---

## Decision 3: Coverage Profile Reuse — `--coverprofile=coverage.out`

**Decision**: Add `-coverprofile=coverage.out` to the existing `Test` step and pass `--coverprofile=coverage.out` to `gaze report`.

**Rationale**: `gaze report` (spec 020, commit `63a9f1e`) now accepts `--coverprofile` to skip internal `go test` re-execution. Without this flag, `gaze report` spawns its own `go test -coverprofile` run internally, causing the full test suite to run twice. Reusing the coverage file satisfies FR-004 and SC-003.

**Mechanism**: The coverage file is written to `coverage.out` in the workspace root by the `Test` step. It persists in the GitHub Actions job workspace for subsequent steps. The `gaze report` step reads it directly.

**Alternatives considered**:
- Upload/download artifact between steps: Unnecessary complexity — steps within the same job share a workspace.
- Let `gaze report` generate its own coverage: Causes double test execution, violating SC-003.

---

## Decision 4: OpenCode CLI Installation — `npm install -g opencode-ai`

**Decision**: Install via npm (`npm install -g opencode-ai`) in a dedicated CI step before the gaze step.

**Rationale**: npm is pre-installed on all `ubuntu-latest` GitHub Actions runners. The `opencode-ai` npm package is the official distribution channel. This puts the `opencode` binary on `$PATH` for the gaze adapter's `exec.LookPath("opencode")` call.

**Alternatives considered**:
- Homebrew: Available on ubuntu-latest but slower than npm for a single binary.
- Direct binary download from GitHub releases: Requires version pinning and SHA verification; adds maintenance overhead.
- Build from source: Prohibitively slow for CI.

---

## Decision 5: gaze Installation — `go install github.com/unbound-force/gaze/cmd/gaze@latest`

**Decision**: Install gaze via `go install ... @latest` after Go is already set up in CI.

**Rationale**: Go is already set up by the `Set up Go` step. `go install` is the standard mechanism for installing Go tools. `@latest` tracks the current released version including specs 019 (opencode adapter) and 020 (coverprofile flag), which are already merged to `main`.

**Alternatives considered**:
- Pin to a specific commit SHA: More reproducible but adds maintenance burden when new versions are released.
- Vendor gaze into the repository: Violates FR-002 (must not require committing the tool).

---

## Decision 6: Hard Gate Thresholds (Initial Values)

**Decision**: `--max-crapload=10`, `--max-gaze-crapload=5`, `--min-contract-coverage=50`

**Rationale**: These are informed starting values for a codebase with no prior gaze baseline. They are intentionally permissive to avoid immediately failing CI on the first run, while still enforcing a meaningful floor. All three flags are required — omitting any threshold flag disables that gate entirely (gaze's nil-pointer gate design). The values are set in the workflow YAML (FR-010: configurable without modifying tool source).

**Alternatives considered**:
- Report-only (no threshold flags): Would satisfy observability (SC-002) but not enforcement (SC-001, FR-007). Rejected per user decision.
- Tighter thresholds (e.g., max-crapload=5): Risk of immediately failing CI on the first run before the baseline is known.

---

## Decision 7: Step Summary — No Explicit Piping Required

**Decision**: No `>> $GITHUB_STEP_SUMMARY` piping needed in the workflow step.

**Rationale**: `gaze report` reads `GITHUB_STEP_SUMMARY` from the environment (set automatically by GitHub Actions) and writes the formatted report to it via `O_APPEND` in `internal/aireport/output.go`. This is a built-in behavior activated by the presence of the env var. Write failures are non-fatal (the command does not exit non-zero if Step Summary writing fails).

---

## Decision 8: Step Ordering and Failure Propagation

**Decision**: gaze steps are appended after the existing `Test` step within the same `build-and-test` job. No `continue-on-error: true`.

**Rationale**: GitHub Actions stops executing steps in a job after any step exits non-zero (default behavior). Placing gaze steps after Build, Vet, and Test means FR-009 is satisfied by the runner's default behavior — no explicit `if:` conditions needed. No `continue-on-error` means a threshold breach blocks the PR (user decision).

---

## Decision 9: No New Go Code Required

**Decision**: This feature is implemented entirely as changes to `.github/workflows/ci.yml`. No Go source files are created or modified.

**Rationale**: The feature integrates two external tools (gaze, opencode) that already implement all required logic. The CI workflow is the sole integration surface. This minimizes the blast radius and keeps the change reviewable as a single YAML diff.

---

## Decision 10: README Update Required

**Decision**: `README.md` must be updated to document the quality gate CI behavior.

**Rationale**: Constitution Article IX (Documentation Maintenance) requires that any change to CI behavior, commands, or configuration be reflected in README.md before the feature is considered complete. The README must describe the quality gates, the `OPENCODE_API_KEY` secret requirement, and how to interpret a gate failure.
