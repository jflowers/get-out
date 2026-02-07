# Feature Specification: Slack Message Export

**Feature Branch**: `001-slack-message-export`  
**Created**: 2026-02-03  
**Status**: In Progress (MVP complete, US1+US2+US5 partial done)  
**Last Updated**: 2026-02-07  
**Input**: User description: "Build a CLI tool that exports Slack DMs and group threads to Google Docs in Google Drive using session-driven extraction from an active browser session"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Export Direct Messages (Priority: P1)

As a Slack user, I want to export my direct message conversations to Google Docs in my Google Drive so that I can preserve important conversations in an easily accessible, searchable, and shareable format.

**Why this priority**: Direct messages are the most personal and often most important conversations users want to preserve. Google Docs provides rich formatting, easy sharing, and cross-device access.

**Independent Test**: Can be fully tested by exporting a single DM conversation and verifying a Google Doc is created in Drive with all messages, proper formatting, timestamps, and sender names.

**Acceptance Scenarios**:

1. **Given** the user has an active Slack session in their browser and Google Drive access configured, **When** they run the export command for a specific DM conversation, **Then** a Google Doc is created in their Drive with all messages properly formatted
2. **Given** a DM conversation with 500+ messages, **When** the user exports it, **Then** all messages are captured in the Google Doc without data loss, including those requiring scroll-to-load
3. **Given** an export is interrupted mid-process, **When** the user restarts the export, **Then** it resumes from the last saved checkpoint without duplicating messages

---

### User Story 2 - Export Group Threads with Organized Structure (Priority: P2)

As a Slack user, I want to export group thread conversations to Google Docs organized by day with threads in subfolders so that I can easily navigate large conversation archives and find specific discussions.

**Why this priority**: Group threads contain valuable team knowledge but are secondary to personal DM preservation. Building on the DM export foundation.

**Independent Test**: Can be fully tested by exporting a group conversation spanning multiple days with threaded replies and verifying:
- A folder is created for the conversation
- Daily Google Docs are created for main channel messages
- A "Threads" subfolder contains thread folders, each with daily-chunked docs

**Acceptance Scenarios**:

1. **Given** the user selects a group conversation spanning multiple days, **When** they export it, **Then** Google Docs are created for each day's messages (e.g., "2026-02-01.gdoc", "2026-02-02.gdoc")
2. **Given** the conversation has threaded replies, **When** exported, **Then** each thread gets a subfolder in "Threads" named by start date and topic preview (e.g., "2026-02-01 - Sprint planning/")
3. **Given** a thread spans multiple days, **When** exported, **Then** the thread folder contains daily-chunked docs (e.g., "2026-02-01.gdoc", "2026-02-02.gdoc") just like the main conversation
4. **Given** a thread has replies from multiple users, **When** exported, **Then** each reply in the thread's daily docs shows the correct sender name (not user IDs like U12345)
5. **Given** a daily doc contains a message that started a thread, **When** exported, **Then** the message includes a link to the thread's folder in the Threads subfolder

---

### User Story 3 - Resolve User Identities with Google Integration (Priority: P3)

As a user reading exported conversations, I want user mentions to show real names and link to their Google account (when available) so that the exported content is human-readable and actionable.

**Why this priority**: User ID resolution is essential for readability but depends on having the basic export infrastructure in place first.

**Independent Test**: Can be fully tested by exporting a conversation containing user mentions and verifying all `<@U12345>` patterns are replaced with actual names, and where a Google account mapping exists, the mention links to that user.

**Acceptance Scenarios**:

1. **Given** a message contains user mentions in Slack's mrkdwn format (`<@U12345>`), **When** exported, **Then** the mention is converted to a readable name format (e.g., "@John Smith")
2. **Given** a Slack user has a corresponding Google account in the user mapping, **When** their mention is exported, **Then** the mention becomes a link to their Google profile or includes their Google email
3. **Given** a user ID cannot be resolved, **When** exported, **Then** the original ID is preserved with a notation that it could not be resolved

---

### User Story 4 - Replace Slack Links with Export References (Priority: P3)

As a user reading exported conversations, I want links to other Slack messages, threads, and channels to be replaced with links to the corresponding exported Google Docs so that I can navigate between related conversations within the export.

**Why this priority**: Cross-referencing between conversations makes the export more useful as a complete archive, but requires the export structure to be in place first.

**Independent Test**: Can be fully tested by exporting a conversation that contains links to other Slack threads/channels and verifying those links point to the corresponding Google Docs in the export folder structure.

**Acceptance Scenarios**:

1. **Given** a message contains a link to another Slack message or thread, **When** exported, **Then** the link is replaced with a link to the corresponding Google Doc (if that conversation was exported)
2. **Given** a message contains a link to a Slack channel, **When** exported, **Then** the link is replaced with a link to the channel's export folder in Google Drive (if exported)
3. **Given** a Slack link references a conversation that hasn't been exported, **When** exported, **Then** the original Slack link is preserved with a note indicating it's an external Slack reference

---

### User Story 5 - Handle Rate Limiting Gracefully (Priority: P3)

As a user exporting large volumes of messages, I want the tool to handle Slack's rate limits automatically so that I don't lose progress or have to manually retry.

**Why this priority**: Rate limiting is a common obstacle but the tool should handle it transparently without user intervention.

