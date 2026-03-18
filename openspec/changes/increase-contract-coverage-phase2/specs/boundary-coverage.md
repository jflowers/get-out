## ADDED Requirements

### Requirement: Nil and Empty Input Boundary Tests

All Q3 functions that accept interface types, slices, or strings MUST have tests verifying behavior with nil/empty inputs.

#### Scenario: IsAuthError with nil error
- **GIVEN** a nil error value
- **WHEN** `IsAuthError(nil)` is called
- **THEN** it MUST return `false`

#### Scenario: IsAuthError with wrapped error
- **GIVEN** an `*AuthError` wrapped with `fmt.Errorf("%w", authErr)`
- **WHEN** `IsAuthError(wrappedErr)` is called
- **THEN** it MUST return `false` (function uses type assertion, not `errors.As`)

#### Scenario: IsNotFoundError with nil error
- **GIVEN** a nil error value
- **WHEN** `IsNotFoundError(nil)` is called
- **THEN** it MUST return `false`

#### Scenario: IsNotFoundError with wrapped error
- **GIVEN** a `*NotFoundError` wrapped with `fmt.Errorf`
- **WHEN** `IsNotFoundError(wrappedErr)` is called
- **THEN** it MUST return `false`

#### Scenario: TSToTime with empty string
- **GIVEN** an empty string `""`
- **WHEN** `TSToTime("")` is called
- **THEN** the result MUST have `.Unix() == 0` (Unix epoch)

#### Scenario: TSToTime with malformed input
- **GIVEN** a non-numeric string `"not.a.number"`
- **WHEN** `TSToTime("not.a.number")` is called
- **THEN** it MUST NOT panic AND the result MUST have `.Unix() == 0`

#### Scenario: FindSlackLinks with empty string
- **GIVEN** an empty string
- **WHEN** `FindSlackLinks("")` is called
- **THEN** it MUST return a zero-length result

#### Scenario: ReplaceSlackLinks with empty text
- **GIVEN** an empty string and any resolver function
- **WHEN** `ReplaceSlackLinks("", resolver)` is called
- **THEN** it MUST return `""`

#### Scenario: ReplaceSlackLinks identity passthrough
- **GIVEN** text containing no Slack URLs
- **WHEN** `ReplaceSlackLinks(text, resolver)` is called
- **THEN** the returned string MUST equal the input text unchanged

#### Scenario: ReplaceSlackLinks resolver receives correct arguments
- **GIVEN** text containing a Slack archive URL with channel C999 and timestamp p1706745603123456
- **WHEN** `ReplaceSlackLinks` is called with a capturing resolver
- **THEN** the resolver MUST receive `channelID="C999"` and `messageTS="1706745603.123456"`

#### Scenario: InsertFormattedContent with nil content
- **GIVEN** a nil `[]FormattedText` (not empty slice)
- **WHEN** `InsertFormattedContent` is called
- **THEN** no API call MUST be made AND the function MUST return nil

#### Scenario: LoadUsers with empty API response
- **GIVEN** a mock API that returns zero users
- **WHEN** `LoadUsers` is called
- **THEN** `Count()` MUST return 0 AND no error MUST be returned

#### Scenario: LoadUsersForConversations with nil channelIDs
- **GIVEN** nil (not empty slice) passed as channelIDs
- **WHEN** `LoadUsersForConversations` is called
- **THEN** no API calls MUST be made AND no error MUST be returned

#### Scenario: LoadChannels with empty API response
- **GIVEN** a mock API returning zero channels
- **WHEN** `LoadChannels` is called
- **THEN** `Resolve("C999")` MUST return `"C999"` (raw ID fallback)

### Requirement: Parameter Forwarding Verification Tests

Functions that forward caller-provided parameters to HTTP requests MUST have tests verifying the forwarding via request capture.

#### Scenario: GetAllMessages forwards channelID
- **GIVEN** a test HTTP server capturing form parameters
- **WHEN** `GetAllMessages` is called with channelID `"C_TARGET"`
- **THEN** the HTTP request MUST include `channel=C_TARGET`

