# 知涟 Architecture

## Tech Stack

| Layer | Choice | Reason |
| --- | --- | --- |
| Backend language | Go | Main learning target and production-style backend language |
| HTTP framework | Gin | Simple routing, middleware, and ecosystem fit |
| ORM | GORM | Fast CRUD and model mapping for MySQL |
| Database | MySQL | Durable business data and resume-friendly backend practice |
| Cache | Redis | Feed cache, counters, token state, hot ranking |
| Async | RabbitMQ | Content events, review events, notification events |
| Frontend | React + Vite + TypeScript | Fast full-stack demo and maintainable UI |
| Deployment | Docker Compose | One-command local/demo deployment |

## Runtime Topology

```text
Browser
  -> React Web
  -> Gin API
       -> MySQL
       -> Redis
       -> RabbitMQ
       -> Local image storage volume

RabbitMQ
  -> Worker process
       -> MySQL
       -> Redis

ripple-guard-agent (知涟洞察)
  -> Internal review/governance APIs on 知涟
```

## Backend Processes

`cmd/server` runs the public and admin HTTP API.

`cmd/worker` consumes async events:

- `note.published`
- `note.review_requested`
- `note.review_decided`
- `interaction.created`
- `notification.created`

The first implementation can keep workers simple. The value is clear async boundaries, not maximum throughput.

## Proposed Directory Structure

```text
cmd/
  server/
  worker/
  seed/

configs/
  config.local.yaml
  config.docker.yaml

internal/
  account/
  auth/
  note/
  feed/
  interaction/
  social/
  review/
  admin/
  upload/
  event/
  notification/
  http/
  middleware/
  storage/
  cache/
  queue/
  config/
  observability/

web/
  src/

docs/
```

## Module Responsibilities

| Module | Responsibility |
| --- | --- |
| `account` | Users, profiles, login identity |
| `auth` | JWT, password hash, token middleware |
| `note` | Note lifecycle, images, tags, visibility |
| `feed` | Feed scenes, cursor pagination, ranking inputs |
| `interaction` | Likes, favorites, comments, counters |
| `social` | Follow and follower relationships |
| `review` | Review tasks, state machine, agent callbacks |
| `admin` | Admin review and content management APIs |
| `upload` | Image upload and static file serving |
| `event` | Outbox and event publishing |
| `notification` | User-visible system messages |
| `http` | Router wiring and DTO boundaries |
| `observability` | Logs, metrics, tracing hooks |

## Request Flow

### Publishing A Note

```text
POST /api/notes
  -> validate title/body/images/tags
  -> insert notes(status=pending_review)
  -> insert review_tasks(status=pending_agent)
  -> insert outbox_events(note.review_requested)
  -> return note detail
```

### Agent Review Callback

```text
PUT /internal/review/tasks/{taskId}/agent-result
  -> verify internal API token
  -> validate decision payload
  -> update review task
  -> update note status
  -> publish review event
  -> return current task status
```

### Feed Read

```text
GET /api/feed/latest
  -> read Redis feed cache if available
  -> query MySQL approved public notes by cursor
  -> hydrate author, counters, images
  -> write short-lived Redis cache
  -> return cursor page
```

## Deployment Shape

V1 Docker Compose should start:

- `mysql`
- `redis`
- `rabbitmq`
- `api`
- `worker`
- `web`

Images are stored in a mounted volume in V1. MinIO can be added after the core demo works.
