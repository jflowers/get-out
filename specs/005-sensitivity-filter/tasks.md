# Tasks: Sensitivity Filter for Local Markdown Export

**Input**: Design documents from `/specs/005-sensitivity-filter/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, quickstart.md ✅

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Exact file paths included in all descriptions

---

## Phase 1: Setup

**Purpose**: Create the new `pkg/ollama/` package directory for Ollama client code.

- [x] T001 Create `pkg/ollama/` package directory and add a `doc.go` file with package comment: `// Package ollama provides an HTTP client for the Ollama REST API and a Granite Guardian sensitivity classifier.`

**Checkpoint**: Package directory exists, `go build ./...` still passes.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented. This phase builds the Ollama HTTP client and extends the config model.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T002 [P] Implement Ollama HTTP client in `pkg/ollama/client.go` — Define the `Client` struct with fields `endpoint string`, `model string`, `httpClient *http.Client`. Implement `NewClient(endpoint, model string, opts ...Option) *Client` with functional options pattern (at minimum an `WithHTTPClient(*http.Client)` option for test injection). Implement `Generate(ctx context.Context, prompt string) (string, error)` that POSTs to `/api/generate` with `ollamaGenerateRequest{Model, Prompt, Stream: false, Format: "json"}` and parses `ollamaGenerateResponse` to return the `Response` field. Implement `Ping(ctx context.Context) error` that does `GET /` and returns nil on 200. Implement `ModelAvailable(ctx context.Context) (bool, error)` that does `GET /api/tags` and checks if `c.model` is in the returned models list. Define unexported request/response structs per data-model.md (`ollamaGenerateRequest`, `ollamaGenerateResponse`, `ollamaTagsResponse`). Use 60-second timeout on the HTTP client. Wrap all errors with `fmt.Errorf("ollama: <context>: %w", err)`.

- [x] T003 [P] Add `OllamaConfig` struct to `pkg/config/types.go` — Add the `OllamaConfig` struct with fields `Enabled bool`, `Endpoint string` (json tag `endpoint,omitempty`), `Model string` (json tag `model,omitempty`). Add constants `DefaultOllamaEndpoint = "http://localhost:11434"` and `DefaultOllamaModel = "granite-guardian:8b"`. Add `Ollama *OllamaConfig` field to the existing `Settings` struct with json tag `json:"ollama,omitempty"`. Do NOT modify `DefaultSettings()` — Ollama remains nil by default (opt-in feature).

- [x] T004 [P] Write Ollama HTTP client tests in `pkg/ollama/client_test.go` — Use `net/http/httptest` to create mock Ollama servers. Test cases: (1) `TestGenerate_Success` — mock `/api/generate` returning valid `ollamaGenerateResponse` JSON, verify returned response string. (2) `TestGenerate_OllamaUnreachable` — use an invalid endpoint, verify error contains "connection refused" or similar. (3) `TestGenerate_NonOKStatus` — mock returning HTTP 500, verify error. (4) `TestPing_Success` — mock `/` returning 200, verify nil error. (5) `TestPing_Unreachable` — verify error on connection failure. (6) `TestModelAvailable_Found` — mock `/api/tags` returning a models list that includes the configured model, verify returns `true, nil`. (7) `TestModelAvailable_NotFound` — mock `/api/tags` returning a models list without the configured model, verify returns `false, nil`. (8) `TestNewClient_WithHTTPClient` — verify the `WithHTTPClient` option injects the custom client.

- [x] T005 [P] Write OllamaConfig settings tests in `pkg/config/config_test.go` — Add test cases to the existing test file: (1) `TestLoadSettings_WithOllamaConfig` — write a `settings.json` with the `ollama` block (`enabled: true, endpoint, model`), load it, assert `settings.Ollama` is non-nil and all fields match. (2) `TestLoadSettings_WithoutOllamaConfig` — write a `settings.json` without the `ollama` key, load it, assert `settings.Ollama` is nil. (3) `TestLoadSettings_OllamaOmitemptyDefaults` — write a `settings.json` with `{"ollama": {"enabled": true}}` (no endpoint/model), load it, assert `Endpoint` and `Model` are empty strings (caller is responsible for applying defaults). (4) `TestSettings_MarshalOllamaOmitempty` — marshal a `Settings` with `Ollama: nil`, assert the JSON does not contain `"ollama"`.

**Checkpoint**: `go test -race -count=1 ./pkg/ollama/... ./pkg/config/...` passes. Foundation ready — user story implementation can now begin.

---

