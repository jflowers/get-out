# get-out Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-02-03

## Active Technologies

- Go 1.21+ + Chromedp (CDP), cobra (CLI), errgroup (concurrency)
- Google Drive API v3 + Google Docs API

## Project Structure

```text
get-out/
├── cmd/get-out/main.go       # CLI entry point
├── internal/cli/             # Command implementations
│   ├── root.go               # Base command and global flags
│   ├── auth.go               # Google OAuth command
│   ├── list.go               # List conversations command
│   ├── test.go               # Test browser connection
│   └── export.go             # Export command
├── pkg/
│   ├── chrome/               # Chrome DevTools Protocol client
│   ├── slackapi/             # Slack API client (browser + bot modes)
│   ├── gdrive/               # Google Drive/Docs API client
│   ├── exporter/             # Export orchestration and indexing
│   ├── parser/               # Slack mrkdwn conversion
│   ├── config/               # Configuration loading
│   └── models/               # Shared data models
├── config/                   # Configuration files (gitignored except examples)
│   ├── conversations.json    # Conversations to export
│   ├── people.json           # User ID to name mappings
│   └── credentials.json      # Google OAuth credentials
└── specs/                    # Feature specifications
```

## Commands

### Build
```bash
go build -o get-out ./cmd/get-out
```

### Run
```bash
# Authenticate with Google
./get-out auth --config ./config

# List configured conversations
./get-out list --config ./config

# Test Chrome connection
./get-out test --config ./config

# Export (dry run)
./get-out export --dry-run --config ./config

# Export to specific folder ID
./get-out export --folder-id <google-drive-folder-id> --config ./config
```

### Test
```bash
go test ./...
```

## Code Style

- Go 1.21+: Follow standard Go conventions (gofmt, golint)
- Error handling: Wrap errors with context using fmt.Errorf
- Logging: Use progress callbacks for user-facing output

## Documentation Requirements

When making changes, always review and update:
1. **README.md** - Usage examples, flags, configuration
2. **AGENTS.md** - Build commands, project structure
3. **Constitution** - Architectural decisions

## Recent Changes

- 001-slack-message-export: Core export functionality with browser mode
- Added --folder-id flag for exporting to existing Drive folders

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
