# CLI Command Schema: Distribution, Config Migration & Self-Service Commands

**Feature**: 003-distribution  
**Date**: 2026-03-11  
**Project type**: CLI (`project_type: cli`)

This document defines the complete command schema for all new and modified commands introduced by this feature. Commands are described as interface contracts — inputs (flags, arguments, environment), outputs (stdout, stderr, exit codes), and side effects (files written, APIs called).

---

## Global Flags (unchanged, documented for reference)

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `~/.get-out/` | Config directory path |
| `--chrome-port` | int | `9222` | Chrome DevTools Protocol port |
| `--verbose` / `-v` | bool | `false` | Verbose output |
| `--debug` | bool | `false` | Enable debug output |

---

## `get-out init`

**Purpose**: First-run setup. Scaffold the config directory, migrate legacy files, prompt for Google Drive folder ID.

### Input

| Name | Type | Default | Description |
|------|------|---------|-------------|
| `--non-interactive` | bool flag | `false` | Skip interactive folder-ID prompt |

### Behaviour (sequential)

1. Create `~/.get-out/` with mode `0700` if absent
2. If `~/.config/get-out/` exists: for each managed file (`settings.json`, `conversations.json`, `people.json`, `credentials.json`, `token.json`) present in old dir but absent in `~/.get-out/` → copy; `credentials.json`/`token.json` copied with mode `0600`
3. Write `~/.get-out/conversations.json` template if absent
4. Write `~/.get-out/settings.json` template if absent
5. If not `--non-interactive` AND `settings.folder_id == ""` → show huh prompt; on accept, write `folder_id` to `settings.json`
6. Print "Next Steps" box

### Output

| Stream | Content |
|--------|---------|
| stdout | One line per copied file (migration); "Next Steps" box |
| stderr | Error message on failure (directory is a file, write permission denied, etc.) |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Fatal error (e.g., `~/.get-out/` is a file, write failed) |

### Side effects

| Effect | Condition |
|--------|-----------|
| Creates `~/.get-out/` (mode 0700) | Dir did not exist |
| Writes `conversations.json` | File absent |
| Writes `settings.json` | File absent |
| Writes `folder_id` into `settings.json` | Prompt answered with valid ID |
| Copies managed files from old dir | Old dir exists, files absent in new dir |

---

## `get-out auth` (command group)

**Purpose**: Group for authentication sub-commands. Running without a sub-command prints help.

### Sub-commands
- `get-out auth login`
- `get-out auth status`

### Exit code when called without sub-command
| Code | Meaning |
|------|---------|
| `0` | Help printed |

---

## `get-out auth login`

**Purpose**: Perform Google OAuth 2.0 browser consent flow; save token to `token.json`.

### Input

No additional flags beyond globals.

### Pre-conditions (checked before OAuth flow)

| Check | Failure behaviour |
|-------|------------------|
| `credentials.json` exists at resolved path | Exit 1 with instructions (path to place file, Google Cloud Console URL) |

### Behaviour

1. Load `credentials.json` from config dir (or `settings.google_credentials_file` if set)
2. Start local OAuth callback server on `127.0.0.1:8085` with CSRF state token
3. Print auth URL for user to open in browser
4. Wait for callback (5-minute timeout)
5. Exchange auth code for token; save to `token.json` (mode `0600`)
6. Verify by calling `drive.About.Get`; print connected Google account email

### Output

| Stream | Content |
|--------|---------|
| stdout | Auth URL, progress messages, "Authentication successful!", connected email |
| stderr | Error message on failure |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Authentication succeeded; `token.json` written |
| `1` | `credentials.json` missing; OAuth flow failed; Drive verification failed; timeout |

### Side effects

| Effect | Condition |
|--------|-----------|
| Writes `token.json` (mode 0600) | OAuth flow succeeded |

---

## `get-out auth status`

**Purpose**: Read-only auth health check. Reports credential/token state and connected account. Silently refreshes an expired token if a refresh token is present.

### Input

No additional flags beyond globals.

### Behaviour

