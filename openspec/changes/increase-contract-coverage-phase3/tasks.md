## 1. Google Drive — Zero-Coverage Function Tests (`pkg/gdrive`)

- [x] 1.1 Add `TestDefaultConfig` — assert `CredentialsPath` and `TokenPath` are correctly joined from config directory
- [x] 1.2 Add `TestGetFolder_Success` — test server returns folder metadata with `mimeType=application/vnd.google-apps.folder`; assert `FolderInfo.ID`, `Name`, and `URL` are populated
- [x] 1.3 Add `TestGetFolder_NotAFolder` — test server returns file with `mimeType=application/pdf`; assert error returned
- [x] 1.4 Add `TestFindOrCreateDocument_CreatesWhenNotFound` — test server returns empty list for FindDocument, then accepts create; assert `DocInfo.ID` is non-empty
- [x] 1.5 Add `TestFindOrCreateDocument_ReturnsExisting` — test server returns existing doc for FindDocument; assert no create request is made; assert returned `DocInfo.ID` matches
- [x] 1.6 Add `TestFindOrCreateFolder_CreatesWhenNotFound` — test server returns empty for find, accepts create; assert `FolderInfo.ID` non-empty
- [x] 1.7 Add `TestCreateNestedFolders_ThreeLevels` — test server tracks create calls; assert 3 create requests with chained parent IDs
- [x] 1.8 Add `TestDeleteFolder_Success` — test server accepts PATCH; assert request method is PATCH; assert no error
- [x] 1.9 Add `TestShareFolder_ReaderPermission` — test server captures POST to permissions endpoint; assert `role=reader` and `type=user` in request body
- [x] 1.10 Add `TestShareFolderWithWriter_WriterPermission` — assert `role=writer` and `type=user` in request body
- [x] 1.11 Add `TestUploadFile_Success` — test server handles multipart upload; assert returned file ID is non-empty
- [x] 1.12 Add `TestGetWebContentLink_Success` — test server returns `webContentLink`; assert returned string matches
- [x] 1.13 Add `TestMakePublic_AnyoneReader` — test server captures permission request; assert `type=anyone` and `role=reader`
- [x] 1.14 Add `TestDeleteFile_Success` — test server accepts DELETE; assert request method is DELETE; assert no error
- [x] 1.15 Add `TestDeleteFile_APIError` — test server returns HTTP 500; assert error returned
- [x] 1.16 Run `go test -race -count=1 ./pkg/gdrive/...` — all tests MUST pass

## 2. Config — Confidence-79 Q3 Tests (`pkg/config`)

- [x] 2.1 Add `TestLoadConversations_CorruptJSON` — write invalid JSON to temp file; assert error returned and config is nil
- [x] 2.2 Add `TestLoadConversations_FileNotFound` — pass nonexistent path; assert error returned and config is nil
- [x] 2.3 Add `TestDefaultSettings_DistinctInstances` — call twice; mutate first instance's LogLevel; assert second instance unchanged
- [x] 2.4 Run `go test -race -count=1 ./pkg/config/...` — all tests MUST pass

## 3. Parser — Confidence-79 Q3 Tests (`pkg/parser`)

- [x] 3.1 Add `TestResolveWithFallback_ConcurrentAccess` — 20 goroutines calling ResolveWithFallback with different user IDs; assert no panics and all results non-empty
- [x] 3.2 Add `TestChannelResolver_Resolve_ContractAssertions` — assert unknown ID returns raw ID; known ID returns name; empty string returns empty string; no result is ever returned that the test didn't expect
- [x] 3.3 Run `go test -race -count=1 ./pkg/parser/...` — all tests MUST pass

## 4. Slack API — Confidence-79 Q3 Tests (`pkg/slackapi`)

- [x] 4.1 Add `TestRecordRateLimit_ObservableEffect` — call `Wait`, then `RecordRateLimit("fast", 0)` to double interval, then `Wait` again; assert second Wait takes ≥ 80ms (proving the interval mutation is observable through the public API)
- [x] 4.2 Run `go test -race -count=1 ./pkg/slackapi/...` — all tests MUST pass

## 5. Verification

- [x] 5.1 Run full test suite: `go test -race -count=1 ./...` — all tests MUST pass with no regressions
- [x] 5.2 Run `gaze crap --format=json ./pkg/gdrive` — verify line coverage increased above 70% and CRAPload decreased
- [x] 5.3 Run `gaze crap --format=json` per-package for config, parser, slackapi — verify Q3 counts for confidence-79 functions
- [x] 5.4 Verify no production code was modified — only `_test.go` files in `git diff --name-only`
- [x] 5.5 Verify constitution alignment: Testability (§IV) — all 12 gdrive functions tested in isolation via httptest without external services
