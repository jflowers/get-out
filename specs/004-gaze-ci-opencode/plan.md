# Implementation Plan: Add Gaze Quality Analysis to GitHub CI via OpenCode and Zen

**Branch**: `004-gaze-ci-opencode` | **Date**: 2026-03-12 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/004-gaze-ci-opencode/spec.md`

## Summary

Add three steps to the existing `build-and-test` job in `.github/workflows/ci.yml`: install gaze, install OpenCode CLI, and run `gaze report` with the OpenCode adapter pointing at the Zen-hosted Claude Sonnet 4.6 model. The existing `Test` step gains a `-coverprofile=coverage.out` flag so the coverage profile is reused by gaze rather than regenerated. Hard quality gates (CRAPload, GazeCRAPload, contract coverage) block PRs on breach. A formatted report is automatically written to the GitHub Actions Step Summary on every run. No Go source files are created or modified.

## Technical Context

**Language/Version**: Go 1.25 (existing; no change)  
**Primary Dependencies**: `github.com/unbound-force/gaze/cmd/gaze@latest` (external tool, installed via `go install`); `opencode-ai` npm package (external tool, installed via `npm install -g`)  
**Storage**: N/A — no persistent storage; `coverage.out` is an ephemeral workspace file  
**Testing**: No new Go tests; CI workflow YAML is the implementation artifact  
**Target Platform**: GitHub Actions `ubuntu-latest` runner  
**Project Type**: CLI (existing `get-out` CLI tool; this feature adds CI tooling only)  
**Performance Goals**: No second `go test` run; CI overhead limited to tool install time + AI formatting call  
**Constraints**: Single workflow file change; all thresholds configurable without source code changes; `OPENCODE_API_KEY` secret already provisioned  
**Scale/Scope**: Runs on every push to any branch and every PR targeting `main`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Article | Requirement | Status | Notes |
|---|---|---|---|
| I. Session-Driven Extraction | No OAuth tokens or API keys manually configured when browser session exists | ✅ PASS | Feature uses `OPENCODE_API_KEY` secret (CI context, not user session). Constitution Article I applies to Slack extraction UX, not CI tooling. N/A. |
| II. Go-First Architecture | Core functionality in Go; minimize external dependencies | ✅ PASS | No new Go dependencies added to `go.mod`. External tools (gaze, opencode) are CI-only, not runtime dependencies of the CLI. |
| III. Stealth & Reliability | Browser automation via CDP; natural headers; rate limit handling | ✅ PASS | N/A — this feature adds no browser automation. |
| IV. Two-Tier Extraction | API mimicry preferred, DOM fallback | ✅ PASS | N/A — no extraction logic. |
| V. Concurrency & Resilience | errgroup, checkpoint system, rate limit handling | ✅ PASS | N/A — no new concurrent processes. |
| VI. Security First | No hardcoded tokens; secrets via env or keychain | ✅ PASS | `OPENCODE_API_KEY` provided via GitHub Actions secret store, never hardcoded. |
| VII. Output Format | Messages uploaded as Google Docs | ✅ PASS | N/A — this feature produces a CI Step Summary, not a Slack export. |
| VIII. Google Drive Integration | Drive API for uploads | ✅ PASS | N/A — no Drive interaction. |
| IX. Documentation Maintenance | README.md updated for all user-facing changes; AGENTS.md if structure changes | ⚠️ REQUIRED | README.md must document the quality gate, `OPENCODE_API_KEY` secret requirement, and how to interpret a gate failure. AGENTS.md needs no changes (project structure unchanged). |

**Constitution verdict**: PASS with one documentation obligation (Article IX). No gate violations. No Complexity Tracking required.

## Project Structure

### Documentation (this feature)

```text
specs/004-gaze-ci-opencode/
├── plan.md              # This file
├── research.md          # Phase 0 output — all decisions resolved
├── data-model.md        # Phase 1 output — no persistent entities; ephemeral artifacts documented
├── contracts/
│   └── ci-workflow.md   # Phase 1 output — step input/output/exit code contract
└── tasks.md             # Phase 2 output (created by /speckit.tasks, not this command)
```

### Source Code (repository root)

```text
.github/
└── workflows/
    └── ci.yml           # MODIFIED — add -coverprofile to Test step; add 3 new steps

README.md                # MODIFIED — document quality gate, OPENCODE_API_KEY, gate failure guidance
```

**Structure Decision**: Single-file change pattern. This feature touches exactly two files: the CI workflow and the README. No new directories, no new Go source files, no new configuration files.

## Implementation Notes

### Exact diff for `.github/workflows/ci.yml`

The `Test` step gains one flag:
```yaml
      - name: Test
        run: go test -race -v -count=1 -coverprofile=coverage.out ./...
```

Three steps are appended after `Test`:
```yaml
      - name: Install gaze
        run: go install github.com/unbound-force/gaze/cmd/gaze@latest

      - name: Install OpenCode
        run: npm install -g opencode-ai

      - name: Gaze quality report
        env:
          OPENCODE_API_KEY: ${{ secrets.OPENCODE_API_KEY }}
        run: |
          gaze report ./... \
            --ai=opencode \
            --model=opencode/claude-sonnet-4-6 \
            --coverprofile=coverage.out \
            --max-crapload=10 \
            --max-gaze-crapload=5 \
            --min-contract-coverage=50
```

### Why no explicit `GITHUB_STEP_SUMMARY` env var

GitHub Actions sets `GITHUB_STEP_SUMMARY` automatically as an absolute path. `gaze report` reads it from the environment and appends the formatted report to it. No manual piping (`>> $GITHUB_STEP_SUMMARY`) is needed.

### Why no `continue-on-error: true`

A quality gate breach should block the PR. Default GitHub Actions behavior (job fails on non-zero exit) provides this without additional configuration.

### Why no step-level `if:` conditions

Steps within a job already skip automatically when any prior step fails. Placing gaze steps after Build, Vet, Test, and Install satisfies FR-009 by default.

### README.md additions required

The README must gain a CI section (or extend the existing CI section) documenting:
- The quality gate check that runs on every PR
- The `OPENCODE_API_KEY` GitHub Actions secret required for the report step
- Current threshold values and how to change them (edit workflow YAML)
- How to read the Step Summary report when a gate fails