## Phase 3: User Story 1 — Sensitive Messages Excluded from Markdown Export (Priority: P1) 🎯 MVP

**Goal**: Messages classified as sensitive by the Ollama Granite Guardian model are excluded from local markdown files. Google Docs export is unaffected. YAML frontmatter includes sensitivity metadata.

**Independent Test**: Export a conversation containing a mix of normal and sensitive messages with sensitivity filtering enabled. Verify the markdown file contains only non-sensitive messages and includes a `sensitivity:` frontmatter section with `filtered_count` and `categories`.

### Implementation for User Story 1

- [x] T006 [US1] Implement Guardian classifier in `pkg/ollama/guardian.go` — Define sensitivity category constants (`CategoryHR`, `CategoryLegal`, `CategoryFinancial`, `CategoryHealth`, `CategoryTermination`, `CategoryNone`) and `ValidCategories` map per data-model.md. Define the exported `SensitivityResult` struct per data-model.md. Implement `Guardian` struct with a `client *Client` field and constructor `NewGuardian(client *Client) *Guardian`. Implement `Classify(ctx context.Context, messages []string) ([]SensitivityResult, error)` that: (1) builds the batch classification prompt per research.md Decision 2 (numbered messages, JSON array response format, sensitivity categories, err-on-caution instruction per FR-014, individual-vs-policy distinction per FR-015), (2) calls `client.Generate(ctx, prompt)`, (3) parses the JSON response (handle potential markdown code fences around JSON), (4) validates result count matches input count and categories are valid, (5) on parse/validation failure: retries once with the same prompt (FR-009), (6) on second failure: returns all messages as sensitive with `CategoryNone` and `Confidence: 0.0` (err on caution). Messages with empty text should be returned as non-sensitive with `CategoryNone` (FR-013).

- [x] T007 [US1] Write Guardian classifier tests in `pkg/ollama/guardian_test.go` — Test cases: (1) `TestClassify_Success` — mock client returning valid JSON array of SensitivityResults, verify correct parsing and field values. (2) `TestClassify_PromptConstruction` — capture the prompt sent to Generate, verify it contains numbered messages `[1]`, `[2]`, etc., the sensitivity categories, and the err-on-caution instruction. (3) `TestClassify_CategoryValidation` — mock response with invalid category, verify retry and fallback to all-sensitive. (4) `TestClassify_MalformedJSON_RetryOnce` — mock first call returning invalid JSON, second call returning valid JSON, verify retry succeeds. (5) `TestClassify_MalformedJSON_TwiceFallsBack` — mock both calls returning invalid JSON, verify all messages returned as sensitive. (6) `TestClassify_WrongResultCount` — mock response with fewer results than messages, verify retry and fallback. (7) `TestClassify_EmptyTextMessage` — include an empty string in messages, verify it's returned as non-sensitive (FR-013). (8) `TestClassify_MarkdownCodeFence` — mock response wrapped in ````json ... ``` ```, verify JSON is extracted correctly. (9) `TestClassify_FR015_PolicyVsIndividual` — golden-file test validating the prompt produces correct classifications for FR-015 examples: "updating the PTO policy" → not sensitive, "Sarah's excessive absences" → sensitive. Mock the Ollama response to return the expected classifications and verify the prompt contains the individual-vs-policy distinction instruction. (10) `TestClassify_EmptyBatch` — call `Classify` with an empty `[]string{}`, verify it returns an empty `[]SensitivityResult` without making an Ollama call. Use a test helper that creates a `Client` with an `httptest.Server` to avoid mocking the `Client` struct directly.

- [x] T008 [US1] Implement MessageFilter interface and OllamaFilter in `pkg/exporter/sensitivity.go` — Define the `MessageFilter` interface with method `FilterMessages(ctx context.Context, messages []slackapi.Message) (*FilterResult, error)` per data-model.md. Define the `FilterResult` struct per data-model.md with fields `PassedMessages []slackapi.Message`, `FilteredCount int`, `CategoryBreakdown map[string]int`, `TotalCount int`, `Results []ollama.SensitivityResult`, and method `AllFiltered() bool`. Implement `OllamaFilter` struct with field `guardian *ollama.Guardian` and constructor `NewOllamaFilter(guardian *ollama.Guardian) *OllamaFilter`. Implement `FilterMessages`: (1) extract text from each `slackapi.Message` (use `msg.Text` field; treat empty text as non-sensitive per FR-013), (2) call `guardian.Classify(ctx, texts)`, (3) build `FilterResult` — populate `PassedMessages` with messages where `result.Sensitive == false`, compute `FilteredCount`, build `CategoryBreakdown` from sensitive results, set `TotalCount` to `len(messages)`.

