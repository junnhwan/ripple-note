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

## Error Code Catalog

知涟错误响应保持统一结构，`error.code` 既服务前端提示，也服务面试时解释 API 契约。业务代码可以保留更细的错误码，但必须能归入以下类别。

| 类别 | HTTP 状态 | 通用 code | 当前示例 | 说明 |
| --- | --- | --- | --- | --- |
| 参数错误 | `400` | `invalid_argument` / `validation_error` | `invalid_json`、`invalid_note_id`、`invalid_decision` | 请求体、路径参数、业务字段不合法 |
| 未认证 | `401` | `unauthorized` | `unauthorized`、`invalid_credentials` | 缺少 JWT、JWT 无效、登录失败 |
| 权限不足 | `403` | `forbidden` | `forbidden`、`user_disabled` | 已认证但无权限或账号不可用 |
| 资源不存在 | `404` | `not_found` | `not_found`、`note_not_found`、`task_not_found` | 路由或业务资源不存在 |
| 状态冲突 | `409` | `state_conflict` | `already_decided`、`email_already_registered` | 重复注册、重复决策等状态冲突 |
| 频率限制 | `429` | `rate_limited` | `rate_limited` | Redis 固定窗口限流触发 |
| 内部错误 | `500` | `internal_error` | `internal_error` | 未暴露给客户端的服务端错误 |

错误码命名原则：

- 前端可以直接判断 `code`，不解析自然语言 `message`。
- `message` 面向用户或调用方，避免泄露 SQL、Redis、RabbitMQ 等内部错误。
- 内部 Agent API 也使用相同错误结构，便于 `知涟洞察` 统一处理失败。

## Account APIs

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/users` | No | Register |
| `POST` | `/api/sessions` | No | Login |
| `DELETE` | `/api/sessions/current` | Yes | Logout current session; V1 is stateless JWT logout and returns `logged_out=true` |
| `GET` | `/api/users/me` | Yes | Current user |
| `PATCH` | `/api/users/me` | Yes | Update profile |
| `GET` | `/api/users/{userId}` | No | Public profile |

Update profile request:

```json
{
  "nickname": "Alice",
  "avatar_url": "/uploads/images/avatar.jpg",
  "bio": "Go backend learner"
}
```

`PATCH /api/users/me` supports partial updates. Omitted fields keep their current values; an explicitly blank `nickname` is rejected.

Public profile response only exposes public fields:

```json
{
  "id": 1,
  "nickname": "Alice",
  "avatar_url": "/uploads/images/avatar.jpg",
  "bio": "Go backend learner",
  "created_at": "2026-06-01T10:00:00Z"
}
```

Public profile must not expose `email`, `role`, `status`, password hash, or token state.

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

Admin note search:

```text
GET /api/admin/notes?status=published&q=go&limit=20&offset=0
```

Supported status filters:

- `pending_review`
- `published`
- `rejected`
- `removed`

Response items expose note governance fields such as `status`, `visibility`, `review_task_id`, counters, and timestamps. The endpoint is for a compact admin review console, not a full search engine.

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
