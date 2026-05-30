# 知涟 API Design

## API Style

- Public APIs are under `/api`.
- Admin APIs are under `/api/admin`.
- Service-to-service APIs are under `/internal`.
- JSON is the default request and response format.
- Auth uses Bearer JWT.
- Internal APIs use `X-Internal-Token` in V1.

## Common Response

```json
{
  "data": {},
  "error": null,
  "request_id": "req_..."
}
```

Error response:

```json
{
  "data": null,
  "error": {
    "code": "validation_error",
    "message": "title is required"
  },
  "request_id": "req_..."
}
```

## Account APIs

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/users` | No | Register |
| `POST` | `/api/sessions` | No | Login |
| `DELETE` | `/api/sessions/current` | Yes | Logout |
| `GET` | `/api/users/me` | Yes | Current user |
| `PATCH` | `/api/users/me` | Yes | Update profile |
| `GET` | `/api/users/{userId}` | Optional | Public profile |

## Upload APIs

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/uploads/images` | Yes | Upload one image |

V1 stores files on local disk and returns a public URL.

## Note APIs

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/notes` | Yes | Publish note into review flow |
| `GET` | `/api/notes/{noteId}` | Optional | Note detail |
| `DELETE` | `/api/notes/{noteId}` | Yes | Soft delete own note |
| `GET` | `/api/users/{userId}/notes` | Optional | Public notes by author |
| `GET` | `/api/users/me/notes` | Yes | Own notes including pending/rejected |

Create note request:

```json
{
  "title": "Go Feed design notes",
  "body": "content body",
  "image_urls": ["/uploads/a.jpg"],
  "tags": ["go", "backend"]
}
```

## Feed APIs

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/api/feed/latest` | Optional | Latest approved notes |
| `GET` | `/api/feed/following` | Yes | Notes from followed authors |
| `GET` | `/api/feed/hot` | Optional | Hot notes |
| `GET` | `/api/tags/{tagName}/feed` | Optional | Tag feed |

Cursor request:

```text
GET /api/feed/latest?cursor=...&limit=20
```

Cursor response:

```json
{
  "items": [],
  "next_cursor": "base64-json",
  "has_more": true
}
```

## Interaction APIs

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `PUT` | `/api/notes/{noteId}/like` | Yes | Like |
| `DELETE` | `/api/notes/{noteId}/like` | Yes | Unlike |
| `PUT` | `/api/notes/{noteId}/favorite` | Yes | Favorite |
| `DELETE` | `/api/notes/{noteId}/favorite` | Yes | Unfavorite |
| `POST` | `/api/notes/{noteId}/comments` | Yes | Create comment |
| `GET` | `/api/notes/{noteId}/comments` | Optional | List comments |
| `DELETE` | `/api/comments/{commentId}` | Yes | Delete own comment |
| `PUT` | `/api/users/me/following/{targetUserId}` | Yes | Follow |
| `DELETE` | `/api/users/me/following/{targetUserId}` | Yes | Unfollow |

## Admin Review APIs

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/api/admin/review/tasks` | Admin | List review tasks |
| `GET` | `/api/admin/review/tasks/{taskId}` | Admin | Review task detail |
| `PUT` | `/api/admin/review/tasks/{taskId}/decision` | Admin | Approve, reject, remove, or request manual review |
| `GET` | `/api/admin/notes` | Admin | Search notes by status |

Admin decision request:

```json
{
  "decision": "approve",
  "reason": "content complies with policy"
}
```

## Internal Agent APIs

These APIs are called by `ripple-guard-agent` (`知涟洞察`).

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/internal/review/tasks/pending?limit=10` | Pull pending review tasks |
| `GET` | `/internal/review/tasks/{taskId}` | Task detail |
| `GET` | `/internal/notes/{noteId}/review-context` | Note, images, author, recent content |
| `PUT` | `/internal/review/tasks/{taskId}/agent-result` | Submit agent decision |

Agent result request:

```json
{
  "decision": "pass",
  "risk_level": "low",
  "categories": [],
  "reason": "No policy risk found.",
  "evidence": [
    {
      "type": "content_text",
      "summary": "Title and body are normal learning notes."
    }
  ],
  "confidence": 0.91,
  "trace_id": "rg_trace_..."
}
```
