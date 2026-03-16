## Why

The `mode` field in `conversations.json` (`"api"` or `"browser"`) is dead code. It was designed to support two authentication paths — browser session tokens for DMs/groups and bot tokens for public channels. The bot token support was removed in a prior change, and the exporter now uses browser credentials exclusively for all conversation types. The `mode` field is never read during export, but it still requires validation during config loading and is displayed in CLI output, creating the false impression that it matters.

Removing it eliminates confusion, reduces code surface, and removes 5 functions/constants that exist only to support a distinction that no longer affects behavior.

## What Changes

Remove the `ExportMode` type, its constants (`ExportModeAPI`, `ExportModeBrowser`), the `Mode` field from `ConversationConfig`, the `isValidExportMode()` validator, the `FilterByMode()` method, and the `--mode` flag from the `list` command. Update all tests that reference these.

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `ConversationConfig`: The `Mode` field is removed. Existing `conversations.json` files with `"mode"` will silently ignore the field (Go's JSON decoder ignores unknown fields by default).
- `list` command: The `--mode` flag is removed. The mode column is removed from list output.
- `export --dry-run`: No longer prints `Mode: api/browser` per conversation.
- Config validation: No longer validates the `mode` field when loading `conversations.json`.

### Removed Capabilities
- `ExportMode` type and `ExportModeAPI`/`ExportModeBrowser` constants: No longer needed since all exports use browser mode.
- `FilterByMode()`: No callers remain outside the `list` command's `--mode` flag, which is also removed.
- `isValidExportMode()`: Validation for a field that no longer exists.

## Impact

- **Config files**: Existing `conversations.json` files with `"mode"` fields continue to load without error — Go's JSON decoder silently ignores unknown fields. No migration needed.
- **Files changed**: `pkg/models/models.go`, `pkg/config/types.go`, `pkg/config/config.go`, `internal/cli/list.go`, `internal/cli/export.go`, plus 6 test files.
- **No behavioral changes**: Export behavior is unchanged because the `mode` field was already ignored during export.
- **Zero-waste**: This change removes ~50 lines of dead code and 5 dead symbols, satisfying the constitution's zero-waste mandate.

## Constitution Alignment

Assessed against the get-out project constitution.

### I. Session-Driven Extraction

**Assessment**: N/A

This change does not alter the extraction strategy. All exports continue to use browser session tokens via Chrome CDP.

### II. Go-First Architecture

**Assessment**: PASS

Pure removal of dead Go code. No new dependencies, no architectural changes.

### III. Stealth & Reliability

**Assessment**: N/A

No changes to network behavior, rate limiting, or API interactions.

### IV. Testability

**Assessment**: PASS

Removes test code for dead functionality. Remaining tests are simplified by not needing to specify a `Mode` field on every `ConversationConfig` in test fixtures.
