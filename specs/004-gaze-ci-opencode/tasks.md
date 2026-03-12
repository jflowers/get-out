# Tasks: Add Gaze Quality Analysis to GitHub CI via OpenCode and Zen

**Input**: Design documents from `/specs/004-gaze-ci-opencode/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ci-workflow.md ✅

**Tests**: Not requested — no test tasks generated.

**Organization**: Tasks are grouped by user story. This feature touches exactly two files:
- `.github/workflows/ci.yml` — CI workflow
- `README.md` — documentation (Constitution Article IX obligation)

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm the working branch and existing CI baseline are correct before making changes.

- [x] T001 Verify branch `004-gaze-ci-opencode` is checked out and `git status` is clean
- [x] T002 Read `.github/workflows/ci.yml` and confirm current `Test` step command matches `go test -race -v -count=1 ./...` (no `-coverprofile` flag yet)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: No shared infrastructure changes are needed — this feature has no database, no new Go packages, and no shared configuration files. Phase 2 is a no-op for this feature.

**Checkpoint**: Phase 1 verified — user story implementation can begin immediately.

---

## Phase 3: User Story 1 — Quality Gate Blocks Failing PRs (Priority: P1) 🎯 MVP

**Goal**: Hard quality gates are enforced on every PR. A breach exits non-zero and blocks the merge. Three gaze steps are added to `.github/workflows/ci.yml`.

**Independent Test**: Push a PR with a deliberately complex, untested function; confirm the CI check fails with a non-zero exit. Push a PR with clean code; confirm it passes.

### Implementation for User Story 1

- [x] T003 [US1] In `.github/workflows/ci.yml`, add `-coverprofile=coverage.out` flag to the existing `Test` step run command so it becomes `go test -race -v -count=1 -coverprofile=coverage.out ./...`
- [x] T004 [US1] In `.github/workflows/ci.yml`, append step `Install gaze` after the `Test` step: `run: go install github.com/unbound-force/gaze/cmd/gaze@latest`
- [x] T005 [US1] In `.github/workflows/ci.yml`, append step `Install OpenCode` after `Install gaze`: `run: npm install -g opencode-ai`
- [x] T006 [US1] In `.github/workflows/ci.yml`, append step `Gaze quality report` after `Install OpenCode` with `env: OPENCODE_API_KEY: ${{ secrets.OPENCODE_API_KEY }}` and `run:` block containing `gaze report ./... --ai=opencode --model=opencode/claude-sonnet-4-6 --coverprofile=coverage.out --max-crapload=10 --max-gaze-crapload=5 --min-contract-coverage=50`

**Checkpoint**: User Story 1 is complete when `.github/workflows/ci.yml` contains all four changes (T003–T006) and a pushed branch triggers a CI run that reaches the `Gaze quality report` step.

---

## Phase 4: User Story 2 — Quality Report Appears in Step Summary (Priority: P2)

**Goal**: Every CI run that reaches the gaze step produces a formatted quality report in the GitHub Actions Step Summary tab. This is satisfied automatically by `gaze report` reading `GITHUB_STEP_SUMMARY` from the environment — no additional workflow changes are needed beyond what T003–T006 already implement.

**Independent Test**: Trigger any CI run that reaches the `Gaze quality report` step and open the Step Summary tab in the GitHub Actions UI; confirm a formatted markdown report is present.

### Implementation for User Story 2

- [x] T007 [US2] Verify in `.github/workflows/ci.yml` that no explicit `GITHUB_STEP_SUMMARY` env var override exists in the `Gaze quality report` step — it must be absent (GitHub Actions sets it automatically; `gaze report` reads it without any workflow configuration)

**Checkpoint**: User Story 2 is complete when a CI run shows a formatted report in the Step Summary tab. No code changes beyond Phase 3 are required; this is a verification task.

---

## Phase 5: User Story 3 — No Duplicate Test Execution (Priority: P3)

**Goal**: The test suite runs exactly once per CI run. T003 already adds `-coverprofile=coverage.out` to the `Test` step and T006 passes `--coverprofile=coverage.out` to `gaze report`, so gaze skips its internal `go test` run. This story requires verifying the implementation is correct, not adding new steps.

**Independent Test**: Inspect CI run logs and confirm `go test` appears exactly once; the `Gaze quality report` step log must NOT show a `go test` invocation.

### Implementation for User Story 3

- [x] T008 [US3] Confirm in `.github/workflows/ci.yml` that `--coverprofile=coverage.out` is present in the `Gaze quality report` step's `run:` block (this flag was set in T006; this task is an explicit correctness check before the feature is considered done)

**Checkpoint**: User Story 3 is complete when the three tasks above are verified and a CI run log shows `go test` invoked exactly once.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Constitution Article IX documentation obligation and final validation.

- [x] T009 [P] In `README.md`, add or extend a CI section documenting: (1) the quality gate check runs on every PR targeting `main`; (2) `OPENCODE_API_KEY` must be set as a GitHub Actions secret; (3) current threshold values (`--max-crapload=10`, `--max-gaze-crapload=5`, `--min-contract-coverage=50`) and how to change them by editing `.github/workflows/ci.yml`; (4) how to read the Step Summary report when a gate fails
- [x] T010 [P] In `AGENTS.md`, verify the `Recent Changes` section reflects the new tools added by this feature (`gaze@latest` via `go install`, `opencode-ai` via npm) — the `update-agent-context.sh` script ran during planning and updated `Active Technologies`; confirm the entry is accurate

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: No-op for this feature
- **User Story 1 (Phase 3)**: Depends on Phase 1 verification — BLOCKS US2 and US3 verification
- **User Story 2 (Phase 4)**: Depends on Phase 3 completion (T003–T006 must be in place for the Step Summary to be populated)
- **User Story 3 (Phase 5)**: Depends on Phase 3 completion (T006 must be in place for `--coverprofile` to suppress the internal test run)
- **Polish (Phase 6)**: Depends on Phase 3 completion; T009 and T010 can run in parallel with each other

### User Story Dependencies

- **US1 (P1)**: No dependency on US2 or US3 — implements the gate
- **US2 (P2)**: No new implementation beyond US1; depends on US1 being in place to have a run to inspect
- **US3 (P3)**: No new implementation beyond US1; depends on US1 being in place to verify single test execution

### Within Each User Story

- T003 must complete before T004, T005, T006 (all edits are to the same file; sequential to avoid conflicts)
- T004 must complete before T005 (step ordering in the workflow)
- T005 must complete before T006 (step ordering in the workflow)

### Parallel Opportunities

- T009 (README) and T010 (AGENTS.md) in Phase 6 can run in parallel — different files, no dependencies
- US2 and US3 verification tasks (T007, T008) can be done in parallel once Phase 3 is complete

---

## Parallel Example: Phase 6

```bash
# Both documentation tasks can run simultaneously (different files):
Task: "Update README.md with CI quality gate documentation"        # T009
Task: "Verify AGENTS.md Recent Changes entry is accurate"          # T010
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Verify branch and baseline (T001–T002)
2. Complete Phase 3: Implement the four CI changes (T003–T006)
3. **STOP and VALIDATE**: Push branch, confirm CI run reaches `Gaze quality report` step and the Step Summary tab contains a report
4. If CI passes baseline (no gate breach on current code), all three user stories are satisfied simultaneously

### Incremental Delivery

Because US2 and US3 have no new implementation tasks (they are satisfied by the same T003–T006 changes), the entire feature is delivered as a single coherent increment:

1. Phase 1 verification → Phase 3 implementation → Phase 4/5 verification → Phase 6 documentation
2. Single PR with two file changes: `.github/workflows/ci.yml` and `README.md`

---

## Notes

- All four CI changes (T003–T006) must be made to the same file (`.github/workflows/ci.yml`); make them sequentially in a single editing session to avoid conflicts
- The `OPENCODE_API_KEY` secret is already provisioned in the repository (confirmed during spec phase) — no secrets setup task is needed
- T007 and T008 are verification-only tasks; they require no file edits if T003–T006 were implemented correctly
- The Step Summary report (US2) and single-test-run guarantee (US3) are both emergent properties of the US1 implementation — there is no additional code to write for those stories
