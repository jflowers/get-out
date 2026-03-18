## ADDED Requirements

### Requirement: Google Docs Contract Assertions

Tests for `pkg/gdrive` document operation functions MUST assert on the contractual side effects of each function, not just error status.

#### Scenario: InsertFormattedContent italic formatting
- **GIVEN** a `Client` connected to a test Docs API server
- **WHEN** `InsertFormattedContent` is called with a content block that has `Italic: true`
- **THEN** the BatchUpdate request body MUST contain an `UpdateTextStyle` request with `TextStyle.Italic` set to `true` AND the field mask MUST include `italic`

#### Scenario: InsertFormattedContent monospace formatting
- **GIVEN** a `Client` connected to a test Docs API server
- **WHEN** `InsertFormattedContent` is called with a content block that has `Monospace: true`
- **THEN** the BatchUpdate request body MUST contain an `UpdateTextStyle` request with `TextStyle.WeightedFontFamily.FontFamily` set to `"Courier New"` AND the field mask MUST include `weightedFontFamily`

#### Scenario: InsertFormattedContent combined formatting
- **GIVEN** a `Client` connected to a test Docs API server
- **WHEN** `InsertFormattedContent` is called with a content block that has `Bold: true`, `Italic: true`, and `LinkURL` set
- **THEN** the BatchUpdate request body MUST contain a single `UpdateTextStyle` request with bold, italic, and link all set AND the field mask MUST include all three fields

#### Scenario: InsertFormattedContent multiple segments
- **GIVEN** a `Client` connected to a test Docs API server
- **WHEN** `InsertFormattedContent` is called with 3 content blocks of different formatting
- **THEN** the BatchUpdate request body MUST contain `InsertText` and `UpdateTextStyle` requests in reverse order (last segment first) AND each segment's index range MUST be correct

#### Scenario: InsertFormattedContent API error
- **GIVEN** a `Client` connected to a test Docs API server that returns HTTP 500
- **WHEN** `InsertFormattedContent` is called
- **THEN** the returned error MUST be non-nil

#### Scenario: BatchAppendMessages with image annotations
- **GIVEN** a `Client` connected to a test Docs API server
- **WHEN** `BatchAppendMessages` is called with a message containing an `ImageAnnotation` with a URL
- **THEN** the BatchUpdate request body MUST contain an `InsertInlineImage` request with the image URL AND the image MUST be preceded and followed by newline characters

#### Scenario: BatchAppendMessages with multiple messages
- **GIVEN** a `Client` connected to a test Docs API server
- **WHEN** `BatchAppendMessages` is called with 2 messages
- **THEN** both messages MUST be inserted AND the second message's insertion indices MUST account for the first message's length

#### Scenario: BatchAppendMessages with link text not in body
- **GIVEN** a `Client` connected to a test Docs API server
- **WHEN** `BatchAppendMessages` is called with a message whose `LinkAnnotation.Text` does not appear in the message body
- **THEN** the function MUST NOT produce an `UpdateTextStyle` request for that link AND MUST NOT error

#### Scenario: AppendText API error on Get
- **GIVEN** a `Client` connected to a test Docs API server where the Documents.Get endpoint returns HTTP 500
- **WHEN** `AppendText` is called
- **THEN** the returned error MUST be non-nil AND MUST wrap the underlying API error

#### Scenario: GetDocumentContent API error
- **GIVEN** a `Client` connected to a test Docs API server where the Documents.Get endpoint returns HTTP 500
- **WHEN** `GetDocumentContent` is called
- **THEN** the returned error MUST be non-nil AND SHOULD contain "failed to get document"

### Requirement: Google Drive Auth Contract Assertions

Tests for `pkg/gdrive` authentication functions MUST assert on error behavior for malformed inputs.

#### Scenario: ClientFromStore with malformed credentials JSON
- **GIVEN** a SecretStore containing credentials that are not valid JSON
- **WHEN** `ClientFromStore` is called
- **THEN** the returned error MUST be non-nil AND SHOULD mention credential parsing

### Requirement: Slack API Parameter Forwarding Assertions

Tests for `pkg/slackapi` pagination and query functions MUST verify that caller-provided parameters are forwarded to the underlying HTTP requests.

#### Scenario: GetAllMessages forwards oldest and latest parameters
- **GIVEN** a Slack API client connected to a test HTTP server
- **WHEN** `GetAllMessages` is called with `oldest: "1700000000.000000"` and `latest: "1700100000.000000"`
- **THEN** the HTTP request to `conversations.history` MUST include `oldest=1700000000.000000` and `latest=1700100000.000000` as form parameters

#### Scenario: GetAllMessages empty page skips callback
- **GIVEN** a Slack API client connected to a test HTTP server that returns zero messages with `has_more: false`
- **WHEN** `GetAllMessages` is called with a callback
- **THEN** the callback MUST NOT be invoked

#### Scenario: GetAllReplies forwards threadTS parameter
- **GIVEN** a Slack API client connected to a test HTTP server
- **WHEN** `GetAllReplies` is called with `threadTS: "1700000000.123456"`
- **THEN** the HTTP request to `conversations.replies` MUST include `ts=1700000000.123456` as a form parameter

#### Scenario: ListConversations forwards ExcludeArchived
- **GIVEN** a Slack API client connected to a test HTTP server
- **WHEN** `ListConversations` is called with `ExcludeArchived: true`
- **THEN** the HTTP request MUST include `exclude_archived=true` as a form parameter

#### Scenario: ListConversations with nil options
- **GIVEN** a Slack API client connected to a test HTTP server
- **WHEN** `ListConversations` is called with `nil` options
- **THEN** the function MUST NOT panic AND MUST return a valid response

#### Scenario: ListConversations default limit
- **GIVEN** a Slack API client connected to a test HTTP server
- **WHEN** `ListConversations` is called with default options (no limit override)
- **THEN** the HTTP request MUST include `limit=200` as a form parameter

#### Scenario: ListConversations return value assertions
- **GIVEN** a Slack API client connected to a test HTTP server returning 2 channels
- **WHEN** `ListConversations` is called
- **THEN** the returned `ConversationsListResponse` MUST have `len(Channels) == 2` AND each channel MUST have its `ID` and `Name` fields populated

#### Scenario: DownloadFile enforces 50MB size cap
- **GIVEN** a Slack API client connected to a test HTTP server that returns a response body larger than 50MB
- **WHEN** `DownloadFile` is called
- **THEN** the returned `[]byte` MUST have length ≤ 50 * 1024 * 1024

### Requirement: Parser Resolver Contract Assertions

Tests for `pkg/parser` resolver functions MUST assert on context cancellation and parameter forwarding behavior.

#### Scenario: LoadChannels context cancellation during pagination
- **GIVEN** a `ChannelResolver` with a mock `SlackAPI` that returns `has_more: true` on the first page
- **WHEN** `LoadChannels` is called and the context is cancelled before the second page request
- **THEN** the returned error MUST be non-nil AND MUST be `context.Canceled`

#### Scenario: LoadChannels types filter
- **GIVEN** a `ChannelResolver` with a mock `SlackAPI`
- **WHEN** `LoadChannels` is called
- **THEN** the `Types` field passed to `ListConversations` MUST equal `["public_channel", "private_channel"]`

## MODIFIED Requirements

_None. All existing test behavior is preserved. New test cases are additive._

## REMOVED Requirements

_None._