- [x] T009 [US1] Write SensitivityFilter tests in `pkg/exporter/sensitivity_test.go` — Create a `mockGuardian` test helper (or use a mock `MessageFilter` implementation). Test cases: (1) `TestOllamaFilter_MixedMessages` — 5 messages, 2 sensitive, verify `PassedMessages` has 3, `FilteredCount` is 2, `CategoryBreakdown` is correct. (2) `TestOllamaFilter_AllSensitive` — all messages sensitive, verify `AllFiltered()` returns true, `PassedMessages` is empty. (3) `TestOllamaFilter_NoneSensitive` — no messages sensitive, verify `FilteredCount` is 0, `PassedMessages` equals input. (4) `TestOllamaFilter_EmptyTextMessage` — message with empty `Text` field, verify it passes through as non-sensitive (FR-013). (5) `TestOllamaFilter_ClassifyError` — guardian returns error, verify `FilterMessages` returns error (hard gate). (6) `TestFilterResult_AllFiltered` — unit test the `AllFiltered()` method directly. (7) `TestOllamaFilter_EmptyBatch` — call `FilterMessages` with an empty `[]slackapi.Message{}`, verify it returns a `FilterResult` with `FilteredCount: 0`, empty `PassedMessages`, and no error.

- [x] T010 [US1] Update `RenderDailyDoc` in `pkg/exporter/mdwriter.go` to accept optional `*FilterResult` — Change the signature of `RenderDailyDoc` to add a regular parameter: `func (w *MarkdownWriter) RenderDailyDoc(convName string, convType string, date string, messages []slackapi.Message, filterResult *FilterResult) ([]byte, error)`. Callers pass `nil` when no filtering is active. When a non-nil `FilterResult` is provided, add a `sensitivity:` YAML block to the frontmatter after the `participants:` block, containing `filtered_count: N` and a `categories:` sub-block listing each category with count > 0 (per research.md Decision 6). When `filterResult` is nil, the frontmatter is unchanged (backward compatible). Include `filtered_count: 0` when filtering ran but found nothing sensitive (audit trail).

- [x] T011 [US1] Update mdwriter tests in `pkg/exporter/mdwriter_test.go` — Add test cases: (1) `TestRenderDailyDoc_WithFilterResult` — pass a `FilterResult` with `FilteredCount: 3` and `CategoryBreakdown: {"hr": 2, "financial": 1}`, verify the YAML frontmatter contains the `sensitivity:` block with correct values. (2) `TestRenderDailyDoc_WithFilterResultZeroFiltered` — pass a `FilterResult` with `FilteredCount: 0`, verify `sensitivity:` block is present with `filtered_count: 0` and no `categories:` sub-block. (3) `TestRenderDailyDoc_WithNilFilterResult` — pass `nil` as the `filterResult` parameter, verify no `sensitivity:` block in frontmatter (backward compatibility). Update all existing `RenderDailyDoc` call sites in tests to pass `nil` as the new `filterResult` parameter.

- [x] T012 [US1] Integrate sensitivity filter into export loop in `pkg/exporter/exporter.go` — Add a `messageFilter MessageFilter` field to the `Exporter` struct. Add a `MessageFilter MessageFilter` field to `ExporterConfig`. In `NewExporter`, set `messageFilter` from `cfg.MessageFilter`. In the `ExportConversation` method, modify the local markdown write block (the section that calls `RenderDailyDoc` for local export) to: (1) if `e.messageFilter != nil && conv.LocalExport`, call `e.messageFilter.FilterMessages(ctx, msgs)` before `RenderDailyDoc`, (2) if `filterResult.AllFiltered()`, skip writing the markdown file for this date (FR-010) and log via `e.Progress`, (3) pass `filterResult` to `RenderDailyDoc` as the `*FilterResult` parameter, (4) use `filterResult.PassedMessages` instead of `msgs` for the markdown content. When `e.messageFilter` is nil, pass `nil` for `filterResult` and the existing unfiltered behavior is preserved. Google Docs export path (the section that calls `CreateOrUpdateDoc`) remains completely untouched (FR-006).

