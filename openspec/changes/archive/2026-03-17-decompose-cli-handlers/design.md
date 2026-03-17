## Context

`runDiscover` (cx=44) and `runExport` (cx=37) are the two highest-complexity functions. Both have gaze `fix_strategy: decompose_and_test`. Earlier decomposition extracted 6 functions but the residual handlers remain above threshold. This change continues extracting testable logic from both.

The extraction pattern established in prior work: pull pure logic into functions that accept explicit parameters and return values, leaving the cobra handler as a thin orchestrator that constructs dependencies and calls the extracted functions in sequence.

## Goals / Non-Goals

### Goals
- Reduce `runDiscover` from cx=44 to ≤15
- Reduce `runExport` from cx=37 to ≤15
- Make the extracted functions testable without external services
- Preserve all existing behavior exactly

### Non-Goals
- Testing the residual cobra handlers (they still construct real Chrome/Slack/Drive clients)
- Changing the discover or export command output format
- Introducing interfaces for Chrome or Slack clients (separate future change)
- Reducing the CRAP scores of other functions

## Decisions

### 1. Extract collectChannelMembers from runDiscover

The member-fetching loop (lines 109-172 of discover.go) iterates conversations, skips DMs/MPIMs, calls `GetConversationMembers` with cursor pagination, and accumulates unique member IDs. This is a self-contained block with cx≈12.

```go
// collectChannelMembers fetches unique member IDs from channel conversations.
// It skips DM and MPIM conversations (which don't support member listing).
// Returns the member set and the count of skipped conversations.
func collectChannelMembers(
    ctx context.Context,
    client *slackapi.Client,
    conversations []config.ConversationConfig,
    progress func(msg string),
) (memberSet map[string]bool, skippedConvs int, err error)
```

The `progress` callback replaces the direct spinner/fmt.Printf calls, keeping the function independent of the spinner implementation.

### 2. Extract fetchUserProfiles from runDiscover

The user-info fetching loop (lines 200-252) iterates member IDs, skips already-known users, calls `GetUserInfo`, and accumulates `*slackapi.User` results. This is a self-contained block with cx≈8.

```go
// fetchUserProfiles fetches user profiles for the given member IDs, skipping
// any IDs present in the skip map. Returns fetched users and skip count.
func fetchUserProfiles(
    ctx context.Context,
    client *slackapi.Client,
    memberIDs map[string]bool,
    skip map[string]config.PersonConfig,
    progress func(msg string),
) (users []*slackapi.User, skipped int, err error)
```

### 3. Extract writePeopleJSON from runDiscover

The merge-and-write block (lines 254-278) converts users, merges with existing, marshals JSON, and writes the file. This is a pure function with cx≈4.

```go
// writePeopleJSON merges new users with existing people and writes the result.
func writePeopleJSON(
    path string,
    fetchedUsers []*slackapi.User,
    existingPeople map[string]config.PersonConfig,
    merge bool,
) (int, error)
```

### 4. Extract resolveExportFolderID from runExport

The folder ID resolution logic (lines 114-119) checks CLI flag → settings.FolderID → settings.GoogleDriveFolderID. Currently 3 if-statements, but extracting it makes the intent clear and reduces runExport's branching.

```go
// resolveExportFolderID determines the Google Drive folder ID from flags and settings.
func resolveExportFolderID(flagValue string, settings *config.Settings) string
```

### 5. Extract loadExportDependencies from runExport

The config-loading block (lines 121-167) loads conversations, loads people, selects conversations, and prints the summary. This is the largest extractable block with cx≈10.

```go
type exportDeps struct {
    toExport []config.ConversationConfig
    people   *config.PeopleConfig
    folderID string
}

// loadExportDependencies loads configs, selects conversations, and resolves
// the folder ID. Returns the dependencies needed for export.
func loadExportDependencies(
    configDir string,
    args []string,
    folderID, folderName, userMapping string,
    allDMs, allGroups, debug bool,
    w io.Writer,
) (*exportDeps, error)
```

### 6. Extract parseDateRange from runExport

The date flag parsing block (lines 217-236) parses --from and --to flags into Slack timestamps and adjusts --to to end-of-day. Currently cx≈5 but extracting it makes runExport cleaner.

```go
// parseDateRange converts --from and --to flag values into Slack timestamp strings.
// The --to value is adjusted to end-of-day (23:59:59).
func parseDateRange(from, to string) (dateFrom, dateTo string, err error)
```

### 7. Test strategy

Each extracted function gets table-driven tests with mock data. No external services needed:
- `collectChannelMembers`: Mock via the `parser.SlackAPI` interface pattern (but the function takes `*slackapi.Client` directly — accept this limitation for now)
- `fetchUserProfiles`: Same limitation
- `writePeopleJSON`: Pure function, fully testable with temp dirs
- `resolveExportFolderID`: Pure function, fully testable
- `loadExportDependencies`: Needs config files on disk, testable with temp dirs
- `parseDateRange`: Pure function, fully testable

## Risks / Trade-offs

### Residual handler complexity

Even after extraction, `runDiscover` will still connect to Chrome, create a Slack client, load existing people, and manage the spinner lifecycle. This is ~15 lines of orchestration with cx≈10-15. Similarly, `runExport` will still manage the exporter lifecycle and format the summary. The goal is ≤15, not zero.

### Parameter count on extracted functions

Some extracted functions have 5+ parameters. This is acceptable for internal functions — they're not part of a public API, and the parameters make dependencies explicit.
