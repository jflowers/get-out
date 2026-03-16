## Context

The `ExportMode` type (`"api"` / `"browser"`) was introduced to support two authentication paths: browser session tokens (xoxc) for DMs/groups and bot tokens (xoxb) for public channels. The bot token support was removed, making all exports browser-only. The `mode` field in `conversations.json` is never read by the exporter, but its type, constants, validator, filter method, and CLI flag still exist.

## Goals / Non-Goals

### Goals
- Remove all dead code related to `ExportMode`
- Ensure existing `conversations.json` files with `"mode"` fields continue to load without error
- Simplify test fixtures by not requiring a `Mode` field on every `ConversationConfig`

### Non-Goals
- Migrating existing conversations.json files (Go's JSON decoder silently ignores unknown fields)
- Removing `AuthModeAPI` / `NewAPIClient` from `pkg/slackapi` (they're part of the package's public API and aren't specific to export mode)

## Decisions

### 1. Remove the type and constants from models.go

Delete `ExportMode`, `ExportModeAPI`, and `ExportModeBrowser` from `pkg/models/models.go`. This is the source of truth — everything else follows.

### 2. Remove the Mode field from ConversationConfig

Delete `Mode models.ExportMode` from the struct in `pkg/config/types.go`. Go's `json.Unmarshal` silently ignores JSON fields that don't have a corresponding struct field, so existing config files with `"mode": "api"` or `"mode": "browser"` will load without error.

### 3. Remove isValidExportMode and its call site

Delete the `isValidExportMode` function from `pkg/config/config.go` and remove the mode validation check from `validateConversationConfig`.

### 4. Remove FilterByMode

Delete the `FilterByMode` method from `ConversationsConfig` in `pkg/config/config.go`. Its only caller is the `--mode` flag handler in `list.go`.

### 5. Remove --mode flag from list command

Delete the `listMode` variable, the flag registration, and the mode filtering logic from `internal/cli/list.go`. Remove the mode display from the `listCore` output formatting.

### 6. Remove mode display from export dry-run

Delete the `Mode: %s` line from `formatExportDryRun` in `internal/cli/export.go`.

### 7. Update all test fixtures

Remove `Mode:` field assignments from `ConversationConfig` literals in all test files. Remove `TestFilterByMode` and `TestListCore_FilterByMode` tests entirely.

## Risks / Trade-offs

### Backward compatibility with conversations.json

No risk. Go's JSON decoder ignores unknown fields by default. Users with `"mode": "api"` in their config will see no change in behavior — the field is silently skipped during deserialization. If they edit the file later, they can remove the field, but they don't have to.

### The discover command writes conversations.json

The `discover` command generates `conversations.json` entries. It currently sets `Mode: "browser"` on generated entries. This field assignment will be removed, so newly generated entries won't have a `mode` field — which is correct since mode is no longer meaningful.
