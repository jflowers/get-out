## ADDED Requirements

### Requirement: Zero-Coverage Google Drive Function Tests

All testable zero-coverage functions in `pkg/gdrive` MUST have tests covering the primary success path and at least one error path.

#### Scenario: DefaultConfig returns correct paths
- **GIVEN** a config directory path `"/home/user/.get-out"`
- **WHEN** `DefaultConfig("/home/user/.get-out")` is called
- **THEN** `CredentialsPath` MUST equal `"/home/user/.get-out/credentials.json"` AND `TokenPath` MUST equal `"/home/user/.get-out/token.json"`

#### Scenario: GetFolder returns folder info
- **GIVEN** a test server returning a file with `mimeType=application/vnd.google-apps.folder`
- **WHEN** `GetFolder` is called
- **THEN** the returned `FolderInfo` MUST have non-empty `ID`, `Name`, and `URL` fields

#### Scenario: GetFolder rejects non-folder
- **GIVEN** a test server returning a file with `mimeType=application/pdf`
- **WHEN** `GetFolder` is called
- **THEN** an error MUST be returned

#### Scenario: FindOrCreateDocument creates when not found
- **GIVEN** a test server where FindDocument returns no results
- **WHEN** `FindOrCreateDocument` is called
- **THEN** a create request MUST be sent AND the returned `DocInfo` MUST have a non-empty `ID`

#### Scenario: FindOrCreateDocument returns existing
- **GIVEN** a test server where FindDocument returns a document
- **WHEN** `FindOrCreateDocument` is called
- **THEN** no create request MUST be sent AND the returned `DocInfo.ID` MUST match the found document

#### Scenario: FindOrCreateFolder creates when not found
- **GIVEN** a test server where FindFolder returns no results
- **WHEN** `FindOrCreateFolder` is called
- **THEN** a create request MUST be sent AND the returned `FolderInfo` MUST have a non-empty `ID`

#### Scenario: CreateNestedFolders creates three levels
- **GIVEN** a test server that returns "not found" for all folder lookups
- **WHEN** `CreateNestedFolders` is called with 3 folder names and a parent ID
- **THEN** 3 folder create requests MUST be sent AND each MUST have the previous folder's ID as parent

#### Scenario: DeleteFolder trashes the folder
- **GIVEN** a test server accepting PATCH requests
- **WHEN** `DeleteFolder` is called
- **THEN** a PATCH request MUST be sent to `/files/{id}` with `trashed: true`

#### Scenario: ShareFolder sets reader permission
- **GIVEN** a test server accepting permission creation requests
- **WHEN** `ShareFolder` is called with an email
- **THEN** a POST to `/files/{id}/permissions` MUST include `role=reader` and `type=user`

#### Scenario: ShareFolderWithWriter sets writer permission
- **GIVEN** a test server accepting permission creation requests
- **WHEN** `ShareFolderWithWriter` is called with an email
- **THEN** a POST to `/files/{id}/permissions` MUST include `role=writer` and `type=user`

#### Scenario: UploadFile uploads with correct parent
- **GIVEN** a test server accepting multipart upload
- **WHEN** `UploadFile` is called with content, name, mimeType, and parentID
- **THEN** the returned file ID MUST be non-empty AND no error MUST be returned

#### Scenario: GetWebContentLink returns download URL
- **GIVEN** a test server returning a file with `webContentLink`
- **WHEN** `GetWebContentLink` is called
- **THEN** the returned string MUST equal the server's `webContentLink` value

#### Scenario: MakePublic sets anyone-reader permission
- **GIVEN** a test server accepting permission creation requests
- **WHEN** `MakePublic` is called
- **THEN** a POST to `/files/{id}/permissions` MUST include `type=anyone` and `role=reader`

#### Scenario: DeleteFile sends DELETE request
- **GIVEN** a test server accepting DELETE requests
- **WHEN** `DeleteFile` is called
- **THEN** a DELETE request MUST be sent to `/files/{id}` AND no error MUST be returned

#### Scenario: DeleteFile handles API error
- **GIVEN** a test server returning HTTP 500
- **WHEN** `DeleteFile` is called
- **THEN** an error MUST be returned

### Requirement: Confidence-79 Q3 Function Tests

Functions at confidence 79 MUST have targeted tests that cover their specific identified gap.

#### Scenario: LoadConversations with corrupt JSON
- **GIVEN** a file containing invalid JSON
- **WHEN** `LoadConversations` is called
- **THEN** an error MUST be returned AND the config MUST be nil

#### Scenario: LoadConversations with missing file
- **GIVEN** a path to a nonexistent file
- **WHEN** `LoadConversations` is called
- **THEN** an error MUST be returned AND the config MUST be nil

#### Scenario: DefaultSettings returns distinct instances
- **GIVEN** two calls to `DefaultSettings()`
- **WHEN** the first instance's `LogLevel` is mutated
- **THEN** the second instance's `LogLevel` MUST still be `"INFO"`

#### Scenario: ResolveWithFallback concurrent access
- **GIVEN** a `UserResolver` with a mock API
- **WHEN** 20 goroutines call `ResolveWithFallback` concurrently with different user IDs
- **THEN** no panics MUST occur AND every result MUST be non-empty

#### Scenario: RecordRateLimit observable effect
- **GIVEN** a RateLimiter with a 50ms interval for endpoint "fast"
- **WHEN** `RecordRateLimit("fast", 0)` is called (doubling the interval)
- **THEN** the subsequent `Wait("fast")` MUST take at least 80ms (verifying the doubled interval)

#### Scenario: ChannelResolver.Resolve contract assertions
- **GIVEN** a `ChannelResolver` with channel C123 mapped to "general"
- **WHEN** `Resolve("C123")` is called THEN it MUST return `"general"`
- **WHEN** `Resolve("C_UNKNOWN")` is called THEN it MUST return `"C_UNKNOWN"` (raw ID)
- **WHEN** `Resolve("")` is called THEN it MUST return `""`

## MODIFIED Requirements

_None. All existing test behavior is preserved._

## REMOVED Requirements

_None._
