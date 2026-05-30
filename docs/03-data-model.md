# 知涟 Data Model

## Database

Use MySQL 8 with InnoDB and `utf8mb4`. IDs can be unsigned big integers with snowflake-style or database auto increment. For a learning project, auto increment is acceptable in V1.

Timestamps:

- `created_at`
- `updated_at`
- `deleted_at` for soft-delete tables

## Core Tables

### `users`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | bigint unsigned pk | User ID |
| `email` | varchar(191) unique | Login email |
| `password_hash` | varchar(255) | Bcrypt hash |
| `nickname` | varchar(64) | Display name |
| `avatar_url` | varchar(512) | Optional |
| `bio` | varchar(512) | Optional |
| `role` | varchar(32) | `user`, `admin` |
| `status` | varchar(32) | `active`, `disabled` |

### `notes`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | bigint unsigned pk | Note ID |
| `author_id` | bigint unsigned index | Author |
| `title` | varchar(120) | Required |
| `body` | text | Markdown/plain text in V1 |
| `status` | varchar(32) index | `draft`, `pending_review`, `published`, `rejected`, `removed` |
| `visibility` | varchar(32) | `public`, `private` |
| `review_task_id` | bigint unsigned null | Latest review task |
| `published_at` | datetime null index | Set when approved |
| `likes_count` | bigint unsigned | Denormalized |
| `favorites_count` | bigint unsigned | Denormalized |
| `comments_count` | bigint unsigned | Denormalized |
| `hot_score` | double | Baseline hot sorting |

### `note_images`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | bigint unsigned pk | Image ID |
| `note_id` | bigint unsigned index | Note |
| `url` | varchar(512) | Public URL |
| `storage_key` | varchar(512) | Local path or object key |
| `sort_order` | int | Display order |
| `width` | int null | Optional |
| `height` | int null | Optional |

### `tags`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | bigint unsigned pk | Tag ID |
| `name` | varchar(64) unique | Normalized tag |
| `notes_count` | bigint unsigned | Denormalized |

### `note_tags`

| Column | Type | Notes |
| --- | --- | --- |
| `note_id` | bigint unsigned pk | Note |
| `tag_id` | bigint unsigned pk | Tag |

## Social And Interaction Tables

### `follows`

| Column | Type | Notes |
| --- | --- | --- |
| `follower_id` | bigint unsigned pk | User who follows |
| `followee_id` | bigint unsigned pk | User followed |

### `note_likes`

| Column | Type | Notes |
| --- | --- | --- |
| `user_id` | bigint unsigned pk | User |
| `note_id` | bigint unsigned pk | Note |

### `note_favorites`

| Column | Type | Notes |
| --- | --- | --- |
| `user_id` | bigint unsigned pk | User |
| `note_id` | bigint unsigned pk | Note |

### `comments`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | bigint unsigned pk | Comment ID |
| `note_id` | bigint unsigned index | Note |
| `author_id` | bigint unsigned index | Author |
| `parent_id` | bigint unsigned null | V1 can only show one level |
| `body` | varchar(1000) | Text |
| `status` | varchar(32) | `visible`, `removed` |

## Review Tables

### `review_tasks`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | bigint unsigned pk | Task ID |
| `note_id` | bigint unsigned index | Reviewed note |
| `author_id` | bigint unsigned index | Note author |
| `status` | varchar(32) index | `pending_agent`, `agent_passed`, `agent_rejected`, `manual_required`, `admin_approved`, `admin_rejected` |
| `source` | varchar(32) | `publish`, `edit`, `report` |
| `agent_decision` | varchar(32) null | `pass`, `reject`, `manual_review` |
| `agent_risk_level` | varchar(32) null | `low`, `medium`, `high` |
| `agent_reason` | text null | Agent explanation |
| `agent_trace_id` | varchar(128) null | Trace ID from ripple-guard-agent |
| `admin_decision` | varchar(32) null | Admin final decision |
| `admin_reason` | text null | Admin explanation |
| `decided_at` | datetime null | Final decision time |

### `review_task_events`

Append-only audit trail.

| Column | Type | Notes |
| --- | --- | --- |
| `id` | bigint unsigned pk | Event ID |
| `task_id` | bigint unsigned index | Review task |
| `actor_type` | varchar(32) | `system`, `agent`, `admin` |
| `actor_id` | varchar(128) | User ID or service name |
| `event_type` | varchar(64) | State transition |
| `payload_json` | json | Decision details |

## Event Tables

### `outbox_events`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | bigint unsigned pk | Event ID |
| `topic` | varchar(128) index | Event topic |
| `aggregate_type` | varchar(64) | `note`, `review_task`, `comment` |
| `aggregate_id` | bigint unsigned | Entity ID |
| `payload_json` | json | Event body |
| `status` | varchar(32) index | `pending`, `sent`, `failed` |
| `retry_count` | int | Publish retries |
| `next_retry_at` | datetime null | Retry schedule |

## Index Notes

- `notes(status, published_at, id)` for latest feed.
- `notes(status, hot_score, id)` for hot feed baseline.
- `review_tasks(status, id)` for agent polling.
- `comments(note_id, id)` for comment pagination.
- `follows(follower_id, followee_id)` and `follows(followee_id, follower_id)` for relationship queries.
