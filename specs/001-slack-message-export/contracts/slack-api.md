# Slack API Contract: Message Export

**Feature**: 001-slack-message-export  
**Date**: 2026-02-03

This document defines the Slack API endpoints used for message extraction via API mimicry.

## Authentication

All requests require:
- **Authorization Header**: `Bearer xoxc-...` (user token from localStorage)
- **Cookie**: `d=xoxd-...` (session cookie)

### Token Extraction

```javascript
// In browser console or via Chromedp evaluation
const config = JSON.parse(localStorage.getItem('localConfig_v2'));
const token = config.teams[0].token;  // xoxc-...
```

The `d` cookie is automatically included when connecting to an existing browser session.

---

## Endpoints

### 1. conversations.list

List all conversations the user has access to.

**Request**:
```
POST https://slack.com/api/conversations.list
Content-Type: application/x-www-form-urlencoded
Authorization: Bearer {token}
Cookie: d={cookie}

types=im,mpim,private_channel&exclude_archived=false&limit=200&cursor={cursor}
```

**Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `types` | string | No | Comma-separated: `im`, `mpim`, `public_channel`, `private_channel` |
| `exclude_archived` | bool | No | Skip archived conversations |
| `limit` | int | No | Max 1000, default 100 |
| `cursor` | string | No | Pagination cursor |

**Response**:
```json
{
  "ok": true,
  "channels": [
    {
      "id": "D123ABC456",
      "name": "john.smith",
      "is_im": true,
      "is_mpim": false,
      "user": "U123ABC456",
      "created": 1609459200
    },
    {
      "id": "G789DEF012",
      "name": "mpdm-user1--user2--user3-1",
      "is_im": false,
      "is_mpim": true,
      "created": 1609459200
    }
  ],
  "response_metadata": {
    "next_cursor": "dGVhbTpDMDYxRkE1UEI="
  }
}
```

**Rate Limit**: Tier 2 (~20 req/min)

---

### 2. conversations.history

Retrieve message history for a conversation.

**Request**:
```
POST https://slack.com/api/conversations.history
Content-Type: application/x-www-form-urlencoded
Authorization: Bearer {token}
Cookie: d={cookie}

channel={channel_id}&limit=100&cursor={cursor}&oldest={timestamp}&latest={timestamp}
```

**Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `channel` | string | Yes | Conversation ID |
| `limit` | int | No | Max 1000, default 100 |
| `cursor` | string | No | Pagination cursor |
| `oldest` | string | No | Only messages after this timestamp |
| `latest` | string | No | Only messages before this timestamp |
| `inclusive` | bool | No | Include messages with oldest/latest ts |

**Response**:
```json
{
  "ok": true,
  "messages": [
    {
      "type": "message",
      "user": "U123ABC456",
      "text": "Hello world!",
      "ts": "1704067200.000100",
      "reactions": [
        {
          "name": "thumbsup",
          "users": ["U789DEF012"],
          "count": 1
        }
      ]
    },
    {
      "type": "message",
      "user": "U789DEF012",
      "text": "This is a thread parent",
      "ts": "1704067300.000200",
      "thread_ts": "1704067300.000200",
      "reply_count": 3,
      "reply_users_count": 2,
      "latest_reply": "1704067400.000300"
    }
  ],
  "has_more": true,
  "response_metadata": {
    "next_cursor": "bmV4dF90czoxNzA0MDY3MjAwMDAwMTAw"
  }
}
```

**Rate Limit**: Tier 3 (~50 req/min)

---

### 3. conversations.replies

Retrieve replies in a thread.

**Request**:
```
POST https://slack.com/api/conversations.replies
Content-Type: application/x-www-form-urlencoded
Authorization: Bearer {token}
Cookie: d={cookie}

channel={channel_id}&ts={thread_ts}&limit=100&cursor={cursor}
```

**Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `channel` | string | Yes | Conversation ID |
| `ts` | string | Yes | Thread parent timestamp |
| `limit` | int | No | Max 1000, default 100 |
| `cursor` | string | No | Pagination cursor |
| `oldest` | string | No | Only replies after this timestamp |
| `latest` | string | No | Only replies before this timestamp |

