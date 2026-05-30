# 知涟 Development Roadmap

This roadmap is the stage-by-stage development and learning plan for 知涟. It complements `docs/09-implementation-plan.md`.

Use this document when starting a new AI development session. The expected workflow is:

```text
stage plan -> implement whole stage -> verify -> update learning log -> teach -> user questions -> next stage
```

## Product Name And Repository

- Product name: `知涟`
- Repository/directory name: `ripple-note`
- Positioning: content community and Feed distribution platform
- Paired Agent product: `知涟洞察`
- Paired Agent repository/directory: `ripple-guard-agent`

## Stage 1: Backend Skeleton And API Contract

### Goal

Build the backend foundation without business features.

### Scope

- `docs/10-openapi-contract.md` or `docs/openapi.yaml`
- `go.mod`
- `cmd/server/main.go`
- `configs/config.local.yaml`
- `internal/config`
- `internal/http`
- `internal/middleware`
- `internal/observability`
- `internal/storage`
- `GET /health`

### Learning Goals

- Go module basics.
- Go project layout.
- Gin router setup.
- Middleware execution order.
- Unified JSON response.
- Error handling style in Go.
- Config loading.
- GORM MySQL connection.

### Acceptance

```bash
go test ./...
go run ./cmd/server -config configs/config.local.yaml
curl http://127.0.0.1:8080/health
```

Expected `/health` response should follow the unified response format and include a request ID.

## Stage 2: Account And Authentication

### Goal

Allow users to register, log in, and access authenticated APIs.

### Scope

- `users` table and GORM model.
- `POST /api/users`
- `POST /api/sessions`
- `GET /api/users/me`
- Bcrypt password hashing.
- JWT signing and verification.
- Auth middleware.

### Learning Goals

- Handler/service/repository boundaries.
- DTO vs persistence model.
- Password hashing.
- JWT claims and expiry.
- Gin middleware for authentication.
- Basic API tests.

### Acceptance

- User can register.
- User can log in and receive a token.
- Token can access `/api/users/me`.
- `go test ./...` passes.

## Stage 3: Content Publishing And Uploads

### Goal

Allow users to upload images and publish content that enters the platform state flow.

### Scope

- `notes`
- `note_images`
- `tags`
- `note_tags`
- `POST /api/uploads/images`
- `POST /api/notes`
- `GET /api/notes/{noteId}`
- `GET /api/users/me/notes`

New content should default to:

```text
status = pending_review
```

### Learning Goals

- Multipart upload handling.
- Static file serving.
- Multi-table writes.
- GORM associations.
- Transactions.
- Request validation.
- Content status design.

### Acceptance

- User can upload an image.
- User can publish content.
- Published content starts as `pending_review`.
- Pending content does not appear in public Feed.

## Stage 4: Content State Flow And Review Tasks

### Goal

Build the review/governance task workflow that controls content visibility.

### Scope

- `review_tasks`
- `review_task_events`
- Create review task in the same transaction as content publishing.
- `GET /api/admin/review/tasks`
- `GET /api/admin/review/tasks/{taskId}`
- `PUT /api/admin/review/tasks/{taskId}/decision`

Review task statuses:

```text
pending_agent
agent_passed
agent_rejected
manual_required
admin_approved
admin_rejected
```

Content statuses:

```text
pending_review
published
rejected
removed
```

### Learning Goals

- State machine modeling.
- Transaction boundaries.
- Audit event tables.
- Admin API design.
- Idempotent updates.
- Business state vs database state.

### Acceptance

- Publishing content creates a review task.
- Admin can approve content.
- Approved content becomes `published`.
- Admin can reject content.
- Rejected content becomes `rejected`.

## Stage 5: Internal APIs For 知涟洞察

### Goal

Expose stable service-to-service APIs for the Agent project.

### Scope

- `GET /internal/review/tasks/pending`
- `GET /internal/review/tasks/{taskId}`
- `GET /internal/notes/{noteId}/review-context`
- `PUT /internal/review/tasks/{taskId}/agent-result`
- `X-Internal-Token` authentication.

Agent result fields:

```text
decision
risk_level
categories
reason
evidence
confidence
trace_id
```

### Learning Goals

- Service-to-service API design.
- Internal authentication.
- Idempotent callbacks.
- Trace ID linkage.
- Integration boundaries between services.

### Acceptance

- Agent can pull pending tasks.
- Agent can fetch content context.
- Agent callback can simulate `pass`, `reject`, and `manual_review`.
- Content and task states update correctly after callback.

## Stage 6: Feed Scenes

### Goal

Implement the core read path of the content platform.

### Scope

- `GET /api/feed/latest`
- `GET /api/feed/hot`
- `GET /api/feed/following`
- `GET /api/tags/{tagName}/feed`
- Cursor encoding and decoding.
- Feed DTO hydration.

Cursor strategy:

```text
latest feed: published_at + id
hot feed: hot_score + id
following feed: published_at + id + followee filter
```

### Learning Goals

- Feed read-path design.
- Compound cursor pagination.
- Why not to use offset for infinite scroll.
- SQL index design.
- DTO hydration.
- Optional/soft authentication for public APIs.

### Acceptance

- `pending_review` and `rejected` content never appears in public Feed.
- Cursor pagination does not duplicate items.
- Cursor pagination does not miss items in normal scenarios.
- Logged-in users receive viewer flags such as liked/favorited/following.

