# Quickstart: Slack Message Export

**Feature**: 001-slack-message-export  
**Date**: 2026-02-03

## Prerequisites

1. **Go 1.21+** installed
2. **Chrome/Chromium** browser (or Zen browser)
3. **Active Slack session** in your browser (logged in to the workspace)

## Setup

### 1. Clone and Build

```bash
cd get-out
go mod tidy
go build -o get-out ./cmd/get-out
```

### 2. Enable Chrome Remote Debugging

Before running the export, start Chrome with remote debugging enabled:

```bash
# macOS
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
  --remote-debugging-port=9222 \
  --user-data-dir="$HOME/chrome-debug-profile"

# Or with Zen browser (adjust path as needed)
/Applications/Zen.app/Contents/MacOS/Zen \
  --remote-debugging-port=9222
```

Then log into Slack in this browser instance.

### 3. Verify Connection

```bash
# Test that Chrome is accessible
curl http://localhost:9222/json/version
```

## Basic Usage

### List Available Conversations

```bash
./get-out list
```

Output:
```
DMs:
  D123ABC456  John Smith
  D789DEF012  Jane Doe

Groups:
  G111222333  project-alpha (3 members)
  G444555666  team-standup (5 members)
```

### Export a Single Conversation

```bash
# Export by conversation ID
./get-out export D123ABC456

# Export by name (DM)
./get-out export --name "John Smith"

# Export to specific directory
./get-out export D123ABC456 --output ./my-exports
```

### Export All DMs

```bash
./get-out export --all-dms
```

### Resume Interrupted Export

```bash
# Resume automatically detects checkpoint
./get-out export D123ABC456 --resume
```

## Output Format

Exports are saved as Markdown files:

```
export/
├── dm_john-smith_2026-02-03/
│   ├── messages.md
│   └── metadata.json
└── users.json
```

### Sample Output (messages.md)

```markdown
# Conversation with John Smith

Exported: 2026-02-03 10:30:00 UTC
Messages: 547

---

## 2026-01-15

**John Smith** (10:30 AM):
Hey, did you see the latest PR?

**You** (10:32 AM):
Yes! Looks good, I'll review it today.

> **Thread** (3 replies)
>
> **Jane Doe** (10:35 AM):
> I can help with the review too
>
> **John Smith** (10:36 AM):
> That would be great, thanks!

---

## 2026-01-16
...
```

## CLI Reference

```
get-out - Export Slack messages to Markdown

USAGE:
  get-out <command> [options]

COMMANDS:
  list        List available conversations
  export      Export conversation(s) to Markdown
  resume      Resume an interrupted export
  status      Show export session status

GLOBAL OPTIONS:
  --debug           Enable debug logging
  --chrome-port     Chrome debugging port (default: 9222)
  --help, -h        Show help

EXPORT OPTIONS:
  --output, -o      Output directory (default: ./export)
  --name            Export by conversation name
  --all-dms         Export all direct messages
  --all-groups      Export all group conversations
  --resume          Resume from checkpoint
  --since           Only messages after date (YYYY-MM-DD)
  --until           Only messages before date (YYYY-MM-DD)
```

## Troubleshooting

### "Cannot connect to Chrome"

Ensure Chrome is running with `--remote-debugging-port=9222` and no other process is using that port.

### "Token extraction failed"

1. Make sure you're logged into Slack in the debugging Chrome instance
2. Navigate to your Slack workspace in the browser
3. Try the export again

### "Rate limited"

The tool automatically handles rate limits with exponential backoff. For very large exports, this is normal. Check progress with:

```bash
./get-out status
```

### "Session expired"

Your Slack session may have timed out. Log back into Slack in the browser and retry.

## Development

### Run Tests

```bash
go test ./...
```

### Run with Verbose Logging

```bash
./get-out export D123ABC456 --debug
```
