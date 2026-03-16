## ADDED Requirements

_None._

## MODIFIED Requirements

### Requirement: Conversation Configuration

Previously: Each conversation in `conversations.json` required a `mode` field set to `"api"` or `"browser"` to determine the authentication path for export.

Now: The `mode` field is removed. All conversations are exported using browser credentials. Existing config files with `"mode"` fields continue to load without error (the field is silently ignored by Go's JSON decoder).

#### Scenario: Config without mode field loads successfully
- **GIVEN** a `conversations.json` file with entries that do not have a `"mode"` field
- **WHEN** the config is loaded via `LoadConversations`
- **THEN** the config MUST load successfully with all other fields populated

#### Scenario: Legacy config with mode field loads successfully
- **GIVEN** a `conversations.json` file with entries that have `"mode": "api"` or `"mode": "browser"`
- **WHEN** the config is loaded via `LoadConversations`
- **THEN** the config MUST load successfully and the `mode` field MUST be silently ignored

## REMOVED Requirements

### Requirement: Export Mode Validation

Previously: `LoadConversations` validated that the `mode` field was either `"api"` or `"browser"`, returning an error for invalid values.

Now: Removed. The `mode` field no longer exists in the struct, so no validation is performed.

### Requirement: Filter by Export Mode

Previously: The `list` command supported a `--mode` flag to filter conversations by export mode. `ConversationsConfig` provided a `FilterByMode` method.

Now: Removed. The `--mode` flag and `FilterByMode` method no longer exist.