**Response**:
```json
{
  "ok": true,
  "messages": [
    {
      "type": "message",
      "user": "U789DEF012",
      "text": "This is the parent message",
      "ts": "1704067300.000200",
      "thread_ts": "1704067300.000200",
      "reply_count": 3
    },
    {
      "type": "message",
      "user": "U123ABC456",
      "text": "First reply",
      "ts": "1704067350.000250",
      "thread_ts": "1704067300.000200",
      "parent_user_id": "U789DEF012"
    },
    {
      "type": "message",
      "user": "U456GHI789",
      "text": "Second reply",
      "ts": "1704067400.000300",
      "thread_ts": "1704067300.000200",
      "parent_user_id": "U789DEF012"
    }
  ],
  "has_more": false
}
```

**Rate Limit**: Tier 3 (~50 req/min)

---

### 4. users.info

Get information about a single user.

**Request**:
```
POST https://slack.com/api/users.info
Content-Type: application/x-www-form-urlencoded
Authorization: Bearer {token}
Cookie: d={cookie}

user={user_id}
```

**Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `user` | string | Yes | User ID |

**Response**:
```json
{
  "ok": true,
  "user": {
    "id": "U123ABC456",
    "name": "john.smith",
    "real_name": "John Smith",
    "profile": {
      "display_name": "John",
      "image_72": "https://avatars.slack-edge.com/..."
    },
    "is_bot": false,
    "deleted": false
  }
}
```

**Rate Limit**: Tier 4 (~100 req/min)

---

### 5. users.list

List all users in the workspace (for bulk name resolution).

**Request**:
```
POST https://slack.com/api/users.list
Content-Type: application/x-www-form-urlencoded
Authorization: Bearer {token}
Cookie: d={cookie}

limit=200&cursor={cursor}
```

**Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `limit` | int | No | Max 1000, default 100 |
| `cursor` | string | No | Pagination cursor |

**Response**:
```json
{
  "ok": true,
  "members": [
    {
      "id": "U123ABC456",
      "name": "john.smith",
      "real_name": "John Smith",
      "profile": {
        "display_name": "John"
      },
      "is_bot": false,
      "deleted": false
    }
  ],
  "response_metadata": {
    "next_cursor": "dGVhbTpDMDYxRkE1UEI="
  }
}
```

**Rate Limit**: Tier 2 (~20 req/min)

---

## Error Responses

All endpoints may return errors:

```json
{
  "ok": false,
  "error": "channel_not_found"
}
```

### Common Error Codes

| Error | Description | Action |
|-------|-------------|--------|
| `ratelimited` | Too many requests | Wait and retry with backoff |
| `token_revoked` | Token is invalid | Re-authenticate |
| `channel_not_found` | Invalid channel ID | Skip or report |
| `not_in_channel` | No access to channel | Skip or report |
| `invalid_auth` | Auth failed | Check token/cookie |
| `account_inactive` | User deactivated | Re-authenticate |

### Rate Limit Headers

When rate limited, response includes:
```
Retry-After: 30
```

---

## Go Interface Definition

```go
package slackapi

import "context"

type Client interface {
    // List conversations accessible to the user
    ListConversations(ctx context.Context, opts ListConversationsOpts) (*ConversationsListResponse, error)
    
    // Get message history for a conversation
    GetHistory(ctx context.Context, channelID string, opts HistoryOpts) (*HistoryResponse, error)
    
    // Get thread replies
    GetReplies(ctx context.Context, channelID, threadTS string, opts RepliesOpts) (*RepliesResponse, error)
    
    // Get user info
    GetUser(ctx context.Context, userID string) (*User, error)
    
    // List all users (for bulk resolution)
    ListUsers(ctx context.Context, opts ListUsersOpts) (*UsersListResponse, error)
}

type ListConversationsOpts struct {
    Types          []string // "im", "mpim", "private_channel"
    ExcludeArchived bool
    Limit          int
    Cursor         string
}

type HistoryOpts struct {
    Limit     int
    Cursor    string
    Oldest    string // timestamp
    Latest    string // timestamp
    Inclusive bool
}

type RepliesOpts struct {
    Limit  int
    Cursor string
    Oldest string
    Latest string
}

type ListUsersOpts struct {
    Limit  int
    Cursor string
}
```