1. Check `credentials.json` present (no read/parse required — existence only)
2. Check `token.json` present
3. Attempt `gdrive.EnsureTokenFresh` (silent refresh if expired + refresh token present; save refreshed token)
4. Report `token.Valid()` result and expiry
5. If token valid: call `drive.About.Get` to retrieve and print connected email

### Output

```
Credentials:   ✓ found  (~/.get-out/credentials.json)
Token:         ✓ found  (~/.get-out/token.json)
Token valid:   ✓ expires 2026-04-11 10:32:00 UTC
Account:       user@example.com
```

On failure:

```
Credentials:   ✓ found
Token:         ✗ not found
               → Run: get-out auth login
```

| Stream | Content |
|--------|---------|
| stdout | Status table (4 rows) |
| stderr | (none — errors are inline in status table) |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Token present and valid (or successfully refreshed) |
| `1` | Token absent, expired with no refresh token, or Drive API unreachable |

### Side effects

| Effect | Condition |
|--------|-----------|
| Overwrites `token.json` (mode 0600) | Token was expired and silent refresh succeeded |

---

## `get-out doctor`

**Purpose**: Environment health check. Runs 10 checks in order; prints styled pass/warn/fail rows; exits 0 if no failures, 1 if any check fails.

### Input

| Name | Type | Default | Description |
|------|------|---------|-------------|
| `--verbose` / `-v` | bool flag | `false` | Show resolved file path for each check inline |

(`--verbose` is also a global flag; `doctor` honours it specifically for path display.)

### Checks (in order)

| # | Name | Pass condition | Warn condition | Fail condition | Fix message |
|---|------|---------------|---------------|----------------|-------------|
| 1 | Config dir | `~/.get-out/` exists, mode `0700` | exists but wrong perms | does not exist | `Run: get-out init` |
| 2 | credentials.json | file exists | — | missing | instructions + Google Cloud Console URL |
| 3 | token.json | file exists | — | missing | `Run: get-out auth login` |
| 4 | Token valid | `token.Valid() == true` | expired but refresh token present (auto-refresh capable) | expired, no refresh token | `Run: get-out auth login` |
| 5 | Drive API | `drive.About.Get` succeeds | — | error | print error message |
| 6 | conversations.json | file present and parses, N≥1 conversations | file present but 0 conversations | missing or invalid JSON | edit file path |
| 7 | people.json | file present | missing (optional) | — | `Run: get-out discover` |
| 8 | Chrome port | CDP `/json/version` responds on `--chrome-port` | — | not responding | OS-appropriate Chrome launch command |
| 9 | Slack tab | ≥1 Slack tab found | 0 Slack tabs | — (only shown if check 8 passed) | navigate to app.slack.com |
| 10 | export-index.json | file present and parses | missing (first-run — ok) | corrupt JSON | check for disk corruption |
| — | Old config dir | `~/.config/get-out/` absent | both old and new dir exist | — | `rm -rf ~/.config/get-out/` |

**Note**: Check 9 is skipped (reported as skipped) if check 8 failed.  
**Note**: Old config dir warning is shown between checks 10 and the summary if both directories exist; it is counted as a warning.

### Output

```
get-out doctor
──────────────────────────────────────────
  ✓ Config dir (~/.get-out/, mode 0700)
  ✓ credentials.json
  ✓ token.json
  ✓ Token valid (expires 2026-04-11)
  ✓ Google Drive API reachable
  ✓ conversations.json (3 conversations)
  ⚠ people.json — not found (optional; run: get-out discover)
  ✓ Chrome reachable (port 9222)
  ✓ Slack tab found (app.slack.com)
  ⚠ export-index.json — not found (first run)
──────────────────────────────────────────
  8 passed · 2 warnings · 0 failures
```

With `--verbose`:
```
  ✓ Config dir  /Users/you/.get-out/  (mode 0700)
  ✓ credentials.json  /Users/you/.get-out/credentials.json
  ...
```

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | All checks passed or warn-only |
| `1` | One or more checks failed |

### Side effects