#### Scenario: GetAllMessages terminates on HasMore with empty cursor
- **GIVEN** a server returning `has_more: true` with `next_cursor: ""`
- **WHEN** `GetAllMessages` is called
- **THEN** the callback MUST be invoked exactly once (no infinite pagination)

#### Scenario: GetAllReplies forwards channelID
- **GIVEN** a test HTTP server capturing form parameters
- **WHEN** `GetAllReplies` is called with channelID `"C_REPLIES"`
- **THEN** the HTTP request MUST include `channel=C_REPLIES`

#### Scenario: FindDocument with empty folderID
- **GIVEN** an empty string `""` as folderID
- **WHEN** `FindDocument` is called
- **THEN** the Drive API query MUST NOT contain `in parents`

#### Scenario: FindDocument with title containing single quotes
- **GIVEN** a title `"doc's title"` containing a single quote
- **WHEN** `FindDocument` is called
- **THEN** the Drive API query MUST escape the single quote (via `escapeName`)

### Requirement: Branch Path Exercise Tests

Untested branch paths in Q3 functions MUST be exercised to push confidence above 80.

#### Scenario: RateLimiter.Wait debug logging path
- **GIVEN** a RateLimiter with `SetDebug(true)`
- **WHEN** `Wait` is called twice for the same endpoint (triggering a delay)
- **THEN** the function MUST return nil (no error)

#### Scenario: DownloadFile in API mode
- **GIVEN** a client created via `NewAPIClient` (not browser mode)
- **WHEN** `DownloadFile` is called
- **THEN** the HTTP request MUST NOT include a `Cookie` header AND MUST include `Authorization: Bearer`

#### Scenario: AppendText with nil document body
- **GIVEN** a test server returning a document with no `body` field
- **WHEN** `AppendText` is called
- **THEN** text MUST be inserted at index 0 (fallback endIndex=1, insert at 1-1=0)

#### Scenario: AppendText with empty text
- **GIVEN** a document with existing content
- **WHEN** `AppendText(ctx, docID, "")` is called
- **THEN** an InsertText request with empty text MUST be sent (no short-circuit)

### Requirement: Overwrite and Reload Semantics Tests

Functions that populate caches via Load methods MUST verify second-call-overwrites behavior.

#### Scenario: LoadUsers overwrite on reload
- **GIVEN** a UserResolver that has loaded user U001 with name "old-name"
- **WHEN** `LoadUsers` is called again with the API returning U001 with name "new-name"
- **THEN** `Resolve("U001")` MUST return `"new-name"`

#### Scenario: LoadChannels overwrite on reload
- **GIVEN** a ChannelResolver that has loaded channel C1 with name "old"
- **WHEN** `LoadChannels` is called again with C1 named "new"
- **THEN** `Resolve("C1")` MUST return `"new"`

#### Scenario: NewPersonResolver with duplicate SlackIDs
- **GIVEN** a PeopleConfig with two entries sharing SlackID "U001"
- **WHEN** `NewPersonResolver` is called
- **THEN** `ResolveName("U001")` MUST return the last entry's DisplayName AND `Count()` MUST return 1

### Requirement: Auth Edge Case Tests

Authentication functions MUST verify edge cases around token freshness and credential parsing.

#### Scenario: ClientFromStore with expired token that has refresh token
- **GIVEN** a store containing valid credentials and an expired token with a non-empty refresh_token
- **WHEN** `ClientFromStore` is called
- **THEN** it MUST return a non-nil client with a configured Transport (not an error)

#### Scenario: InsertFormattedContent with Unicode text range computation
- **GIVEN** content containing emoji characters (UTF-16 surrogate pairs)
- **WHEN** `InsertFormattedContent` is called with bold formatting
- **THEN** the UpdateTextStyle range MUST use UTF-16 code unit length (emoji "😀" = 2 units)

## MODIFIED Requirements

_None. All existing test behavior is preserved._

## REMOVED Requirements

_None._