## Stage 7: Interactions And Social Graph

### Goal

Add interaction and relationship loops.

### Scope

- `note_likes`
- `note_favorites`
- `comments`
- `follows`
- `PUT /api/notes/{noteId}/like`
- `DELETE /api/notes/{noteId}/like`
- `PUT /api/notes/{noteId}/favorite`
- `DELETE /api/notes/{noteId}/favorite`
- `POST /api/notes/{noteId}/comments`
- `GET /api/notes/{noteId}/comments`
- `PUT /api/users/me/following/{targetUserId}`
- `DELETE /api/users/me/following/{targetUserId}`

### Learning Goals

- Idempotent API design.
- Unique indexes.
- Counter consistency.
- Concurrent updates.
- Transactions.
- Soft delete behavior.

### Acceptance

- Repeated like does not double-count.
- Repeated unlike does not make counts negative.
- Comments can be paginated.
- Following feed shows followed authors' content.

## Stage 8: Redis Cache

### Goal

Add targeted cache to reduce hot read pressure.

### Scope

Cache keys:

```text
feed:latest:first-page
note:detail:{id}
note:counts:{id}
user:profile:{id}
```

Strategies:

- Short TTL.
- Active invalidation.
- Cache miss fallback to MySQL.
- Clear affected cache after content state changes.

### Learning Goals

- Redis data structures.
- Cache penetration, breakdown, and avalanche.
- TTL strategy.
- Cache consistency.
- Optional singleflight for hot key fallback.

### Acceptance

- Latest Feed first page can hit Redis.
- Content detail can hit Redis.
- Review/governance status changes invalidate affected cache.
- Interaction count cache does not remain permanently inconsistent with MySQL.

## Stage 9: RabbitMQ And Outbox

### Goal

Build an async event path and eventual consistency story.

### Scope

- `outbox_events`
- `cmd/worker`
- Event publisher.
- RabbitMQ producer.
- RabbitMQ consumer.

Events:

```text
note.review_requested
note.review_decided
interaction.created
notification.created
```

### Learning Goals

- Message queues.
- Producer and consumer responsibilities.
- Ack, retry, and dead-letter basics.
- Outbox pattern.
- Eventual consistency.
- API + Worker process split.

### Acceptance

- Content publish transaction writes business rows and `outbox_events`.
- Worker can publish pending outbox events.
- Failed event handling can retry.
- Core business correctness does not depend on RabbitMQ being immediately available.

## Stage 10: React Frontend

### Goal

Build a browser demo for the main product loop.

### Scope

- Login/register page.
- Feed page.
- Publish page.
- Content detail page.
- Profile/my content page.
- Admin review page.
- API client.
- Token management.

### Learning Goals

- Frontend/backend API integration.
- Auth token handling.
- API client design.
- Form state.
- Error display.
- Routing.

### Acceptance

Browser can complete:

```text
register -> login -> publish -> review -> Feed display -> like/comment/follow
```

## Stage 11: Docker Compose, Seed Data, README

### Goal

Make the project deployable and demo-ready.

### Scope

- `Dockerfile`
- `web/Dockerfile`
- `docker-compose.yml`
- `cmd/seed`
- `README.md`
- Screenshots.
- Demo accounts.

Services:

```text
mysql
redis
rabbitmq
api
worker
web
```

### Learning Goals

- Dockerfile basics.
- Compose service networking.
- Environment variables.
- Health checks.
- Volumes.
- Full-stack deployment.

### Acceptance

```bash
docker compose up -d --build
```

Expected demo URLs:

```text
API: http://127.0.0.1:8080
Web: http://127.0.0.1:5173
RabbitMQ: http://127.0.0.1:15672
```

## Stage 12: Benchmark And Resume Polish

### Goal

Turn the project from "works" into "can be explained and defended in interviews".

### Scope

- Feed benchmark.
- Redis before/after comparison.
- Hot feed query optimization.
- README architecture diagram.
- Resume bullets.
- Screenshots.
- API document cleanup.

### Learning Goals

- P95/P99 latency.
- Throughput.
- Benchmark design.
- Reading performance results.
- Resume quantification.

### Acceptance

Produce defensible data such as:

```text
Feed cache before/after P95/P99 latency
Feed endpoint throughput change
Hot feed optimization result
```

Do not invent metrics. Only write numbers after actual benchmark runs.

## Per-Stage Execution And Teaching Protocol

Use one stage as the main execution unit. At the beginning of a stage, give a short plan and list the main files to create or modify. Then implement the stage directly unless a decision is risky enough to require user input.

Do not stop after every small function or file just to teach. The user prefers progress first, then a focused teaching summary after the stage is verified.

It is still useful to think internally in smaller tasks. For example, Stage 2 may include:

```text
2.1 users model and migration
2.2 password hashing
2.3 registration API
2.4 login and JWT
2.5 auth middleware and /me
```

But the outward workflow should be:

```text
brief stage plan -> implement -> verify -> update docs/learning-log.md -> teach -> answer questions
```

Teaching summary format:

```text
【本次实现】
【Go 后端知识点】
【Java 对比】
【关键代码讲解】
【常见坑】
【我可以追问的问题】
```

Keep `docs/learning-log.md` updated as development progresses.