None. `doctor` is read-only (except for the optional silent token refresh inherited from check #4 calling `EnsureTokenFresh`, which writes `token.json` as a side effect).

---

## `get-out setup-browser` (replaces `test`)

**Purpose**: Guided 5-step Chrome connection wizard. Validates Chrome reachability, Slack tab presence, credential extraction, and API auth.

### Input

No additional flags beyond globals (`--chrome-port` controls which port is checked).

### Behaviour (5 steps, sequential — any failure skips remaining steps)

| Step | Name | Action | Pass | Warn | Fail |
|------|------|--------|------|------|------|
| 1 | Chrome port | GET `http://127.0.0.1:<port>/json/version` | Version string received | — | No response → print OS launch command |
| 2 | List tabs | `session.ListTargets()` | N page tabs listed | 0 tabs | Error calling CDP |
| 3 | Slack tab | `chrome.IsSlackURL` on each tab URL | ≥1 Slack tab found | 0 Slack tabs (warn, not skip) | — |
| 4 | Extract credentials | `chrome.ExtractCredentials` | Token extracted (masked preview) | — | Extraction failed |
| 5 | Validate Slack API | `slackapi.Client.ValidateAuth` | Workspace + username printed | — | Token rejected |

**Skip logic**: If step N fails (`status == fail`), steps N+1 through 5 are reported as "  — Skipped (step N failed)" without executing.  
**Step 3**: A warn (no Slack tab) does NOT skip step 4. Only a fail does.

### Output

```
get-out setup-browser
──────────────────────────────────────────
  ✓ Step 1 — Chrome reachable (port 9222, Chrome 131.0.6778.85)
  ✓ Step 2 — 4 tabs listed
  ✓ Step 3 — Slack tab found (app.slack.com/client/T0123456)
  ✓ Step 4 — Credentials extracted (xoxc-123456789012-...abc1)
  ✓ Step 5 — Authenticated as My Workspace / John Flowers
──────────────────────────────────────────
  Setup complete. You can now run: get-out export
```

Failure example:
```
  ✗ Step 1 — Chrome not responding on port 9222
             Launch Chrome with remote debugging:
             open -a "Google Chrome" --args --remote-debugging-port=9222
  — Step 2 — Skipped (step 1 failed)
  — Step 3 — Skipped (step 1 failed)
  — Step 4 — Skipped (step 1 failed)
  — Step 5 — Skipped (step 1 failed)
```

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | All 5 steps passed |
| `1` | Any step failed |

### Side effects

None. `setup-browser` is read-only with respect to local files.

---

## Commands Removed by This Feature

| Command | Replacement |
|---------|-------------|
| `get-out test` | `get-out setup-browser` |
| `get-out auth` (bare login) | `get-out auth login` |

---

## Release Pipeline Contracts

### GitHub Actions: `sign-macos` job

**Trigger**: After `release` job completes AND `has_signing_secrets == 'true'`

**Inputs** (secrets):

| Secret | Description |
|--------|-------------|
| `MACOS_SIGN_P12` | Base64-encoded Developer ID Application `.p12` |
| `MACOS_SIGN_PASSWORD` | `.p12` decryption password |
| `MACOS_NOTARY_KEY` | Base64-encoded App Store Connect API `.p8` key |
| `MACOS_NOTARY_KEY_ID` | App Store Connect API key ID (`4K669B7BD9`) |
| `MACOS_NOTARY_ISSUER_ID` | App Store Connect issuer UUID |
| `HOMEBREW_TAP_TOKEN` | PAT with write access to `jflowers/homebrew-tools` |

**Outputs** (to GitHub Release):

| Asset | Description |
|-------|-------------|
| `get-out_<ver>_darwin_arm64.tar.gz` | Signed + notarized arm64 archive (replaces unsigned) |
| `get-out_<ver>_darwin_amd64.tar.gz` | Signed + notarized amd64 archive (replaces unsigned) |
| `checksums.txt` | Updated with signed archive SHA256 values |

**Side effects**:
- Pushes commit to `jflowers/homebrew-tools` patching `Casks/get-out.rb` `sha256` values for darwin arches

**Graceful degradation**: If `MACOS_SIGN_P12` secret is absent, `sign-macos` job does not run; unsigned release assets remain published; Homebrew cask SHA values are not updated (cask may be temporarily out of sync until next signed release).
