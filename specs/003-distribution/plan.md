# Implementation Plan: Distribution, Config Migration & Self-Service Commands

**Branch**: `003-distribution` | **Date**: 2026-03-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-distribution/spec.md`

## Summary

Move the default configuration directory from `~/.config/get-out` to `~/.get-out`, add self-service commands (`init`, `doctor`, `auth login`, `auth status`, `setup-browser`), replace the `test` command with `setup-browser`, add OS keychain credential storage via `pkg/secrets` (matching gcal-organizer), publish Apple-signed and notarized macOS binaries via a GitHub Actions signing pipeline, and distribute the tool via a Homebrew cask in the `jflowers/homebrew-tools` tap. New dependencies: `charmbracelet/huh` (interactive prompts), `charmbracelet/lipgloss` (styled terminal output), `github.com/zalando/go-keyring` (OS credential store).

## Technical Context

**Language/Version**: Go 1.25.0
**Primary Dependencies**: Chromedp (CDP), cobra v1.10.2 (CLI), charmbracelet/huh (interactive prompts), charmbracelet/lipgloss (styled output), Google Drive API v3, Google Docs API v1, golang.org/x/oauth2, github.com/zalando/go-keyring v0.2.6
**Storage**: Non-secret config in JSON files in `~/.get-out/`; secrets (`token.json`, `credentials.json`) in OS keychain (KeychainStore) with FileStore (mode-0600 files) fallback — no database
**Testing**: `go test -race -count=1 ./...`
**Target Platform**: macOS (arm64 + amd64), Linux (arm64 + amd64) — single static binary (CGO_ENABLED=0)
**Project Type**: CLI
**Performance Goals**: `get-out auth status` completes (including silent token refresh) in under 3 seconds; `get-out doctor` completes all 10 checks in under 10 seconds on a machine with an active network connection
**Constraints**: Single static binary; no CGO; no daemon/service process; credential files written with mode 0600 when FileStore is active; config dir created with mode 0700; go-keyring uses OS-native backends (Security framework on macOS, Secret Service on Linux) with no CGO on macOS
**Scale/Scope**: Single-user CLI tool; no concurrent users; all state in local files or OS keychain

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Session-Driven Extraction | PASS | `setup-browser` guides users to extract the active browser session; no manual token config required |
| II. Go-First Architecture | PASS | All new commands implemented in Go; charmbracelet deps are pure Go; go-keyring is pure Go on macOS |
| III. Stealth & Reliability | PASS | No changes to CDP/Chromedp usage; `setup-browser` validates the existing session |
| IV. Two-Tier Extraction | PASS | No changes to extraction strategy; `setup-browser` replaces `test` with guided validation of Tier 1 |
| V. Concurrency & Resilience | PASS | No changes to export pipeline; `doctor` and `init` are sequential by design |
| VI. Security First | PASS | Secrets stored in OS keychain by default; FileStore fallback uses mode 0600; credentials masked in `setup-browser` output; no tokens hardcoded; `--no-keyring` for headless environments |
| VII. Output Format | PASS | No changes to export output format |
| VIII. Google Drive Integration | PASS | `auth login`/`auth status` improve OAuth UX; SecretStore wraps token read/write; no changes to Drive API usage |
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
├── root.go                        # EDIT: defaultConfigDir() → ~/.get-out; add --no-keyring flag; init SecretStore
├── auth.go                        # REFACTOR: auth group + login + status sub-commands; use SecretStore
├── export.go                      # EDIT: read folder_id from settings.json as default
├── selfservice.go                 # NEW: init, doctor, setup-browser commands; call secrets.Migrate in init
├── helpers.go                     # NEW: safePreview() + shared formatting helpers
├── test.go                        # DELETE: replaced by setup-browser
├── export_test.go                 # EDIT: safePreview reference (same package, no import change)
└── selfservice_test.go            # NEW: unit tests for pure helper functions

pkg/secrets/                       # NEW: SecretStore abstraction (mirrors gcal-organizer/internal/secrets)
├── store.go                       # NEW: SecretStore interface, NewStore(), Backend type, probe logic
├── keychain.go                    # NEW: KeychainStore — OS keychain via go-keyring (service: com.jflowers.get-out)
├── file.go                        # NEW: FileStore — mode-0600 files in ~/.get-out/
├── migrate.go                     # NEW: Migrate() — idempotent token.json + credentials.json → store migration
└── store_test.go                  # NEW: unit tests for FileStore; KeychainStore tested via integration tag

pkg/gdrive/
├── auth.go                        # EDIT: Authenticate/SaveToken accept SecretStore; remove direct file I/O for secrets
└── (other files unchanged)

pkg/config/
├── config.go                      # EDIT: add DefaultConfigDir() exported helper
└── types.go                       # EDIT: add FolderID field to Settings struct

man/
└── get-out.1                      # NEW: troff man page

.goreleaser.yml                    # EDIT: add checksum, homebrew_casks, man page in archives
.github/workflows/release.yml      # REWRITE: add sign-macos job + HOMEBREW_TAP_TOKEN env
README.md                          # EDIT: Homebrew install section, updated Quick Start, keychain note
AGENTS.md                          # EDIT: config dir, commands, new deps including go-keyring
go.mod / go.sum                    # EDIT: add charmbracelet/huh, charmbracelet/lipgloss, go-keyring
```