- [x] T012a [US1] Write export integration tests in `pkg/exporter/exporter_test.go` — Create an integration test that wires a mock `httptest.Server` as the Ollama backend through the full export pipeline. Test cases: (1) `TestExportConversation_WithSensitivityFilter` — configure an `Exporter` with an `OllamaFilter` backed by a mock Ollama server, export a conversation with mixed messages, verify the written markdown file excludes sensitive messages and includes the `sensitivity:` frontmatter block. (2) `TestExportConversation_GoogleDocsUnaffected` — configure an `Exporter` with an active `MessageFilter`, export a conversation with `localExport: true`, verify the Google Docs export path receives all messages unfiltered (FR-006). (3) `TestExportConversation_ExistingFileNotReprocessed` — pre-create a markdown file for a date, run export with filtering enabled, verify the existing file is not re-processed or re-filtered (FR-012).

**Checkpoint**: `go test -race -count=1 ./pkg/ollama/... ./pkg/exporter/...` passes. User Story 1 is independently functional: sensitivity classification, filtering, frontmatter enrichment, and export integration are complete.

---

## Phase 4: User Story 2 — Export Fails When Ollama Is Unavailable (Priority: P1)

**Goal**: When sensitivity filtering is configured and Ollama is unreachable or the model is unavailable, the export fails immediately with a clear, actionable error message. No silent fallback to unfiltered export.

**Independent Test**: Configure sensitivity filtering with an unreachable Ollama endpoint, run the export, verify it fails with an actionable error before any files are written.

### Implementation for User Story 2

- [x] T013 [US2] Add Ollama pre-export health check in `internal/cli/export.go` — Extract a standalone testable function `validateOllamaPrerequisites(ctx context.Context, client *ollama.Client) error` that: (1) calls `client.Ping(ctx)` — on failure, returns an error with the format from research.md Decision 3 (endpoint, suggestion to start Ollama or use `--no-sensitivity-filter`), (2) calls `client.ModelAvailable(ctx)` — on failure, returns an error with the model name and suggestion to `ollama pull <model>`. Then, in the `runExport` function, after loading settings and resolving the local export directory (the section that builds `ExporterConfig`), add a sensitivity filter initialization block: (1) determine if sensitivity filtering is active: `settings.Ollama != nil && settings.Ollama.Enabled && !noSensitivityFilter` (where `noSensitivityFilter` is the flag from Phase 5 — for now, hardcode as `false`; Phase 5 will wire the flag), (2) resolve the Ollama endpoint using the priority chain: `--ollama-endpoint` flag > `settings.Ollama.Endpoint` > `DefaultOllamaEndpoint`, (3) resolve the model: `settings.Ollama.Model` or `DefaultOllamaModel` if empty, (4) create `ollama.NewClient(endpoint, model)`, (5) call `validateOllamaPrerequisites(ctx, client)` — on failure, return the error, (6) create `ollama.NewGuardian(client)` and `exporter.NewOllamaFilter(guardian)`, (7) set the filter on `ExporterConfig.MessageFilter`. This ensures SC-002: fail within 5 seconds before any files are written.

