## 1. Decompose runDiscover

- [x] 1.1 Extract `collectChannelMembers(ctx, client, conversations, progress) (memberSet, skippedConvs, err)` — the member-fetching loop (lines 109-172) that iterates conversations, skips DMs/MPIMs, paginates GetConversationMembers, and accumulates unique member IDs
- [x] 1.2 Extract `fetchUserProfiles(ctx, client, memberIDs, skip, progress) (users, skipped, err)` — the user-info fetching loop (lines 200-242) that calls GetUserInfo for each member, skips already-known users, and accumulates results
- [x] 1.3 Extract `writePeopleJSON(path, fetchedUsers, existingPeople, merge) (count, err)` — the merge + marshal + write block (lines 254-278) that converts users, merges with existing, and writes people.json
- [x] 1.4 Update `runDiscover` to call the three extracted functions, keeping only Chrome connection, Slack client creation, existing people loading, and spinner lifecycle as residual orchestration

## 2. Decompose runExport

- [x] 2.1 Extract `resolveExportFolderID(flagValue string, settings *config.Settings) string` — the 3-way folder ID resolution (flag → settings.FolderID → settings.GoogleDriveFolderID)
- [x] 2.2 Extract `parseDateRange(from, to string) (dateFrom, dateTo string, err error)` — the date flag parsing + end-of-day adjustment block (lines 217-236)
- [x] 2.3 Update `runExport` to call `resolveExportFolderID` and `parseDateRange`, removing the inline logic

## 3. Test Extracted Functions

- [x] 3.1 Test `collectChannelMembers` — verify it skips DMs/MPIMs, accumulates unique members, handles pagination, and calls progress callback (use mock `*slackapi.Client` via httptest)
- [x] 3.2 Test `fetchUserProfiles` — verify it skips known users, accumulates results, handles errors gracefully, and calls progress callback
- [x] 3.3 Test `writePeopleJSON` — verify merge behavior, JSON output format, file permissions (pure function, use temp dir)
- [x] 3.4 Test `resolveExportFolderID` — verify flag takes priority over settings, settings.FolderID over GoogleDriveFolderID, empty returns empty
- [x] 3.5 Test `parseDateRange` — verify from/to parsing, end-of-day adjustment, empty inputs, invalid date error

## 4. Verification

- [x] 4.1 Run full test suite: `go test -race -count=1 ./...` — all tests MUST pass
- [x] 4.2 Verify `runDiscover` complexity is ≤ 15 via gaze
- [x] 4.3 Verify `runExport` complexity is ≤ 15 via gaze
- [x] 4.4 Verify CRAPload decreased (should lose 2 entries: runDiscover and runExport)