**Independent Test**: Can be tested by simulating rate limit responses and verifying the tool backs off and retries without data loss.

**Acceptance Scenarios**:

1. **Given** Slack returns a rate limit response, **When** the tool receives it, **Then** it waits with exponential backoff and automatically retries
2. **Given** multiple rate limits occur during a large export, **When** the export completes, **Then** no messages are missing and the user is informed of any delays

---

### Edge Cases

- What happens when the browser session expires mid-export? The tool should detect this and notify the user to re-authenticate.
- What happens when a conversation is deleted during export? The tool should capture what was available and note the incomplete state.
- What happens when messages contain special formatting (code blocks, emojis, attachments)? These should be preserved in Google Docs-compatible format where possible.
- What happens when Google Drive API rate limits are hit? The tool should handle with exponential backoff similar to Slack API.
- What happens when Google Drive authentication expires? The tool should prompt for re-authentication.
- What happens when the user has no access to a requested conversation? The tool should report a clear access denied message.
- What happens when a Slack link references a conversation not yet exported? The original Slack link should be preserved with a marker.
- What happens when the user mapping file doesn't exist? Fall back to Slack names only without Google links.
- What happens when a conversation has thousands of threads? Threads subfolder should handle large numbers with clear naming.
- What happens when a thread spans multiple days? The thread doc should include all replies regardless of date.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST extract authentication tokens from an active browser session without requiring manual configuration
- **FR-002**: System MUST export direct message conversations to individual Google Docs in Google Drive
- **FR-003**: System MUST export group thread conversations including all nested replies to Google Docs
- **FR-004**: System MUST resolve Slack user IDs to human-readable names in exported content
- **FR-005**: System MUST convert Slack's mrkdwn format to Google Docs-compatible formatting
- **FR-011**: System MUST authenticate with Google Drive API using OAuth 2.0
- **FR-012**: System MUST organize exported Google Docs in a configurable Drive folder
- **FR-013**: System MUST handle Google Drive API rate limits with automatic retry
- **FR-006**: System MUST handle rate limiting with automatic exponential backoff and retry
- **FR-007**: System MUST implement checkpointing to allow interrupted exports to resume
- **FR-008**: System MUST preserve message timestamps in a human-readable format
- **FR-009**: System MUST handle conversations with virtual scrolling (lazy-loaded message history)
- **FR-010**: System MUST name Google Docs by conversation with clear naming conventions (e.g., "DM - John Smith - 2026-02-03")
- **FR-014**: System MUST organize exports into folders per conversation with daily-chunked Google Docs
- **FR-015**: System MUST export threads into a "Threads" subfolder, with each thread in its own folder containing daily-chunked docs
- **FR-016**: System MUST replace Slack user mentions with Google account links where a mapping exists
- **FR-017**: System MUST replace links to other Slack messages/threads/channels with links to corresponding exported Google Docs
- **FR-018**: System MUST support a user mapping file that maps Slack user IDs to Google account emails
- **FR-019**: System MUST link from daily docs to thread docs and vice versa for navigability
- **FR-020**: System MUST read conversation definitions from a config file (conversations.json)
- **FR-021**: System MUST support two extraction modes: API (bot token for channels) and Browser (session token for DMs/groups)
- **FR-022**: System MUST support selective sharing via shareMembers list in config
- **FR-023**: System MUST respect user opt-out preferences (noShare, noNotifications) from people.json

### Key Entities

- **Conversation**: A DM or group thread container with a unique identifier, participants, and message history
- **Message**: A single communication unit with sender, timestamp, content, and optional thread replies
- **User**: A Slack workspace member with ID and display name, used for identity resolution
- **Export Session**: A stateful operation tracking progress, checkpoints, and configuration for resumability
- **Google Doc**: The output document in Google Drive containing the formatted conversation export
- **User Mapping**: A configuration file (people.json) that maps Slack user IDs to Google account emails for enhanced mentions
- **Export Folder**: A Google Drive folder containing daily docs and a Threads subfolder for a conversation
- **Conversation Config**: The conversations.json file defining which conversations to export and their settings

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can export a 1000-message conversation in under 5 minutes under normal conditions
- **SC-002**: Exported Google Docs are readable without any Slack-specific formatting artifacts (no raw user IDs, no mrkdwn syntax)
- **SC-003**: 100% of messages in a conversation are captured, verified by comparing message count before and after export
- **SC-004**: Interrupted exports can resume within 10 seconds of restart, losing at most 1 minute of prior progress
- **SC-005**: Users can successfully export conversations without providing any manual API tokens or credentials
- **SC-006**: Exported Google Docs are searchable via Google Drive search and Google's full-text indexing
- **SC-007**: Users can share exported Google Docs with others using standard Drive sharing

## Assumptions

- Users have the Zen browser environment available with an active Slack session
- Users have appropriate Slack workspace permissions to view the conversations they want to export
- The browser session remains valid for the duration of typical exports (< 1 hour)
- Slack's internal API endpoints remain accessible via authenticated browser sessions
- Message volume for typical exports is under 10,000 messages per conversation
- Users have a Google account with Google Drive access
- Users can complete OAuth 2.0 authentication flow for Google Drive API
- Users have sufficient Google Drive storage quota for exported documents
