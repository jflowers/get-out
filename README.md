# get-out

A CLI tool to export Slack messages (DMs, groups, channels) to Google Docs with organized folder structure.

## Features

- **Browser-based extraction**: Access DMs and private group messages via Chrome DevTools Protocol
- **API-based extraction**: Access public/private channels via Slack bot token
- **Google Drive integration**: Creates organized folder hierarchy with daily Google Docs
- **Thread support**: Exports threads to separate subfolders with linked references
- **Resumable exports**: Checkpoint system allows resuming interrupted exports
- **Name resolution**: Converts Slack user IDs to real names in exported documents

## Prerequisites

1. **Go 1.21+** - For building the binary
2. **Google Cloud Project** with:
   - Drive API enabled
   - Docs API enabled
   - OAuth 2.0 credentials (Desktop app type)
3. **Chrome/Chromium** running with remote debugging enabled (for browser mode)
4. **Slack workspace access** - Active session in browser or bot token

## Installation

```bash
# Clone the repository
git clone https://github.com/jflowers/get-out.git
cd get-out

# Build the binary
go build -o get-out ./cmd/get-out

# Verify installation
./get-out --help
```

## Configuration

### 1. Google Cloud Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing
3. Enable **Google Drive API** and **Google Docs API**
4. Create OAuth 2.0 credentials (Desktop application)
5. Download `credentials.json` to your config directory

### 2. Config Directory

Create a config directory (default: `~/.config/get-out/`):

```bash
mkdir -p ~/.config/get-out
# Place credentials.json here
```

Or use a local config directory with `--config ./config`.

### 3. conversations.json

Define which conversations to export:

```json
{
    "conversations": [
        {
            "id": "C04KFBJTDJR",
            "name": "team-engineering",
            "type": "channel",
            "mode": "api",
            "export": true,
            "share": true
        },
        {
            "id": "D06DDJ2UH2M",
            "name": "John Smith",
            "type": "dm",
            "mode": "browser",
            "export": true,
            "share": true
        }
    ]
}
```

**Fields:**
- `id`: Slack conversation ID (C=channel, D=DM, G=group)
- `name`: Display name for the export folder
- `type`: `channel`, `private_channel`, `dm`, or `mpim`
- `mode`: `browser` (uses Chrome session) or `api` (uses bot token)
- `export`: Set to `true` to include in export
- `share`: Whether to share the exported folder (future feature)
- `shareMembers`: Optional list of emails to share with

### 4. people.json (Optional)

Map Slack user IDs to display names and preferences:

```json
{
    "people": [
        {
            "slackId": "U1234567890",
            "email": "user@example.com",
            "displayName": "John Doe"
        },
        {
            "slackId": "U0987654321",
            "displayName": "Jane Smith",
            "noNotifications": true
        }
    ]
}
```

## Usage

### Authenticate with Google

Run this first to complete OAuth flow:

```bash
./get-out auth --config ./config
```

This opens a browser for Google consent and saves the token.

### List Configured Conversations

```bash
./get-out list --config ./config
```

### Test Browser Connection

Start Chrome with remote debugging, then test the connection:

```bash
# Start Chrome (macOS example)
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222

# Test connection and token extraction
./get-out test --config ./config
```

### Export Messages

```bash
# Dry run - see what would be exported
./get-out export --dry-run --config ./config

# Export all configured conversations
./get-out export --config ./config

# Export specific conversations
./get-out export D06DDJ2UH2M C04KFBJTDJR --config ./config

# With verbose output
./get-out export --config ./config -v

# Custom Chrome port
./get-out export --chrome-port 9223 --config ./config

# Custom Drive folder name
./get-out export --folder "My Slack Archive" --config ./config
```

### Global Flags

```
--config string      Config directory path (default "~/.config/get-out")
--chrome-port int    Chrome DevTools Protocol port (default 9222)
--debug              Enable debug output
-v, --verbose        Verbose output
```

## Output Structure

Exported content is organized in Google Drive as:

```
Slack Exports/
├── DM - John Smith/
│   ├── 2024-01-15.gdoc
│   ├── 2024-01-16.gdoc
│   └── Threads/
│       └── 2024-01-15 - Project discussion/
│           └── 2024-01-15.gdoc
├── Channel - engineering/
│   ├── 2024-01-14.gdoc
│   └── 2024-01-15.gdoc
└── Group - Alice, Bob, Carol/
    └── 2024-01-16.gdoc
```

## Project Structure

```
get-out/
├── cmd/get-out/          # CLI entry point
├── internal/cli/         # Command implementations
│   ├── root.go           # Base command and global flags
│   ├── auth.go           # Google OAuth command
│   ├── list.go           # List conversations command
│   ├── test.go           # Test browser connection
│   └── export.go         # Export command
├── pkg/
│   ├── chrome/           # Chrome DevTools Protocol client
│   ├── slackapi/         # Slack API client (browser + bot modes)
│   ├── gdrive/           # Google Drive/Docs API client
│   ├── exporter/         # Export orchestration and indexing
│   ├── parser/           # Slack mrkdwn conversion
│   ├── config/           # Configuration loading
│   └── models/           # Shared data models
├── config/               # Example configuration files
└── specs/                # Feature specifications
```

## How It Works

### Browser Mode (for DMs/Groups)

1. Connects to Chrome via DevTools Protocol (port 9222)
2. Finds active Slack tab and extracts `xoxc` token from localStorage
3. Extracts `xoxd` cookie for authentication
4. Makes authenticated API calls to Slack's internal endpoints

### API Mode (for Channels)

1. Uses `xoxb` bot token (configured via environment or config)
2. Makes standard Slack API calls
3. Requires bot to be installed in the workspace

### Export Process

1. Authenticates with Google Drive
2. Creates folder structure (root -> conversation -> threads)
3. Fetches messages with pagination
4. Groups messages by date
5. Writes to Google Docs with formatting
6. Saves checkpoint for resume capability

## Security Notes

- Tokens are extracted at runtime from active browser sessions
- Google OAuth tokens are stored locally in `token.json`
- Never commit `credentials.json`, `token.json`, or `conversations.json` with real data
- The `.gitignore` excludes sensitive files by default

## Troubleshooting

### "No Slack tab found in browser"
Make sure Chrome is running with `--remote-debugging-port=9222` and has an active Slack tab open.

### "Failed to connect to browser"
Check that Chrome is running and the port matches `--chrome-port`.

### "No valid xoxc token found"
Ensure you're logged into Slack in the browser. Try refreshing the Slack tab.

### "Google credentials not found"
Download `credentials.json` from Google Cloud Console and place it in your config directory.

## License

MIT