**Structure Decision**: Single-project CLI (Option 1). `pkg/secrets` is a new top-level package so it can be reused by future tools — matching gcal-organizer's layout decision. The `SecretStore` interface is injected into `pkg/gdrive/auth.go` to replace direct file I/O for `token.json` and `credentials.json`. All command registration follows existing cobra patterns (`RunE`, `init()` registration, `SilenceUsage: true`).

## Testing Strategy

### Tiers

| Tier | Scope | Runner | Command |
|------|-------|--------|---------|
| Unit | Pure functions, FileStore, filesystem helpers (no network, no live services, no OS keychain) | ubuntu-latest + macos-latest | `go test -race -count=1 ./...` |
| Integration (skipped in CI) | KeychainStore on macOS, Google Drive, Chrome, Slack API | Manual only (T034) | `go test -run Integration ./...` |
| E2E | Full `get-out init` → `auth login` → `setup-browser` → `export` flow | Manual only (T034) | — |

### Unit-Testable Functions (`internal/cli`, `pkg/secrets`)

| Function | Test |
|----------|------|
| `validateDriveID` | `TestValidateDriveID` |
| `migrateFiles` | `TestMigrateFiles_*` (5 cases) |
| `oauthToken.Valid()` | `TestOAuthToken_Valid` |
| `loadTokenForDoctor` | `TestLoadTokenForDoctor` |
| `checkConfigDir` | `TestCheckConfigDir` |
| `checkFile` | `TestCheckFile` |
| `checkTokenValidity` | `TestCheckTokenValidity` |
| `checkConversations` | `TestCheckConversations` |
| `checkPeople` | `TestCheckPeople` |
| `checkExportIndex` | `TestCheckExportIndex` |
| `chromeLaunchCmd(goos, port)` | `TestChromeLaunchCmd` (darwin / linux / windows) |
| `FileStore.Get/Set/Delete` | `TestFileStore_*` |
| `secrets.Migrate` (FileStore backend) | `TestMigrate_*` with temp dirs |

### Excluded from Unit Tests (documented)

| Function | Reason |
|----------|--------|
| `KeychainStore.Get/Set/Delete` | Requires OS keychain; tested manually in T034 on macOS |
| `secrets.NewStore` keychain probe | Requires OS keychain; FileStore path covered via `--no-keyring` |
| `checkDriveAPI` | Requires live Google Drive API; covered by T034 manual validation |
| `checkChrome` / `checkSlackTab` | Require live Chrome; covered by T034 |
| `runSetupBrowser` steps 2–5 | Require live Chrome + Slack; covered by T034 |
| `runAuthStatus` Drive API call | Requires live credentials; covered by T034 |
| `promptFolderID` | Requires TTY; `validateDriveID` is tested separately |
| `os.Exit(1)` call sites | Not unit-testable; exit-code contract verified manually in T034 |

### Coverage Target

- **Floor**: ≥ 60% line coverage for new code in `internal/cli` and `pkg/secrets` (excluding documented exclusions above)
- **Measure**: `go test -race -count=1 -coverprofile=coverage.out ./internal/cli/... ./pkg/secrets/... && go tool cover -func=coverage.out`
- **CI**: Coverage profile generated on every PR; no automated enforcement gate (manual review)

### CI Matrix

| Runner | Unit Tests | Integration Tests |
|--------|-----------|------------------|
| ubuntu-latest | `go test -race -count=1 ./...` | skipped |
| macos-latest | `go test -race -count=1 ./...` | skipped |

Integration and E2E tests run manually per T034 on a developer machine with Chrome + Google Drive access.
