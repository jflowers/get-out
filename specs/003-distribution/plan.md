# Implementation Plan: Distribution, Config Migration & Self-Service Commands

**Branch**: `003-distribution` | **Date**: 2026-03-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-distribution/spec.md`

## Summary

Move the default configuration directory from `~/.config/get-out` to `~/.get-out`, add self-service commands (`init`, `doctor`, `auth login`, `auth status`, `setup-browser`), replace the `test` command with `setup-browser`, publish Apple-signed and notarized macOS binaries via a GitHub Actions signing pipeline, and distribute the tool via a Homebrew cask in the `jflowers/homebrew-tools` tap. New dependencies: `charmbracelet/huh` (interactive prompts) and `charmbracelet/lipgloss` (styled terminal output).

## Technical Context

**Language/Version**: Go 1.25.0
**Primary Dependencies**: Chromedp (CDP), cobra v1.10.2 (CLI), charmbracelet/huh (interactive prompts), charmbracelet/lipgloss (styled output), Google Drive API v3, Google Docs API v1, golang.org/x/oauth2
**Storage**: JSON files in `~/.get-out/` (config, token, export index) — no database
**Testing**: `go test -race -count=1 ./...`
**Target Platform**: macOS (arm64 + amd64), Linux (arm64 + amd64) — single static binary (CGO_ENABLED=0)
**Project Type**: CLI
**Performance Goals**: `get-out auth status` completes (including silent token refresh) in under 3 seconds; `get-out doctor` completes all 10 checks in under 10 seconds on a machine with an active network connection
**Constraints**: Single static binary; no CGO; no daemon/service process; credential files (`credentials.json`, `token.json`) written with mode 0600; config dir created with mode 0700
**Scale/Scope**: Single-user CLI tool; no concurrent users; all state in local files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Session-Driven Extraction | PASS | `setup-browser` guides users to extract the active browser session; no manual token config required |
| II. Go-First Architecture | PASS | All new commands implemented in Go; charmbracelet deps are pure Go |
| III. Stealth & Reliability | PASS | No changes to CDP/Chromedp usage; `setup-browser` validates the existing session |
| IV. Two-Tier Extraction | PASS | No changes to extraction strategy; `setup-browser` replaces `test` with a guided validation of Tier 1 |
| V. Concurrency & Resilience | PASS | No changes to export pipeline; `doctor` and `init` are sequential by design |
| VI. Security First | PASS | `credentials.json` and `token.json` written with mode 0600; config dir 0700; credentials masked in `setup-browser` output; no tokens hardcoded |
| VII. Output Format | PASS | No changes to export output format |
| VIII. Google Drive Integration | PASS | `auth login`/`auth status` improve OAuth UX; no changes to Drive API usage |
| IX. Documentation Maintenance | PASS | README.md, AGENTS.md, and man page all updated as part of this feature |

**Constitution Check Result: ALL PASS — proceed to Phase 0.**

> **Complexity Tracking**: No constitution violations requiring justification.

## Project Structure

### Documentation (this feature)

```text
specs/003-distribution/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── cli-schema.md
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code (repository root)

```text
cmd/get-out/
└── main.go                        # unchanged (ldflags for version/commit/date)

internal/cli/
├── root.go                        # EDIT: defaultConfigDir() → ~/.get-out
├── auth.go                        # REFACTOR: auth group + login + status sub-commands
├── export.go                      # EDIT: read folder_id from settings.json as default
├── selfservice.go                 # NEW: init, doctor, setup-browser commands
├── helpers.go                     # NEW: safePreview() + shared formatting helpers
├── test.go                        # DELETE: replaced by setup-browser
├── export_test.go                 # EDIT: safePreview reference (same package, no import change)
└── selfservice_test.go            # NEW: unit tests for pure helper functions

pkg/config/
├── config.go                      # EDIT: add DefaultConfigDir() exported helper
└── types.go                       # EDIT: add FolderID field to Settings struct

man/
└── get-out.1                      # NEW: troff man page

.goreleaser.yml                    # EDIT: add checksum, homebrew_casks, man page in archives
.github/workflows/release.yml      # REWRITE: add sign-macos job + HOMEBREW_TAP_TOKEN env
README.md                          # EDIT: Homebrew install section, updated Quick Start
AGENTS.md                          # EDIT: config dir, commands, new deps
go.mod / go.sum                    # EDIT: add charmbracelet/huh, charmbracelet/lipgloss
```

**Structure Decision**: Single-project CLI (Option 1). All new command logic goes in `internal/cli/` following existing cobra patterns (`RunE`, `init()` registration). A new `selfservice.go` file groups `init`, `doctor`, and `setup-browser` together, mirroring gcal-organizer's `selfservice.go` layout. A new `helpers.go` extracts shared utilities (`safePreview`) that are currently in `test.go` to prevent compilation breakage when `test.go` is deleted.
