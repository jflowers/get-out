# Contract: CI Workflow — Gaze Quality Gate

**Feature**: 004-gaze-ci-opencode  
**Date**: 2026-03-12  
**Contract Type**: CI Pipeline Schema (GitHub Actions workflow step contract)

## Overview

This contract describes the observable interface of the gaze quality gate as it appears to developers interacting with the CI system. It covers inputs (configuration), outputs (exit codes, Step Summary), and the behavior contract for each outcome.

---

## Step Contract: `Install gaze`

**Trigger**: Runs after `Test` step succeeds.

| Property | Value |
|---|---|
| Command | `go install github.com/unbound-force/gaze/cmd/gaze@latest` |
| Success condition | `gaze` binary available on `$PATH`, exit 0 |
| Failure condition | Network failure or version unavailable, exit non-zero |
| Effect on subsequent steps | Failure prevents `Install OpenCode` and `Gaze quality report` from running |

---

## Step Contract: `Install OpenCode`

**Trigger**: Runs after `Install gaze` succeeds.

| Property | Value |
|---|---|
| Command | `npm install -g opencode-ai` |
| Success condition | `opencode` binary available on `$PATH`, exit 0 |
| Failure condition | npm registry unavailable or package not found, exit non-zero |
| Effect on subsequent steps | Failure prevents `Gaze quality report` from running |

---

## Step Contract: `Gaze quality report`

**Trigger**: Runs after `Install OpenCode` succeeds. Requires `coverage.out` present in workspace root (produced by the `Test` step).

### Inputs

| Input | Source | Description |
|---|---|---|
| `OPENCODE_API_KEY` | GitHub Actions secret | Zen authentication key. Must be non-empty; opencode fails if absent. |
| `coverage.out` | Workspace file (from `Test` step) | Go coverage profile. Must be a valid profile for the current module. |
| `--ai=opencode` | Hardcoded in workflow | Selects the OpenCode adapter. |
| `--model=opencode/claude-sonnet-4-6` | Hardcoded in workflow | Zen model. Configurable by editing the workflow YAML. |
| `--coverprofile=coverage.out` | Hardcoded in workflow | Points gaze at the pre-generated coverage file. |
| `--max-crapload=10` | Hardcoded in workflow | Gate: fail if more than 10 functions exceed CRAP threshold. |
| `--max-gaze-crapload=5` | Hardcoded in workflow | Gate: fail if more than 5 functions exceed GazeCRAP threshold. |
| `--min-contract-coverage=50` | Hardcoded in workflow | Gate: fail if average contract coverage is below 50%. |

### Outputs

| Output | Condition | Description |
|---|---|---|
| Exit 0 | All gates pass | No further action; PR check passes. |
| Exit non-zero | Any gate breached, or tool/AI failure | PR check fails; merge blocked. |
| GitHub Step Summary entry | Always (when `GITHUB_STEP_SUMMARY` env var is set) | Formatted markdown quality report appended. Write failure is non-fatal. |
| stderr gate summary | When any gate is evaluated | One-line summary per gate: e.g. `CRAPload: 3/10 (PASS)` |

### Exit Code Semantics

| Exit Code | Meaning |
|---|---|
| `0` | All configured gates pass. Report may still surface warnings. |
| `1` | One or more gates breached (threshold exceeded). |
| `1` | `opencode` binary not found on `$PATH`. |
| `1` | `coverage.out` file missing, malformed, or is a directory. |
| `1` | `opencode` subprocess exited non-zero (API key invalid, model unavailable, rate limit). |
| `1` | `opencode` subprocess returned empty output. |

### Threshold Configuration

All thresholds are set in the workflow YAML. Changing them requires no code changes to `get-out` source or gaze source. To disable a specific gate, remove that flag from the `run:` block (omitting a flag entirely disables that gate; setting it to `0` enables a zero-tolerance gate).

| Flag | Disable gate by | Enable zero-tolerance by |
|---|---|---|
| `--max-crapload` | Remove flag | `--max-crapload=0` |
| `--max-gaze-crapload` | Remove flag | `--max-gaze-crapload=0` |
| `--min-contract-coverage` | Remove flag | `--min-contract-coverage=0` |

---

## PR Check Behavior

| Scenario | PR Check Status |
|---|---|
| All gates pass, report written | ✅ Pass |
| Any gate breached | ❌ Fail (blocked) |
| `OPENCODE_API_KEY` missing/invalid | ❌ Fail (blocked) |
| `coverage.out` missing or malformed | ❌ Fail (blocked) |
| AI provider rate-limited or unavailable | ❌ Fail (blocked) |
| Earlier step (Build/Vet/Test/Install) failed | Step skipped (job fails on earlier step) |
