# Configuration Schema: Slack Message Export

**Feature**: 001-slack-message-export  
**Date**: 2026-02-03

This document defines the configuration file schemas for the get-out tool.

## File Overview

| File | Purpose | Required |
|------|---------|----------|
| `config/conversations.json` | Defines which conversations to export | Yes |
| `config/people.json` | User mapping (Slack→Google) and preferences | No (optional) |

---

## conversations.json

Unified configuration for all conversations to export. Replaces the old split between `channels.json` and `browser-export.json`.

### Schema

```json
{
  "conversations": [
    {
      "id": "C04KFBJTDJR",
      "name": "team-engineering",
      "type": "channel",
      "mode": "api",
      "export": true,
      "share": true,
      "shareMembers": []
    },
    {
      "id": "D1234567890",
      "name": "John Smith",
      "type": "dm",
      "mode": "browser",
      "export": true,
      "share": true,
      "shareMembers": []
    },
    {
      "id": "G9876543210",
      "name": "Alice, Bob, Carol",
      "type": "mpim",
      "mode": "browser",
      "export": true,
      "share": false,
      "shareMembers": []
    }
  ]
}
```

### Field Definitions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Slack conversation ID (C=channel, D=DM, G=group DM) |
| `name` | string | Yes | Display name for folder naming in Google Drive |
| `type` | enum | Yes | `channel`, `private_channel`, `dm`, `mpim` |
| `mode` | enum | Yes | `api` (bot token) or `browser` (session extraction) |
| `export` | bool | No | Whether to export (default: true) |
| `share` | bool | No | Whether to share folder with members (default: true) |
| `shareMembers` | array | No | Selective sharing list (empty = all members) |

### Type Definitions

| Type | Slack ID Prefix | Description | Recommended Mode |
|------|-----------------|-------------|------------------|
| `channel` | C | Public channel | `api` |
| `private_channel` | C | Private channel | `api` |
| `dm` | D | Direct message | `browser` |
| `mpim` | G | Multi-party instant message (group DM) | `browser` |

### Mode Definitions

| Mode | Description | When to Use |
|------|-------------|-------------|
| `api` | Uses Slack bot token (xoxb-) | Channels where bot is a member |
| `browser` | Uses browser session token (xoxc-) | DMs and group DMs (no bot access) |

### ShareMembers Format

The `shareMembers` array accepts multiple identifier formats:

```json
"shareMembers": [
  "U1234567890",           // Slack user ID
  "alice@example.com",     // Email address
  "Bob Smith"              // Display name (case-insensitive)
]
```

When `shareMembers` is empty or omitted:
- **Channels**: Share with all channel members
- **DMs**: Share with conversation participants
- **MPIMs**: Share with all group participants

---

## people.json

User mapping and preferences. Used for:
1. Slack user ID → Google account mapping (for @mention linking)
2. User opt-out preferences (noShare, noNotifications)
3. Performance cache (avoid repeated Slack API lookups)

### Schema

```json
{
  "people": [
    {
      "slackId": "U1234567890",
      "email": "john.smith@company.com",
      "displayName": "John Smith",
      "googleEmail": "john.smith@gmail.com",
      "noNotifications": false,
      "noShare": false
    },
    {
      "slackId": "U9876543210",
      "email": "jane.doe@company.com",
      "displayName": "Jane Doe",
      "googleEmail": null,
      "noNotifications": true,
      "noShare": false
    }
  ]
}
```

### Field Definitions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `slackId` | string | Yes | Slack user ID (starts with U) |
| `email` | string | No | Slack workspace email |
| `displayName` | string | No | User's display name |
| `googleEmail` | string | No | Google account email (for @mention linking) |
| `noNotifications` | bool | No | Don't send email when sharing (default: false) |
| `noShare` | bool | No | Exclude from all sharing (default: false) |

### Google Email Mapping

When a user is @mentioned in a message:
1. If `googleEmail` is set → Create mailto: link in Google Doc
2. If `googleEmail` is null → Use plain text @mention (no link)

This enables the "replace Slack @mentions with Google user links" feature.

---

## Validation Rules

### conversations.json

1. `id` must match Slack ID pattern: `^[CDGW][A-Z0-9]+$`
2. `type` must be one of: `channel`, `private_channel`, `dm`, `mpim`
3. `mode` must be one of: `api`, `browser`
4. `name` is required and must be non-empty
5. `shareMembers` if present must be an array

### people.json

1. `slackId` must match pattern: `^U[A-Z0-9]+$`
2. `email` if present must be valid email format
3. `googleEmail` if present must be valid email format

---

## Example: Complete Configuration

### config/conversations.json

```json
{
  "conversations": [
    {
      "id": "C04KFBJTDJR",
      "name": "team-psce",
      "type": "channel",
      "mode": "api",
      "export": true,
      "share": true
    },
    {
      "id": "C05LGUSIA25",
      "name": "general",
      "type": "channel",
      "mode": "api",
      "export": false,
      "share": false
    },
    {
      "id": "D08TJE6N4EW",
      "name": "John Smith",
      "type": "dm",
      "mode": "browser",
      "export": true,
      "share": true
    },
    {
      "id": "G095MTKNF3",
      "name": "Alice, Bob, Carol",
      "type": "mpim",
      "mode": "browser",
      "export": true,
      "share": true,
      "shareMembers": ["alice@example.com", "bob@example.com"]
    }
  ]
}
```

### config/people.json

```json
{
  "people": [
    {
      "slackId": "U08TN3N89E",
      "email": "john.smith@company.com",
      "displayName": "John Smith",
      "googleEmail": "john.smith@gmail.com"
    },
    {
      "slackId": "U123456789",
      "email": "alice@company.com",
      "displayName": "Alice",
      "googleEmail": "alice@gmail.com",
      "noNotifications": true
    },
    {
      "slackId": "U987654321",
      "email": "bob@company.com",
      "displayName": "Bob",
      "noShare": true
    }
  ]
}
```

---

## Migration from Old Project

If you have existing `channels.json` and `browser-export.json` files:

### channels.json → conversations.json

```json
// Old format
{
  "channels": [
    {"id": "C04KU2JTDJR", "displayName": "team-orange", "export": true, "share": true}
  ]
}

// New format
{
  "conversations": [
    {"id": "C04KU2JTDJR", "name": "team-orange", "type": "channel", "mode": "api", "export": true, "share": true}
  ]
}
```

### browser-export.json → conversations.json

```json
// Old format
{
  "browser-export": [
    {"id": "D1234567890", "name": "John Smith", "is_im": true, "is_mpim": false, "export": true}
  ]
}

// New format (merged with above)
{
  "conversations": [
    // ... channels from above ...
    {"id": "D1234567890", "name": "John Smith", "type": "dm", "mode": "browser", "export": true, "share": true}
  ]
}
```

### Mapping is_im/is_mpim to type

| Old Fields | New Type |
|------------|----------|
| `is_im: true` | `dm` |
| `is_mpim: true` | `mpim` |
| Neither (default) | `channel` |