- [x] T014 [US2] Write pre-export health check tests — Add tests in `internal/cli/export_test.go` (or a new test file if export_test.go doesn't exist) targeting the extracted `validateOllamaPrerequisites` function: (1) `TestValidateOllamaPrerequisites_Unreachable` — create a client with an unreachable endpoint, verify the returned error mentions the endpoint and `--no-sensitivity-filter`. (2) `TestValidateOllamaPrerequisites_ModelNotAvailable` — mock Ollama reachable but model not in tags list, verify error mentions the model name and `ollama pull`. (3) `TestValidateOllamaPrerequisites_AllOK` — mock Ollama reachable and model available, verify nil error. (4) `TestExportPrecheck_OllamaDisabled` — configure settings with `Ollama.Enabled: false`, verify no Ollama calls are made. (5) `TestExportPrecheck_OllamaNotConfigured` — settings with `Ollama: nil`, verify no Ollama calls are made. (6) `TestEndpointResolution_FlagOverridesSettings` — verify that when `--ollama-endpoint` flag is set, it takes priority over `settings.Ollama.Endpoint`. (7) `TestEndpointResolution_SettingsOverridesDefault` — verify that when `settings.Ollama.Endpoint` is set and no flag is provided, it takes priority over `DefaultOllamaEndpoint`. (8) `TestEndpointResolution_DefaultFallback` — verify that when neither flag nor settings endpoint is set, `DefaultOllamaEndpoint` is used.

- [x] T015 [US2] Handle mid-export Ollama failure in `pkg/exporter/exporter.go` — In the `ExportConversation` method, when `e.messageFilter.FilterMessages(ctx, msgs)` returns an error (Ollama became unreachable mid-export), do NOT continue with unfiltered export. Instead, return the error immediately from `ExportConversation` with context: `fmt.Errorf("sensitivity classification failed for %q (%s): %w", conv.Name, date, err)`. The caller (`ExportAll` or the CLI loop) will handle reporting which conversations succeeded before the failure. This satisfies US2-AS3. Verify that conversations exported before the failure are already checkpointed (they are — the index is saved after each date's Google Docs write).

**Checkpoint**: `go test -race -count=1 ./internal/cli/... ./pkg/exporter/...` passes. Export fails immediately when Ollama is unreachable, fails when model is missing, and stops mid-export if Ollama becomes unavailable.

---

## Phase 5: User Story 3 — --no-sensitivity-filter Flag (Priority: P2)

**Goal**: Users can bypass sensitivity filtering for a single export run using a CLI flag, without changing persistent settings.

**Independent Test**: Configure sensitivity filtering, run export with `--no-sensitivity-filter`, verify all messages appear in markdown without Ollama calls.

### Implementation for User Story 3

- [x] T016 [US3] Add `--no-sensitivity-filter` and `--ollama-endpoint` flags to `internal/cli/export.go` — Add package-level variables: `exportNoSensitivityFilter bool` and `exportOllamaEndpoint string`. In the `init()` function, add: `exportCmd.Flags().BoolVar(&exportNoSensitivityFilter, "no-sensitivity-filter", false, "Disable sensitivity filtering for this run (overrides settings)")` and `exportCmd.Flags().StringVar(&exportOllamaEndpoint, "ollama-endpoint", "", "Override Ollama endpoint URL for this run")`. In the `runExport` function, wire `exportNoSensitivityFilter` into the sensitivity filter initialization block from T013 — when `true`, skip all Ollama client creation and set `MessageFilter` to nil. Wire `exportOllamaEndpoint` into the endpoint resolution chain (flag > settings > default) from T013.

**Checkpoint**: `go build ./cmd/get-out && ./get-out export --help` shows the new flags. `go test -race -count=1 ./internal/cli/...` passes.

---

## Phase 6: User Story 4 — Doctor Health Checks for Ollama (Priority: P3)

**Goal**: `get-out doctor` checks Ollama reachability and model availability when sensitivity filtering is configured, with clear pass/fail output.

**Independent Test**: Run `get-out doctor` with Ollama running/stopped, model present/absent, and filtering unconfigured. Verify correct health check output for each scenario.

### Implementation for User Story 4

- [x] T017 [US4] Add Ollama health checks to doctor command in `internal/cli/selfservice.go` — Add a new function `checkOllama(endpoint, model string, pass_, warn_, fail_ *int)` that: (1) creates an `ollama.Client` with the given endpoint and model, (2) calls `client.Ping(ctx)` — on success, call `pass("Ollama: OK (" + endpoint + ")")`, on failure call `fail("Ollama: FAIL — not reachable at " + endpoint)` and `hint("Start Ollama: ollama serve")` then return early, (3) calls `client.ModelAvailable(ctx)` — on success call `pass("Sensitivity model: OK (" + model + ")")`, on failure call `fail("Sensitivity model: FAIL — \"" + model + "\" not found")` and `hint("Pull the model: ollama pull " + model)`. In `runDoctor`, after the last existing health check (background sync service check), add a conditional block: load settings, if `settings.Ollama != nil && settings.Ollama.Enabled`, resolve endpoint (settings or default) and model (settings or default), then call `checkOllama(endpoint, model, &passCount, &warnCount, &failCount)`. When sensitivity filtering is not configured, do not display any Ollama checks (US4-AS3: no noise for unconfigured features).

- [x] T018 [US4] Write doctor Ollama health check tests — Add tests (in `internal/cli/selfservice_test.go` or a new file): (1) `TestCheckOllama_AllOK` — mock Ollama reachable and model available, verify pass count incremented by 2. (2) `TestCheckOllama_Unreachable` — use unreachable endpoint, verify fail count incremented by 1 (only Ollama check, model check skipped). (3) `TestCheckOllama_ModelMissing` — mock Ollama reachable but model not in tags, verify 1 pass (Ollama OK) and 1 fail (model). (4) `TestRunDoctor_OllamaNotConfigured` — verify no Ollama checks appear when `settings.Ollama` is nil. Note: `checkOllama` is a standalone function with counter pointers, making it testable without the full doctor context.

**Checkpoint**: `go test -race -count=1 ./internal/cli/...` passes. Doctor correctly reports Ollama status when configured.

---

## Phase 7: Polish & Documentation

**Purpose**: Update documentation to reflect the new sensitivity filter feature.

- [x] T019 [P] Update `README.md` — Add a "Sensitivity Filtering" section documenting: (1) prerequisites (Ollama installed, model pulled), (2) `settings.json` `ollama` configuration block with field descriptions, (3) export command usage with `--no-sensitivity-filter` and `--ollama-endpoint` flags, (4) example markdown output showing `sensitivity:` frontmatter, (5) error scenarios (Ollama unavailable, model missing), (6) FAQ items from quickstart.md (Google Docs unaffected, existing files not re-filtered, false positive preference).

- [x] T020 [P] Update `AGENTS.md` — Add `pkg/ollama/` to the Project Structure section under `pkg/` with description: `├── ollama/               # Ollama REST API client and Granite Guardian classifier`. Add `sensitivity.go` to the `pkg/exporter/` listing. Update the Active Technologies section to include `Ollama REST API (HTTP client, net/http)`. Add `--no-sensitivity-filter` and `--ollama-endpoint` to the export command examples.

**Checkpoint**: Documentation is accurate and complete.

---

## Phase 8: Verification

**Purpose**: Final CI-equivalent validation and constitution alignment check.

- [x] T021 Run full CI checks — Execute: `go build ./cmd/get-out && go vet ./... && go test -race -count=1 ./...`. All must pass with zero failures. Verify no new lint warnings. Check that `go build` produces a working binary and `./get-out export --help` shows the new flags.

- [x] T022 Verify constitution alignment — Re-check all nine constitution principles from plan.md against the final implementation: (I) Session-Driven Extraction unchanged, (II) Go-First Architecture — new code is pure Go, (III) Stealth & Reliability unchanged, (IV) Two-Tier Extraction unchanged, (V) Concurrency & Resilience — filter is per-daily-doc in sequential loop, (VI) Security First — no cloud classification, local-only Ollama, (VII) Output Format — Google Docs unaffected, markdown enhanced, (VIII) Google Drive Integration unchanged, (IX) Documentation Maintenance — README and AGENTS.md updated. Report any violations.

**Checkpoint**: All CI checks pass, constitution alignment confirmed. Feature is ready for review.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Phase 2 completion
- **User Story 2 (Phase 4)**: Depends on Phase 3 (needs MessageFilter and OllamaFilter)
- **User Story 3 (Phase 5)**: Depends on Phase 4 (needs the pre-check block to wire the flag into)
- **User Story 4 (Phase 6)**: Depends on Phase 2 only (uses ollama.Client directly, independent of US1-US3)
- **Polish (Phase 7)**: Depends on Phases 3-6 completion
- **Verification (Phase 8)**: Depends on all previous phases

### Parallel Opportunities

- **Phase 2**: T002, T003, T004, T005 can all run in parallel (different files, no dependencies)
- **Phase 3**: T006 must complete before T008 (T008 imports T006's types via `ollama.SensitivityResult` in `FilterResult`); T007 depends on T006; T009 depends on T008; T010-T012a are sequential (same files, cumulative changes)
- **Phase 6**: T017-T018 can run in parallel with Phases 3-5 (only depends on Phase 2)
- **Phase 7**: T019 and T020 can run in parallel (different files)

### Within Each User Story

- Types/interfaces before implementations
- Tests may follow implementations within the same phase (tests validate the contract surface)
- Core logic before integration (e.g., Guardian before OllamaFilter before export loop wiring)

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (core filtering)
4. Complete Phase 4: User Story 2 (hard gate)
5. **STOP and VALIDATE**: Test US1 + US2 independently
6. This delivers the minimum viable protection

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add User Story 1 → Sensitive messages excluded → MVP!
3. Add User Story 2 → Hard gate on Ollama availability → Safety complete
4. Add User Story 3 → CLI escape hatch → Practical usability
5. Add User Story 4 → Doctor checks → Setup experience
6. Polish + Verification → Documentation and final validation

---

## Notes

- [P] tasks = different files, no dependencies between them
- [US#] label maps task to specific user story for traceability
- All file paths are relative to repository root
- Google Docs export is NEVER modified (FR-006) — only the markdown path is affected
- The `MessageFilter` interface enables testing the export pipeline without a running Ollama instance
- Commit after each task or logical group
- Stop at any checkpoint to validate independently

<!-- spec-review: passed -->
<!-- code-review: passed -->
