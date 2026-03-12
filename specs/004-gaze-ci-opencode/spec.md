# Feature Specification: Add Gaze Quality Analysis to GitHub CI via OpenCode and Zen

**Feature Branch**: `004-gaze-ci-opencode`  
**Created**: 2026-03-12  
**Status**: Draft  
**Input**: User description: "Add gaze to GitHub CI using OpenCode and Zen"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Quality Gate Blocks Failing PRs (Priority: P1)

A developer opens a pull request that introduces a function with high cyclomatic complexity and no meaningful test assertions. The CI pipeline runs the gaze quality analysis step, detects that the function exceeds the configured CRAP threshold, and the PR check fails with a clear summary of what exceeded the gate.

**Why this priority**: The primary value of this feature is preventing quality regressions from being merged. Without hard gates, the feature delivers no enforcement — only observation.

**Independent Test**: Can be fully tested by opening a PR with a deliberately complex, untested function and verifying the CI check fails with a non-zero exit and a Step Summary report.

**Acceptance Scenarios**:

1. **Given** a PR is opened with a function whose CRAP score exceeds the configured threshold, **When** the CI pipeline runs, **Then** the gaze step fails with a non-zero exit code and the PR check is blocked.
2. **Given** a PR is opened with code that meets all quality thresholds, **When** the CI pipeline runs, **Then** the gaze step passes and does not block the PR.
3. **Given** the gaze step fails, **When** a developer views the GitHub Actions run, **Then** the Step Summary tab displays a human-readable quality report explaining which functions or metrics exceeded their thresholds.

---

### User Story 2 - Quality Report Appears in Step Summary (Priority: P2)

A developer merges a PR and later wants to understand the current quality health of the codebase. They navigate to the most recent CI run and open the Step Summary tab to find a formatted report covering CRAP scores, contract coverage, and overall health assessment — without needing to read raw log output.

**Why this priority**: The report is the observability layer. Even when no gate fails, the report gives teams ongoing visibility into quality trends. It delivers value independently of the gate.

**Independent Test**: Can be fully tested by triggering a CI run on any branch and confirming the Step Summary tab contains a formatted markdown quality report with CRAP scores and contract coverage metrics.

**Acceptance Scenarios**:

1. **Given** a CI run completes successfully, **When** a developer opens the GitHub Actions Step Summary tab, **Then** a formatted quality report is present with CRAP scores, GazeCRAP quadrant distribution, and contract coverage metrics.
2. **Given** the quality report is generated, **When** a developer reads it, **Then** the report is written in plain language with clear severity indicators (not raw JSON or log output).

---

### User Story 3 - No Duplicate Test Execution (Priority: P3)

A developer reviewing CI run times notices that tests are not run twice despite both a standard test step and a quality analysis step being present in the pipeline.

**Why this priority**: Efficiency matters but does not affect correctness. This story ensures the implementation avoids a known anti-pattern (running `go test` twice) but is independently verifiable and lower stakes than the gate and report stories.

**Independent Test**: Can be fully tested by inspecting CI run logs and confirming `go test` is invoked exactly once, with the coverage output reused by the quality analysis step.

**Acceptance Scenarios**:

1. **Given** the CI pipeline runs, **When** the logs are inspected, **Then** the test suite is executed exactly once and the coverage data produced by that run is reused by the quality analysis step.
2. **Given** a pre-generated coverage file is passed to the quality analysis step, **When** the step runs, **Then** no additional test execution occurs within the quality analysis step.

---

### Edge Cases

- What happens when the `OPENCODE_API_KEY` secret is missing or expired? The quality analysis step must fail with a clear, actionable error message rather than silently producing an empty report.
- What happens when the gaze tool is not available at the version required? The install step must fail explicitly before the analysis step runs.
- What happens when the AI provider is unavailable or rate-limited during the analysis? The step must fail non-zero so the PR is not silently passed without a report.
- What happens when all quality thresholds are set to zero (disabled)? The step must still produce a report but exit zero, not block the PR.
- What happens when the coverage file is malformed or from a different module? The quality analysis step must fail with a clear error before attempting AI formatting.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CI pipeline MUST run the test suite with coverage data collection enabled so that a coverage profile is available for quality analysis.
- **FR-002**: The CI pipeline MUST install the gaze quality analysis tool as part of the CI run without requiring it to be committed to the repository.
- **FR-003**: The CI pipeline MUST install the OpenCode CLI as part of the CI run without requiring it to be committed to the repository.
- **FR-004**: The quality analysis step MUST use the coverage profile generated by the test step rather than re-running the test suite internally.
- **FR-005**: The quality analysis step MUST use OpenCode as the AI backend for report formatting, authenticated via a repository secret.
- **FR-006**: The quality analysis step MUST use the OpenCode Zen model tier for AI inference, avoiding direct provider API keys.
- **FR-007**: The CI pipeline MUST enforce hard quality gates: the step MUST exit non-zero and block the PR if the CRAPload, GazeCRAPload, or average contract coverage breaches its configured threshold.
- **FR-008**: The quality analysis step MUST automatically write a formatted human-readable report to the GitHub Actions Step Summary tab when running in a GitHub Actions environment.
- **FR-009**: The quality analysis step MUST NOT run if any earlier step (build, vet, test, install) has failed — it depends on a clean build and a valid coverage profile.
- **FR-010**: All threshold values MUST be configurable in the workflow definition without modifying tool source code.

### Assumptions

- The `OPENCODE_API_KEY` secret has already been added to the repository's GitHub Actions secrets.
- Initial threshold values are: CRAPload ≤ 10, GazeCRAPload ≤ 5, minimum contract coverage ≥ 50%.
- The gaze tool and OpenCode CLI are installed fresh on each CI run (no caching required for the initial implementation).
- The OpenCode Zen model used is `opencode/claude-sonnet-4-6`.
- The CI runner is Ubuntu (Linux), consistent with the existing workflow.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every pull request targeting `main` receives a quality gate check; PRs that breach any configured threshold are blocked from merging without manual override.
- **SC-002**: A formatted quality report appears in the GitHub Actions Step Summary tab for every CI run that reaches the quality analysis step, readable without inspecting raw logs.
- **SC-003**: The test suite is executed exactly once per CI run; total CI duration increases by no more than the time required to install gaze and OpenCode plus the AI formatting call (not a second full test run).
- **SC-004**: A developer can understand why a quality gate failed — including which function(s) or metric(s) caused it — by reading only the Step Summary report, without downloading artifacts or parsing logs.
- **SC-005**: The quality gate configuration (thresholds and model) can be changed by editing a single workflow file with no changes required to any other file in the repository.
