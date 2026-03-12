# Quickstart Guide: get-out

**Version**: after 003-distribution  
**Last updated**: 2026-03-11

---

## Installation

### Homebrew (macOS and Linux — recommended)

```bash
brew tap jflowers/tools
brew install get-out
```

The Homebrew cask installs:
- The `get-out` binary (signed and notarized on macOS)
- The man page (`man get-out`)

### From source

```bash
git clone https://github.com/jflowers/get-out.git
cd get-out
go build -o get-out ./cmd/get-out
```

---

## First-Time Setup

Run these steps in order. Each command tells you what to do next.

### 1. Initialize your config directory

```bash
get-out init
```

This creates `~/.get-out/` and prompts you for your Google Drive folder ID.

**To find your folder ID**: Open the destination folder in Google Drive. The ID is the long string at the end of the URL:

```
https://drive.google.com/drive/folders/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgVE2upms
                                        ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
                                        This is your folder ID
```

### 2. Place your Google Cloud credentials

You need an OAuth 2.0 "Desktop app" credential from Google Cloud Console.

1. Go to [console.cloud.google.com/apis/credentials](https://console.cloud.google.com/apis/credentials)
2. Click **Create Credentials → OAuth client ID**
3. Choose **Desktop app**
4. Download the JSON file
5. Copy it to `~/.get-out/credentials.json`

Make sure the following APIs are enabled in your project:
- Google Drive API
- Google Docs API

> **Keychain note**: On macOS (and Linux with a Secret Service daemon), `get-out init` automatically migrates `credentials.json` into your OS keychain and offers to delete the plaintext file. This is the recommended setup. If you prefer to keep the file on disk (e.g., in a shared/CI environment), use `--no-keyring`.

### 3. Authenticate with Google

```bash
get-out auth login
```

This opens a browser window for Google OAuth consent. After you approve, the token is stored in the OS keychain (or in `~/.get-out/token.json` if `--no-keyring` is set or the keychain is unavailable).

### 4. Connect Chrome to Slack

get-out uses your existing Chrome session to extract a Slack token. You need Chrome running with remote debugging enabled.

**Launch Chrome with remote debugging** (macOS):

```bash
open -a "Google Chrome" --args --remote-debugging-port=9222
```

**Launch Chrome with remote debugging** (Linux):

```bash
google-chrome --remote-debugging-port=9222 &
```

Then navigate to [app.slack.com](https://app.slack.com) and sign in to your workspace.

Run the browser setup wizard:

```bash
get-out setup-browser
```

This verifies Chrome is reachable, finds your Slack tab, extracts credentials, and validates them against the Slack API — reporting a pass/warn/fail for each step.

### 5. Configure your conversations

Edit `~/.get-out/conversations.json` to list the conversations you want to export:

```json
{
  "conversations": [
    {
      "id": "D0123ABCDE",
      "name": "John Smith",
      "type": "dm",
      "mode": "browser",
      "export": true
    },
    {
      "id": "C0456FGHIJ",
      "name": "engineering",
      "type": "channel",
      "mode": "api",
      "export": true
    }
  ]
}
```

**Conversation types**: `dm`, `mpim`, `channel`, `private_channel`  
**Export modes**: `browser` (uses Chrome session), `api` (uses bot token)

To discover conversations automatically:

```bash
get-out discover
```

This connects to Chrome, fetches your Slack channels and DMs, and writes `~/.get-out/people.json`.

### 6. Run a health check

```bash
get-out doctor
```

All 10 checks should show ✓. Fix any ✗ failures before exporting.

### 7. Export

```bash
get-out export
```

Exports all conversations with `"export": true` in `conversations.json` to your Google Drive folder.

---

## Common Commands

| Command | Description |
|---------|-------------|
| `get-out init` | First-run setup: create `~/.get-out/`, prompt for folder ID |
| `get-out auth login` | Authenticate with Google (OAuth 2.0 browser flow) |
| `get-out auth status` | Check token validity and connected Google account |
| `get-out doctor` | Run 10-point environment health check |
| `get-out setup-browser` | Verify Chrome connection and Slack session (5-step wizard) |
| `get-out discover` | Auto-generate `people.json` from Slack API |
| `get-out list` | List configured conversations |
| `get-out export` | Export all enabled conversations to Google Docs |
| `get-out status` | Show export progress from checkpoint index |

---

## Migrating from an Earlier Version

If you previously used get-out with config in `~/.config/get-out/`, run:

```bash
get-out init
```

Your existing files (`conversations.json`, `people.json`, `credentials.json`, `token.json`) are automatically copied to `~/.get-out/`. No data is lost and the old directory is not deleted — you can remove it manually once you've verified everything works:

```bash
rm -rf ~/.config/get-out/
```

---

## Config Directory Reference

All files live in `~/.get-out/` (or the path specified by `--config`):

| File | Purpose | Created by |
|------|---------|-----------|
| `settings.json` | App preferences (folder ID, credentials path) | `init` |
| `conversations.json` | List of Slack conversations to export | `init` (template), user edits |
| `people.json` | Slack user ID → name/email mapping | `discover` |
| `credentials.json` | Google OAuth 2.0 client credentials | User (manual copy) |
| `token.json` | Google OAuth 2.0 access/refresh token | `auth login` |
| `export-index.json` | Export checkpoint and progress tracking | `export` |

---

## Troubleshooting

**Gatekeeper warning on macOS?**  
This should not happen with a Homebrew-installed binary (it is signed and notarized). If you built from source, right-click the binary in Finder and choose Open, or run:

```bash
xattr -d com.apple.quarantine $(which get-out)
```

**`get-out doctor` shows Chrome not reachable?**  
Launch Chrome with remote debugging enabled (see Step 4 above). Make sure to use the same port as `--chrome-port` (default: 9222).

**Token expired?**

```bash
get-out auth login
```

**Not sure what's wrong?**

```bash
get-out doctor --verbose
```

This shows the resolved path for every checked resource.

---

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config <dir>` | `~/.get-out/` | Override config directory |
| `--chrome-port <n>` | `9222` | Chrome DevTools Protocol port |
| `--verbose` / `-v` | false | Verbose output |
| `--debug` | false | Debug output |
| `--no-keyring` | false | Disable OS keychain; keep secrets in plaintext files (0600) |

Example using a custom config dir (e.g., for a second Slack workspace):

```bash
get-out --config ~/.get-out-workspace2 export
```
