# Quickstart: Slack Message Export

**Feature**: 001-slack-message-export  
**Updated**: 2026-02-07

## Prerequisites

1. **Go 1.21+** installed
2. **Chrome/Chromium** running with remote debugging (for browser mode)
3. **Active Slack session** in your browser
4. **Google Cloud Project** with Drive API + Docs API enabled, OAuth 2.0 credentials

## Setup

### 1. Build

```bash
cd get-out
go mod tidy
go build -o get-out ./cmd/get-out
```

### 2. Config Directory

Create your config directory (default: `~/.config/get-out/`):

```bash
mkdir -p ~/.config/get-out
```

Place your Google `credentials.json` here.

### 3. conversations.json

Create `~/.config/get-out/conversations.json`:

```json
{
  "conversations": [
    {
      "id": "D06DDJ2UH2M",
      "name": "John Smith",
      "type": "dm",
      "mode": "browser",
      "export": true
    }
  ]
}
```

### 4. Chrome Remote Debugging

```bash
# macOS
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
  --remote-debugging-port=9222 \
  --user-data-dir="$HOME/chrome-debug-profile"
```

Log into Slack in this browser instance.

### 5. Google OAuth

```bash
./get-out auth --config ~/.config/get-out
```

This opens a browser for Google consent. Token is saved for future use.

## Usage

### List Conversations

```bash
./get-out list --config ~/.config/get-out

# Filter by type
./get-out list --type dm
./get-out list --type mpim
```

### Discover People

Populate `people.json` for @mention → Google email linking:

```bash
./get-out discover --config ~/.config/get-out
```

### Export Messages

```bash
# Export all configured conversations
./get-out export --config ~/.config/get-out

# Export specific conversations by ID
./get-out export D06DDJ2UH2M --config ~/.config/get-out

# Date range
./get-out export --from 2024-01-01 --to 2024-06-30

# Incremental sync (only new messages)
./get-out export --sync

# Resume a crashed export
./get-out export --resume

# Verbose output
./get-out export -v
```

### Check Status

```bash
./get-out status --config ~/.config/get-out
```

Shows per-conversation export progress: status, message count, doc count, threads.

## Output

Exports create a Google Drive folder structure:

```
Slack Exports/
├── DM - John Smith/
│   ├── 2024-01-15 (Google Doc)
│   ├── 2024-01-16 (Google Doc)
│   └── Threads/
│       └── 2024-01-15 - Project discussion/
│           └── thread.gdoc
├── Channel - engineering/
│   └── 2024-01-14 (Google Doc)
└── Group - Alice, Bob, Carol/
    └── 2024-01-16 (Google Doc)
```

Each Google Doc contains messages formatted with:
- **Bold sender names** with timestamps
- **@mentions** linked to Google email (`mailto:`) when `people.json` mapping exists
- **Slack URLs** replaced with links to exported Google Docs
- **[bot]** and **[deactivated]** indicators on special accounts

## Troubleshooting

### "Cannot connect to Chrome"
Ensure Chrome is running with `--remote-debugging-port=9222`.

### "Token extraction failed"
Log into Slack in the debugging Chrome instance and retry.

### "Rate limited"
The tool handles rate limits automatically with exponential backoff. Check progress:
```bash
./get-out status
```

### "Google credentials not found"
Download `credentials.json` from Google Cloud Console and place it in your config directory.
