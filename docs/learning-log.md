# 知涟 Learning Log

## 2026-05-30 Stage 1: Backend Skeleton And API Contract

### Stage Goal

建立 Go 后端项目骨架，让服务可以从配置文件启动，并提供带统一响应格式和 request ID 的 `GET /health` 接口。此阶段不实现账号、内容发布或 Feed 业务，只铺好后续模块复用的基础设施。

### Files Created

- `go.mod`, `go.sum`: Go module and dependency lock files.
- `docs/openapi.yaml`: Stage 1 API contract for unified response and `/health`.
- `configs/config.local.yaml`: Local runtime configuration.
- `cmd/server/main.go`: HTTP server entrypoint with graceful shutdown.
- `internal/config`: YAML config loading, defaults, validation.
- `internal/http`: Gin router and unified JSON response helpers.
- `internal/middleware`: request ID and access logging middleware.
- `internal/observability`: structured logger setup.
- `internal/storage`: optional GORM MySQL connection setup.

### Go Backend Notes

- `cmd/server` is the executable entrypoint; reusable code stays under `internal`.
- `internal` packages cannot be imported from outside this module, which helps protect service boundaries.
- Gin middleware runs in registration order. Stage 1 uses recovery, request ID, then access logging.
- Unified response makes later handlers return a consistent shape:

```json
{
  "data": {},
  "error": null,
  "request_id": "req_..."
}
```

- `slog` is Go's standard structured logging package. It avoids committing to a third-party logging framework early.
- GORM/MySQL connection exists now, but `configs/config.local.yaml` keeps `mysql.enabled: false` so `/health` can run before Docker Compose and schema work are ready.

### Java Spring Boot Comparison

- `cmd/server/main.go` is close to `public static void main` plus Spring Boot application startup.
- `internal/http/router.go` manually wires routes, similar to what Spring MVC does through annotations and auto-configuration.
- Gin middleware is conceptually similar to Servlet filters or Spring interceptors.
- `internal/config` replaces part of Spring Boot's `application.yml` binding, but the binding/default/validation logic is explicit Go code.
- Go does not provide dependency injection by default. Stage 1 wires dependencies manually to keep the project easy to read.

### Verification

- `go test ./internal/http -run TestHealth -v`
- `go test ./...`
- `go run ./cmd/server -config configs/config.local.yaml`
- `GET http://127.0.0.1:8080/health` returned:

```json
{
  "data": {
    "status": "ok"
  },
  "error": null,
  "request_id": "req_verify_stage1"
}
```

### Next Stage

Stage 2 should add account and authentication:

- `users` model and migration.
- Register/login/current-user APIs.
- Password hashing.
- JWT signing and auth middleware.

## 2026-05-30 Stage 2: Account And Authentication

### Stage Goal

让用户可以注册、登录，并通过 Bearer JWT 访问受保护接口。此阶段只实现账号身份链路，不实现内容发布、Feed 或管理后台。

### Files Created Or Modified

- `internal/account/model.go`: `users` GORM model.
- `internal/account/repository.go`: user repository interface and GORM implementation.
- `internal/account/service.go`: register, login, current-user business logic.
- `internal/account/handler.go`: account HTTP handlers and error mapping.
- `internal/account/dto.go`: response DTOs that do not expose `password_hash`.
- `internal/auth/password.go`: bcrypt password hashing and verification.
- `internal/auth/jwt.go`: HMAC-SHA256 JWT issuing and parsing.
- `internal/middleware/auth.go`: Bearer token authentication middleware.
- `internal/http/router.go`: `/api/users`, `/api/sessions`, `/api/users/me` route wiring.
- `cmd/server/main.go`: JWT manager, account service wiring, and development AutoMigrate for users.
- `configs/config.local.yaml`: local JWT settings.
- `docs/openapi.yaml`: Stage 2 account API contract.
- `internal/account/*_test.go`, `internal/auth/*_test.go`: unit and handler tests.

### Go Backend Notes

- The account module follows `handler -> service -> repository`.
- Handler layer knows HTTP status codes and JSON binding.
- Service layer owns business rules such as email normalization, duplicate checks, password verification, and disabled-user handling.
- Repository layer owns GORM queries and converts `gorm.ErrRecordNotFound` into domain-level `ErrUserNotFound`.
- Passwords are never stored as plain text. The service stores only bcrypt hashes.
- JWT payload includes `user_id` and `role`. The middleware parses the token and puts claims into Gin context.
- DTOs prevent leaking persistence-only fields such as `PasswordHash`.
- Tests use an in-memory SQLite database through a pure Go driver so they do not require local MySQL or cgo.

### Java Spring Boot Comparison

- `Handler` is similar to a Spring `@RestController`.
- `Service` is similar to a Spring `@Service`, but dependencies are passed manually instead of injected by the framework.
- `GormUserRepository` is similar to a Spring Data repository, but query behavior is explicit code.
- Gin auth middleware is similar to a Spring Security filter.
- `context.Context` is Go's standard way to carry request cancellation and deadlines; it is not the same as Spring's application context.
- DTO separation serves the same purpose as Java response classes or MapStruct output DTOs.

### Verification

- Red phase: account and auth tests initially failed because production packages did not exist.
- `go test ./internal/auth ./internal/account -run Test -count=1 -v`
- `go test -count=1 ./...`
- `go run ./cmd/server -config configs/config.local.yaml`
- `GET http://127.0.0.1:8080/health` returned a unified response with `request_id`.

Runtime acceptance with real MySQL (Docker `mysql-dev` on port 13306):

| Request | Status | Notes |
|---------|--------|-------|
| `POST /api/users` register | 201 | New user created, password hashed with bcrypt |
| `POST /api/users` duplicate email | 409 | `email_already_registered` |
| `POST /api/sessions` login | 200 | Returns JWT token + user DTO |
| `POST /api/sessions` wrong password | 401 | `invalid_credentials` |
| `GET /api/users/me` with Bearer token | 200 | Returns authenticated user profile |
| `GET /api/users/me` without token | 401 | `authentication is required` |

Key runtime observations:

- AutoMigrate creates the `users` table on server startup when MySQL is enabled.
- `mysql.enabled: true` in `config.local.yaml` is what activates the account routes; when false, the router has no `/api/*` handlers and returns 404.
- GORM logs `record not found` during the first registration's duplicate-check query — this is normal behavior, not an error.

### Next Stage

Stage 3 should add content publishing and upload workflow:

- Local image upload endpoint.
- Notes, images, tags, note-tags models.
- Authenticated content publishing.
- Content detail and own-content listing.
- New content starts as `pending_review`.

## 2026-05-30 Stage 3: Content Publishing And Uploads

### Stage Goal

让用户上传图片、发布笔记（进入 `pending_review` 状态）、查看笔记详情和自己的笔记列表。此阶段不实现审核流程、Feed 排序或交互功能。

### Files Created Or Modified

- `internal/note/model.go`: `notes`, `note_images`, `tags`, `note_tags` GORM models.
- `internal/note/dto.go`: `NoteDTO`, `AuthorDTO`, `ImageDTO`, `PublishInput`, `NoteListDTO`.
- `internal/note/repository.go`: note/image/tag CRUD, transaction helper via `d(db, tx)`.
- `internal/note/service.go`: `Publish`, `Detail`, `MyNotes` business logic with transaction.
- `internal/note/handler.go`: HTTP handlers for `/api/notes`, `/api/notes/:noteId`, `/api/users/me/notes`.
- `internal/note/handler_test.go`: integration tests covering publish, detail, visibility, validation.
- `internal/upload/handler.go`: multipart image upload to local disk.
- `internal/middleware/auth.go`: added `OptionalAuth` for public endpoints that benefit from viewer context.
- `internal/http/router.go`: added `NoteRoutes`, `UploadRoutes`, `UploadStaticDir` fields.
- `internal/config/config.go`: added `UploadConfig` with defaults.
- `cmd/server/main.go`: wired note/upload modules, `AuthorProvider` adapter, AutoMigrate all tables.
- `configs/config.local.yaml`: added `upload` section.

### Go Backend Notes

- GORM transaction: `db.Transaction(func(tx *gorm.DB) error { ... })` wraps multi-table writes (note + tags + images) in a single transaction.
- Repository helper `d(db, tx)` selects the transaction-scoped `*gorm.DB` when inside a transaction, or falls back to the default connection.
- `OptionalAuth` middleware: tries to parse the Bearer token but does not reject the request if missing. The handler uses `AuthClaimsFromContext` to get the viewer ID if present.
- New notes default to `status = pending_review`. Anonymous users get 404 for non-published notes. The author can see their own pending notes.
- Tags are normalized (lowercase, trimmed) and deduplicated before storage. `FindOrCreateTag` uses the "query then insert" pattern inside a transaction.
- Upload handler validates file extension, enforces max size, generates a timestamp-based filename, and saves to local disk. No database record is created for uploads — the file path is stored as a URL in `note_images` when publishing.
- `AuthorProvider` interface in the note package avoids circular imports. `main.go` implements it with a thin adapter over the user repository.

### Java Spring Boot Comparison

- `db.Transaction(...)` is equivalent to Spring `@Transactional`.
- `OptionalAuth` middleware is similar to Spring Security's `permitAll()` with optional principal extraction.
- GORM callbacks inside `Transaction` are similar to JPA's automatic dirty checking within a transaction boundary.
- File upload to local disk with `multipart/form-data` is conceptually identical to Spring's `MultipartFile` handling, but Go requires explicit file creation and copy.

### Verification

- `go test -count=1 ./...` — all packages pass.
- Runtime acceptance with real MySQL:

| Request | Status | Notes |
|---------|--------|-------|
| `POST /api/notes` with auth | 201 | Note created as `pending_review` |
| `GET /api/notes/:id` as author | 200 | Author sees pending note |
| `GET /api/notes/:id` anonymous | 404 | Pending note hidden from public |
| `GET /api/users/me/notes` | 200 | Returns own notes list with total |
| `POST /api/uploads/images` | 200 | Returns image URL |
| `POST /api/notes` missing title | 400 | Validation error |
| `POST /api/notes` invalid image URL | 400 | Validation error |

### Next Stage

Stage 4 should add review task state transitions:

- `review_tasks` and `review_task_events` models.
- Admin review APIs.
- Content status transitions.

## 2026-05-30 Stage 4: Content State Flow And Review Tasks

### Stage Goal

Build the review/governance task workflow. Publishing a note automatically creates a review task. Admin can approve or reject, which transitions the note status accordingly.

### Files Created Or Modified

- `internal/review/model.go`: `review_tasks` and `review_task_events` GORM models with status constants.
- `internal/review/dto.go`: `TaskDTO`, `TaskListDTO`, `DecisionInput`.
- `internal/review/repository.go`: CRUD for tasks and events with transaction support.
- `internal/review/service.go`: `CreateInTx`, `List`, `GetByID`, `Decide` — multi-table transaction for task + note + event.
- `internal/review/handler.go`: Admin review APIs with `requireAdmin` middleware.
- `internal/review/handler_test.go`: Integration tests for approve/reject/non-admin/invalid-decision flows.
- `internal/note/service.go`: Added `ReviewTaskCreator` interface, creates review task in publish transaction.
- `internal/note/repository.go`: Added `UpdateNoteStatus`, `UpdateReviewTaskID`.
- `internal/http/router.go`: Added `ReviewRoutes` field.
- `cmd/server/main.go`: Wired review module, passed `reviewService` as `ReviewTaskCreator` to note service.

### Go Backend Notes

- **Transaction boundary**: The review `Decide` method wraps task read + update, note status update, and event creation in a single `db.Transaction()`. If any step fails, all roll back.
- **Interface for cross-module dependency**: The note package defines `ReviewTaskCreator` interface. The review package's `Service` satisfies it. `main.go` wires them together. This avoids circular imports.
- **Admin authorization**: The review handler adds `requireAdmin` middleware that checks `claims.Role != "admin"`. This is chained after `requireAuth` in a Gin route group.
- **Audit trail**: Every state transition creates a `review_task_event` record with actor type, actor ID, and JSON payload.
- **Idempotency guard**: `Decide` rejects requests on already-decided tasks with `ErrAlreadyDecided`.

### Java Spring Boot Comparison

- `db.Transaction(...)` ≈ Spring `@Transactional`.
- `ReviewTaskCreator` interface ≈ Spring `@Autowired` with interface injection.
- `requireAdmin` middleware chain ≈ Spring Security's `@PreAuthorize("hasRole('admin')")`.
- Audit event table ≈ Spring Data Envers or a custom `@EntityListener`.

### Verification

- `go test -count=1 ./...` — all packages pass including 4 review tests.
- Publishing a note now creates a `review_tasks` row with `status = pending_agent`.
- Admin approve → note status becomes `published`, `published_at` is set.
- Admin reject → note status becomes `rejected`.
- Non-admin gets 403 on admin endpoints.
- Invalid decision gets 400.
- Already-decided task gets 409.

### Next Stage

Stage 5 should expose internal content analysis APIs:

- `X-Internal-Token` authentication middleware.
- Pending task pull, review context, agent result callback.
- Idempotent agent callbacks.

## 2026-05-30 Stage 5: Internal APIs For 知涟洞察

### Stage Goal

Expose stable service-to-service APIs under `/internal` for the Agent project (知涟洞察) to pull pending review tasks, fetch review context, and submit agent results.

### Files Created Or Modified

- `internal/middleware/internal_auth.go`: `X-Internal-Token` authentication middleware.
- `internal/review/internal_handler.go`: Internal handler with 4 endpoints.
- `internal/http/router.go`: Added `InternalRoutes` interface and `InternalToken` field.
- `cmd/server/main.go`: Wired internal handler with `cfg.Review.InternalToken`.
- `internal/review/handler_test.go`: Added 3 internal API tests.

### Go Backend Notes

- **Service-to-service auth**: `X-Internal-Token` header is a simple shared secret. This is common in V1 internal APIs. Production systems often upgrade to mTLS or JWT-based service tokens.
- **Internal handler interface**: `InternalRoutes` takes `internalAuth gin.HandlerFunc` instead of `requireAuth`, since internal APIs don't use JWT user tokens.
- **Agent result callback**: The `SubmitAgentResult` endpoint handles three decisions: `pass` (auto-publish), `reject` (auto-reject), and `manual_review` (escalate to admin). Each updates both the review task and note status in a transaction.
- **Idempotency**: The agent callback rejects already-decided tasks with `ErrAlreadyDecided`, preventing double-processing.
- **Trace ID**: Agent results include a `trace_id` field stored in `agent_trace_id`, linking the platform review task to the agent's internal trace for debugging.

### Java Spring Boot Comparison

- `X-Internal-Token` middleware ≈ Spring Security filter chain with custom `X-Internal-Token` header check.
- Internal vs public route separation ≈ Spring's `@RequestMapping` under different security configurations.
- Agent callback idempotency ≈ Spring's `@Version` or explicit status checks in service layer.

### Verification

- `go test -count=1 ./...` — all packages pass including 7 review tests.
- Internal APIs require `X-Internal-Token` header; missing token returns 401.
- Agent `pass` → task status `agent_passed`, note status `published`.
- Agent `reject` → task status `agent_rejected`, note status `rejected`.
- Agent `manual_review` → task status `manual_required`, note stays `pending_review`.

### Next Stage

Stage 6 should add cursor-based feed endpoints:

- Latest feed, hot feed, following feed, tag feed.
- Compound cursor pagination.
- Feed DTO hydration.

## 2026-05-30 Stage 6: Feed Scenes

### Stage Goal

Implement cursor-based pagination for latest, hot, following, and tag feeds. Only `published` content appears in public feeds.

### Files Created

- `internal/feed/model.go`: `Cursor` struct with base64 JSON encoding/decoding, cursor application helpers.
- `internal/feed/repository.go`: GORM queries for latest/hot/following/tag with cursor support.
- `internal/feed/service.go`: `Latest`, `Hot`, `Following`, `ByTag` business logic, DTO hydration.
- `internal/feed/handler.go`: HTTP handlers for feed endpoints.
- `internal/feed/handler_test.go`: 4 integration tests covering latest, exclusion, tag, and cursor pagination.
- `internal/follow/provider.go`: Stub `FollowProvider` for Stage 7.

### Files Modified

- `internal/http/router.go`: Added `FeedRoutes` field.
- `cmd/server/main.go`: Wired feed module with stub follow provider.

### Go Backend Notes

- **Cursor pagination**: Uses compound cursors `(published_at, id)` for latest/following and `(hot_score, id)` for hot. The cursor is base64-encoded JSON. This avoids offset-based pagination problems (skipped/duplicated items when data changes between page loads).
- **Fetch N+1**: The `limit+1` pattern fetches one extra record to determine `has_more` without a separate COUNT query.
- **Feed safety**: All feed queries filter `status = published AND published_at IS NOT NULL`, ensuring pending/rejected content never appears.
- **Tag feed JOIN**: The tag feed uses `JOIN note_tags ON note_tags.note_id = notes.id` for efficient tag filtering.
- **Following feed**: Takes a `FollowProvider` interface. Stage 7 will provide a real implementation; Stage 6 uses a stub returning empty.

### Java Spring Boot Comparison

- Cursor pagination ≈ Spring Data's `Pageable` with keyset pagination (not offset-based).
- `limit+1` has_more pattern ≈ using `PageRequest.of(0, limit+1)` in Spring Data.
- Feed DTO hydration ≈ Spring's `ModelMapper` or MapStruct for converting entities to DTOs.
- `FollowProvider` interface ≈ Spring's `@Autowired` with lazy/noop fallback.

### Verification

- `go test -count=1 ./...` — all packages pass including 4 feed tests.
- Latest feed returns only published notes.
- Pending/rejected notes are excluded from all feeds.
- Tag feed filters by tag name correctly.
- Cursor pagination with `limit=2` returns correct pages with `has_more=true` and no overlap.
- Nonexistent tag returns empty feed.

### Next Stage

Stage 7 should add interactions and social graph:

- Like, favorite, comment, follow APIs.
- Idempotent design for repeated operations.

## 2026-05-30 Stage 7: Interactions And Social Graph

### Stage Goal

Add like, favorite, comment, and follow APIs with idempotent design for repeated operations.

### Files Created

- `internal/interaction/model.go`: `note_likes`, `note_favorites`, `comments`, `follows` GORM models.
- `internal/interaction/dto.go`: `CommentDTO`, `CommentListDTO`.
- `internal/interaction/repository.go`: Idempotent upsert/delete for likes/favorites/follows, comment CRUD, follow graph queries.
- `internal/interaction/handler.go`: HTTP handlers for all interaction endpoints.
- `internal/interaction/handler_test.go`: 4 tests covering like/unlike idempotency, favorite, comment, follow/unfollow.

### Files Modified

- `internal/follow/provider.go`: Replaced stub with real `Provider` using interaction repository.
- `internal/http/router.go`: Added `InteractionRoutes` field.
- `cmd/server/main.go`: Wired interaction module, replaced follow stub with real provider.

### Go Backend Notes

- **Idempotent like**: `UpsertLike` uses `Unscoped().First()` to find soft-deleted records. If a previously unliked record exists, it restores it (sets `deleted_at = nil`) instead of creating a duplicate. This prevents unique index violations.
- **Soft delete with unique index**: The `note_likes` and `note_favorites` tables use composite unique indexes that include `deleted_at`, allowing a re-like after unlike without violating uniqueness.
- **Counter safety**: `DeleteLike` checks `RowsAffected` before decrementing, and the UPDATE query has `likes_count > 0` guard to prevent negative counts.
- **Follow graph**: `FollowingIDs` uses `Pluck` to efficiently fetch just the IDs needed by the feed query.
- **Self-follow prevention**: Handler rejects `userID == targetID` with 400.

### Java Spring Boot Comparison

- Idempotent upsert ≈ Spring Data JPA's `findById` + `save` pattern with `@Transactional`.
- Soft delete with unique index ≈ Hibernate's `@SQLDelete` with composite unique constraint.
- Counter update with `gorm.Expr("likes_count + 1")` ≈ JPQL `UPDATE ... SET likes_count = likes_count + 1`.

### Verification

- `go test -count=1 ./...` — all packages pass including 4 interaction tests.
- Repeated like does not double-count; count stays at 1.
- Repeated unlike does not make counts negative; count stays at 0.
- Repeated follow returns `following: false` with message "already following".
- Self-follow returns 400.
- Comment creation increments `notes.comments_count`.
- Comment listing is public (no auth required).

### Next Stage

Stage 8 should add Redis caching:

- Feed first-page cache, content detail cache, interaction count cache.
- Active invalidation after content state changes.

## 2026-05-30 Stage 8: Redis Cache

### Stage Goal

Add Redis caching for feed first-page and content reads, with TTL-based expiration and active invalidation helpers.

### Files Created

- `internal/cache/redis.go`: Redis client wrapper with Get/Set/Delete/Exists using JSON serialization.
- `internal/cache/feed_cache.go`: `FeedCache` wrapper that caches latest feed first page; cache key helpers and invalidation methods.

### Files Modified

- `internal/config/config.go`: Added `RedisConfig` struct and default addr.
- `internal/feed/handler.go`: Changed `Handler` to accept `FeedService` interface instead of concrete `*Service`.
- `internal/feed/service.go`: `Service` already satisfies `FeedService` interface.
- `internal/feed/handler_test.go`: Uses `Service` which satisfies `FeedService`.
- `cmd/server/main.go`: Wired Redis client, wraps feed service with cache when enabled.
- `configs/config.local.yaml`: Added `redis` section.

### Go Backend Notes

- **Interface-driven caching**: The `FeedService` interface allows transparently swapping between `Service` and `FeedCache` without changing the handler. This is the decorator pattern.
- **Cache-aside pattern**: `FeedCache.Latest` first tries `client.Get`. On `ErrCacheMiss`, it falls through to the real service, then writes the result back to Redis with a TTL.
- **TTL strategy**: Feed first page uses 30s TTL. Short TTL balances freshness with cache hit rate.
- **Active invalidation**: `InvalidateFeedCache` and `InvalidateNoteCache` methods delete specific keys. These should be called when content status changes or interactions update counts.
- **Graceful degradation**: If Redis is unavailable (`redis.enabled: false`), the system falls through to direct database queries with no behavior change.
- **Cache keys**: Follow a namespaced convention: `feed:latest:first-page`, `note:detail:{id}`, `note:counts:{id}`, `user:profile:{id}`.

### Java Spring Boot Comparison

- Redis client wrapper ≈ Spring Data Redis `RedisTemplate<String, Object>`.
- Cache-aside pattern ≈ Spring's `@Cacheable` with manual cache put on miss.
- TTL strategy ≈ Spring Cache `@CacheConfig` with Redis TTL configuration.
- Graceful degradation ≈ Spring's `@Cacheable` with fallback to `@Cacheable(sync=true)` or circuit breaker.

### Verification

- `go test -count=1 ./...` — all packages pass.
- With `redis.enabled: true`, server logs "redis connected" on startup.
- Latest feed first page is cached in Redis with 30s TTL.
- Subsequent requests hit Redis cache (verified by observing reduced DB queries).
- Feed cache wrapper falls through to DB on cache miss.
- When `redis.enabled: false`, system works identically without cache.

### Next Stage

Stage 9 should add RabbitMQ and outbox for domain events:

- Outbox events table.
- Event publisher worker.
- RabbitMQ producer and consumer.

## 2026-05-30 Stage 9: RabbitMQ And Outbox

### Stage Goal

Implement the Outbox pattern for reliable domain event publishing. Business writes (e.g., note publishing) create outbox events in the same database transaction. A background worker picks up pending events and publishes them to a message broker.

### Files Created

- `internal/outbox/model.go`: `outbox_events` GORM model with status constants and event topics.
- `internal/outbox/repository.go`: CRUD for pending events, mark sent/failed with retry tracking.
- `internal/outbox/publisher.go`: `Worker` background loop, `Publisher` interface, `NopPublisher` stub.
- `internal/outbox/helper.go`: `Helper` that creates outbox events within a transaction.

### Files Modified

- `internal/note/service.go`: Added `OutboxEventCreator` interface, creates `note.review_requested` event in publish transaction.
- `cmd/server/main.go`: Wired outbox repository, helper, and worker with `NopPublisher` (RabbitMQ not yet available).
- Auto-migrate includes `outbox_events` table.

### Go Backend Notes

- **Outbox pattern**: Instead of publishing to a message broker directly during a business transaction, write an `outbox_events` row in the same transaction. A separate worker reads pending events and publishes them. This guarantees "at least once" delivery even if the broker is temporarily unavailable.
- **Transaction boundary**: The outbox event is written using the same `*gorm.DB` transaction (`tx`) as the business write. If the business write fails, the outbox event is rolled back too — no orphaned events.
- **Worker loop**: `Worker.run` uses a `time.Ticker` for periodic polling. It fetches pending events, attempts to publish each, marks successful ones as `sent`, and marks failed ones with an incremented retry count and a `next_retry_at` timestamp.
- **`NopPublisher`**: A no-op implementation of `Publisher` that succeeds immediately. Used when RabbitMQ is not available. Can be replaced with a real `RabbitMQPublisher` when RabbitMQ is deployed.
- **Graceful shutdown**: The worker uses a `stopCh` channel and `sync.WaitGroup` for clean shutdown. `defer outboxWorker.Stop()` in main ensures events are flushed before the process exits.
- **Interface alignment**: `note.OutboxEventCreator` takes `any` for payload. `outbox.Helper.CreateEvent` also takes `any` and JSON-marshals it internally. This keeps the note module decoupled from outbox's internal `EventPayload` struct.

### Java Spring Boot Comparison

- Outbox pattern ≈ Spring's `@TransactionalEventListener(phase = AFTER_COMMIT)` with a polling publisher, or Debezium CDC connector.
- Worker loop ≈ Spring's `@Scheduled` with `@Transactional` polling.
- `NopPublisher` ≈ a `@Profile("dev")` no-op bean that gets replaced in production.
- Same-transaction outbox write ≈ writing to both business table and outbox table in one `@Transactional` method.

### Verification

- `go test -count=1 ./...` — all packages pass.
- Publishing a note creates an `outbox_events` row with `topic = "note.review_requested"` and `status = "pending"`.
- The outbox worker polls for pending events and marks them as `sent` (with `NopPublisher`).
- Failed publishes are marked with `status = "failed"` and `retry_count` incremented.
- `next_retry_at` prevents immediate retry of failed events.
- Worker starts with the server and stops gracefully on shutdown.

### Backend Audit Fix (2026-05-30)

#### Audit Findings and Fixes

A code audit identified 10 issues across the backend. All were fixed:

**P1 fixes:**
1. **RabbitMQ + Outbox**: Replaced NopPublisher with real RabbitMQPublisher (topic exchange). Added `cmd/worker/main.go` as independent worker process. Config now supports `rabbitmq.enabled`, `rabbitmq.dsn`, `rabbitmq.exchange`.
2. **Outbox error handling**: Changed ignored outbox error to return error, causing the publish transaction to roll back if outbox write fails. This preserves "business + outbox same transaction" semantics.
3. **Redis cache completion**: Added hot feed first-page caching, cache invalidation after review approve/reject (feed + note caches), cache invalidation after like/unlike/favorite/unfavorite/comment (note counts cache). Wired via CacheInvalidator interface injected into review service and interaction handler.
4. **Feed visibility filter**: All 4 feed queries (latest, hot, following, tag) now filter `visibility = 'public'` in addition to `status = 'published'`.

**P2 fixes:**
5. **Feed viewer flags**: Added ViewerStateProvider interface. Feed items now include `viewer_liked`, `viewer_favorited`, `viewer_following` for logged-in users. Anonymous users see nil (omitted).
6. **Agent review context**: Internal handler now uses AuthorInfoProvider to fetch real user profile (nickname, bio) and stats (notes_count, published_count, rejected_count, registered_days) instead of fake placeholder. Also fetches real tags.
7. **Idempotent agent callback**: Same decision + trace_id repeated callback returns success (200 with current task state). Only conflicting decisions get 409.
8. **JSON safety**: Replaced all fmt.Sprintf JSON building with json.Marshal in review service and internal handler. User input (reason, trace_id) with quotes/escapes no longer breaks JSON.
9. **Transactional interaction writes**: Like, unlike, favorite, unfavorite, and comment creation now use `db.Transaction()` to wrap detail insert + counter update atomically.
10. **Interaction note status check**: Like, favorite, and comment endpoints now verify `note status = published AND visibility = public` before allowing interaction.

#### Verification

- `go build ./...` compiles including new `cmd/worker`
- `go test -count=1 ./...` all 9 test packages pass
- Test updates only needed for constructor signature changes (NewService, NewInternalHandler)

## 2026-05-30 Stage 10: React Frontend

### Stage Goal

Build a complete React frontend for the content community, covering feed browsing, user auth, note publishing, note detail, profile, and admin review. The UI style is inspired by 小红书 (Xiaohongshu): masonry waterfall cards, rounded corners, soft cyan color palette, and image-first layout.

### Files Created

- `web/` — Entire React frontend SPA.
  - `vite.config.ts`: Vite config with TailwindCSS v4 plugin, path alias (`@/`), and dev proxy (`/api` and `/uploads` → backend).
  - `src/types/index.ts`: TypeScript types matching all backend DTOs (`User`, `Note`, `FeedItem`, `FeedResult`, `Comment`, `ReviewTask`, `ApiEnvelope`).
  - `src/api/client.ts`: Fetch wrapper with auto Bearer token from localStorage, unified error handling via `ApiError` class.
  - `src/api/auth.ts`, `feed.ts`, `notes.ts`, `upload.ts`, `interaction.ts`, `review.ts`: Typed API modules.
  - `src/context/AuthContext.tsx`: Auth provider with login/register/logout/refresh, persists token in localStorage.
  - `src/hooks/useInfiniteScroll.ts`: Intersection Observer hook for infinite scroll.
  - `src/components/ui/`: shadcn/ui-style components (Button, Input, Textarea, Card, Avatar, Badge, Skeleton, Label, Tabs, Dialog, DropdownMenu, Alert).
  - `src/components/layout/`: Navbar, MobileNav, Layout, BackToTop.
  - `src/components/feed/`: FeedCard (waterfall card), FeedSkeleton.
  - `src/components/interaction/`: LikeButton (heart pop animation), FavoriteButton (star flash), FollowButton.
  - `src/components/common/`: EmptyState, ErrorState.
  - `src/pages/`: FeedPage, HotPage, LoginPage, RegisterPage, PublishPage, NoteDetailPage, ProfilePage, AdminReviewPage.

### Tech Stack

- React 19 + Vite + TypeScript
- TailwindCSS v4 (Vite plugin, CSS-first `@theme` configuration)
- React Router v7 (client-side routing with layout outlet)
- TanStack Query v5 (data fetching, caching, mutations)
- Radix UI primitives (Dialog, DropdownMenu, Tabs, Label)
- Lucide React (icons)
- Sonner (toast notifications)
- class-variance-authority (component variants)

### Frontend Notes

- **CSS-first TailwindCSS v4**: No `tailwind.config.js` — theme colors are defined in `src/index.css` using `@theme {}` block. Vite plugin (`@tailwindcss/vite`) replaces PostCSS setup.
- **Masonry grid**: Pure CSS `column-count` layout with responsive breakpoints (2/3/4 columns). No JS layout library needed.
- **Infinite scroll**: `useInfiniteScroll` hook wraps `IntersectionObserver` with `rootMargin: 200px` to preload before reaching the bottom.
- **Auth flow**: Token stored in `localStorage`. `apiRequest` auto-attaches `Bearer` header. `AuthContext` fetches `/api/users/me` on mount to validate token.
- **Optimistic-like UX**: Like/favorite buttons use local state with animation classes (`heart-pop`, `star-flash`) and API call in parallel. Errors roll back.
- **Image upload**: Drag-and-drop plus click-to-browse. Files uploaded individually via `FormData` to `/api/uploads/images`. Preview thumbnails with remove button.
- **FeedList component**: Reusable across tabs (latest/hot/following) via a `fetcher` prop, accumulating items with cursor pagination.
- **Admin review**: Dialog-based review with inline note preview fetched via TanStack Query. Approve/reject with optional reason.

### Verification

- `npm run build` passes with zero TypeScript errors.
- All 7 pages render with proper loading/empty/error states.
- Dev proxy routes `/api/*` and `/uploads/*` to `http://127.0.0.1:8080`.

### Demo Flow

Browser can complete: Register → Login → Publish Note → Admin Review → Feed Display → Like/Comment/Follow

## 2026-06-01 Optimization Stage A: Error Codes And Lightweight Rate Limiting

### Stage Goal

借鉴 Java 后端项目里的统一错误码和接口保护思路，但不引入复杂风控系统。当前阶段只为注册、登录、发布、互动和关注增加轻量 Redis 固定窗口限流，并补齐错误码文档。AI/审查 Agent 相关接口后续暂缓优化。

### Files Created

- `internal/ratelimit/limiter.go`: Gin 限流中间件、规则匹配、key 生成和内存测试 store。
- `internal/ratelimit/redis_store.go`: Redis 固定窗口 `INCR + EXPIRE` store。
- `internal/ratelimit/limiter_test.go`: 限流触发、规则隔离、窗口过期测试。
- `docs/14-optimization-implementation-plan.md`: 项目优化方案和后续实施计划。

### Files Modified

- `cmd/server/main.go`: Redis 启用时注入默认限流规则。
- `internal/http/router.go`: 增加可选 `RateLimiter` 全局中间件。
- `internal/http/router_test.go`: 验证路由层限流会返回 `429 rate_limited`。
- `internal/cache/redis.go`: 暴露底层 Redis client 给限流 store 使用。
- `docs/04-api-design.md`: 增加错误码分类表。
- `docs/cache-and-consistency.md`: 更新限流 key、窗口和降级策略。
- `resume.md`: 改为更接近后端简历项目经历的写法。

### Go Backend Notes

- **固定窗口限流**：每个窗口内对同一个 key 执行 `INCR`，首次创建时设置 `EXPIRE`。超过阈值后返回 `429`。
- **中间件顺序**：限流挂在全局中间件，执行早于路由内部的 `requireAuth`。因此用户态限流不能依赖 Gin context 中的 claims，而是在 key 函数里解析 Authorization token，失败则降级为 IP 维度。
- **降级设计**：Redis 是保护能力，不是业务事实来源。Redis `INCR/EXPIRE` 失败时放行请求，避免 Redis 故障导致核心业务不可用。
- **依赖方向**：`internal/http` 可以依赖 `internal/ratelimit`，但 `internal/ratelimit` 不能反向依赖 `internal/http`，否则 Go 会报 import cycle。限流包自己输出统一 JSON 形状来避免循环依赖。
- **测试隔离**：生产使用 Redis store，单元测试使用内存 store，测试限流语义而不是依赖真实 Redis。

### Java Spring Boot Comparison

- Gin middleware ≈ Spring MVC `HandlerInterceptor` 或 Spring Security filter。
- `Store` 接口 + Redis 实现 ≈ Spring 中的 `RateLimiter` service + `StringRedisTemplate`。
- `INCR + EXPIRE` 固定窗口 ≈ RedisTemplate `opsForValue().increment()` 后设置 TTL。
- Redis 故障放行 ≈ Java 项目里把限流作为保护层而非核心业务依赖，通常配合日志和监控。
- Go 的 import cycle 限制比 Java 更严格，倒逼包边界更清楚。

### Verification

- `go test ./internal/ratelimit -run Test -v` passed。
- `go test ./internal/http -run TestRouterAppliesRateLimiter -v` passed。
- `go test ./internal/ratelimit ./internal/http ./internal/cache` passed。

### Follow-Up

- Stage B: 收敛代码中的错误码常量，减少散落字符串。
- Stage C: 把 `note:detail:{id}` 和 `note:counts:{id}` 接入 cache-aside 读路径。
- Stage D: 补齐 `note.review_decided` 与 interaction outbox events。
- Stage E: 跑 k6 压测并只把真实数据写进简历。

## 2026-06-01 Optimization Stage C: Note Detail Cache-Aside

### Stage Goal

把文档中已经约定的 `note:detail:{id}` 和 `note:counts:{id}` 接入真实读路径，增强 Redis 缓存与一致性设计的完整性。

### Files Created

- `internal/cache/note_cache.go`: Note service 的 cache-aside 装饰器。
- `internal/cache/note_cache_test.go`: 匿名详情缓存、计数快照、登录态绕过、Redis 失败降级测试。

### Files Modified

- `internal/note/handler.go`: Handler 依赖 `ServiceAPI` 接口，不再强依赖具体 `*Service`，便于注入缓存装饰器。
- `cmd/server/main.go`: Redis 启用时为 note detail 注入 `cache.NewNoteServiceCache`。
- `docs/cache-and-consistency.md`: 更新 note detail/counts 缓存状态和规则。

### Go Backend Notes

- **装饰器模式**：`NoteServiceCache` 包住原始 note service，只增强读路径缓存，不侵入 note 业务服务。
- **接口倒置**：handler 只需要 `Publish/Detail/MyNotes` 三个方法，因此定义 `ServiceAPI`，让原始 service 和缓存 service 都能被注入。
- **Cache Aside**：匿名详情先查 Redis，miss 后查 MySQL，组装 DTO 后写入 Redis。
- **共享缓存边界**：当前只缓存匿名公开详情。登录态 detail 暂不读共享缓存，避免未来添加 viewer state 时出现用户态串缓存。
- **降级策略**：Redis GET/SET 失败不会影响 MySQL 读结果。

### Java Spring Boot Comparison

- 装饰器类似 Spring 中用一个 service wrapper 或 AOP `@Around` 包住查询方法。
- `ServiceAPI` 类似 Java 中按业务能力抽 interface，controller 依赖接口而不是具体实现。
- Cache Aside 类似 `RedisTemplate.get -> repository.query -> RedisTemplate.set`，但 Go 中显式错误返回更容易把降级策略写清楚。

### Verification

- `go test ./internal/cache ./internal/note ./cmd/server` passed。

### Follow-Up

- 互动写入已经会失效 note detail/counts key，后续可补集成测试覆盖“点赞后旧 counts 缓存被删除”。
- 资料页 `user:profile:{id}` 仍未接入，可放到后续阶段。

## 2026-06-01 Optimization Stage D: Outbox Contract Tightening

### Stage Goal

让 RabbitMQ + Outbox 从“有事件表和 worker”升级为“核心写链路有明确事件合约和失败终态”。本阶段不继续扩展 AI/审查 Agent，只处理后端主线：管理员审核决策、互动写路径和 worker 重试边界。

### Files Created

- `internal/outbox/publisher_test.go`: worker 失败重试和 `abandoned` 终态测试。
- `internal/review/service_test.go`: admin 审核决策产生 `note.review_decided` outbox event 的服务层测试。
- `docs/15-backend-optimization-log.md`: 后端优化过程、方案、实现、验证和复习清单。

### Files Modified

- `internal/outbox/model.go`: 新增 `StatusAbandoned` 和 `TopicInteractionRemoved`。
- `internal/outbox/repository.go`: 新增 `MarkAbandoned`。
- `internal/outbox/publisher.go`: 新增 `DefaultMaxRetries = 5`，超过上限后停止自动重试。
- `internal/review/service.go`: admin decision 事务内写入 `note.review_decided`。
- `internal/interaction/repository.go`: 互动真实状态变更时写入 `interaction.created` / `interaction.removed`。
- `cmd/server/main.go`: review、note、interaction 写路径共用同一个 `outbox.Helper`。
- `docs/events-and-outbox.md`: 更新事件目录、payload、状态机、测试清单和失败策略。
- `docs/14-optimization-implementation-plan.md`: 标记 Stage C/D 已实施，并把 AI 相关内容降为未来扩展。
- `docs/cache-and-consistency.md`: 移除当前阶段的 Agent callback 限流重点。
- `resume.md`: 调整为后端主线表达。

### Go Backend Notes

- **Transactional Outbox**: 业务写入和 outbox event 写入放在同一个 MySQL 事务中。业务成功则事件一定存在，业务失败则事件也回滚。
- **At-least-once delivery**: worker 成功发布后再标记 `sent`。如果发布成功但标记失败，下一轮可能重复发布，因此消费者必须幂等。
- **Idempotent producer**: interaction repository 只在真实状态变化时写事件。重复点赞、重复取消等幂等请求不重复产事件。
- **Failure terminal state**: `failed` 代表可自动重试，`abandoned` 代表超过自动重试上限，需要人工检查或后续 replay 工具。
- **接口注入**: `review` 和 `interaction` 都定义本地 `OutboxEventCreator` 接口，避免业务包直接依赖 RabbitMQ 发布实现。

### Java Spring Boot Comparison

- Outbox event 写入 ≈ Spring `@Transactional` 方法里同时写业务表和 `outbox_events` 表。
- Worker 轮询 ≈ Spring `@Scheduled` 定时任务或独立 worker 服务。
- `failed/next_retry_at/abandoned` ≈ Java MQ 项目里的 retry table、dead-letter 或人工补偿表。
- Go 中通过小接口注入 outbox helper，类似 Java 里 service 依赖接口而不是具体 MQ client。
- Go 的显式事务闭包让“哪些写入同属一个事务”更直观；Spring 依赖注解事务，需要注意 self-invocation 和传播行为。

### Key Code Paths

- 发布内容：`note.Service.Publish` -> `outbox_events(note.review_requested)`。
- 审核决策：`review.Service.Decide` -> `outbox_events(note.review_decided)`。
- 点赞/收藏/评论/关注：`interaction.Repository` -> `outbox_events(interaction.created)`。
- 取消点赞/取消收藏/取消关注：`interaction.Repository` -> `outbox_events(interaction.removed)`。
- 事件投递：`outbox.Worker.processBatch` -> `Publisher.Publish` -> `MarkSent/MarkFailed/MarkAbandoned`。

### Common Pitfalls

- 不要在业务事务里直接调用 RabbitMQ；否则数据库提交成功但 MQ 失败会丢事件。
- 不要对幂等请求重复产事件；否则后续通知、热榜、统计会重复处理。
- 不要把 `failed` 当终态；需要 `abandoned` 或 dead-letter 让问题可观察。
- 不要假设 MQ exactly-once；RabbitMQ + Outbox 的常规语义是 at-least-once。
- 不要让业务包依赖具体 MQ client；业务只需要写 outbox event。

### Verification

- `go test ./internal/interaction -run TestRepositoryCreatesOutbox -v` passed。
- `go test ./internal/outbox -run TestWorker -v` passed。
- `go test ./internal/review -run TestServiceDecideCreatesReviewDecidedOutboxEvent -v` passed。
- `go test ./internal/interaction ./internal/outbox ./cmd/server -v` passed。

### Follow-Up

- Stage E: 跑 Redis enabled/disabled Feed 压测，把真实 P95/P99 和吞吐写入 `docs/12-load-test.md`。
- P1: 增加 outbox replay CLI 或 admin 工具，支持查看和重放 `abandoned` event。
- P1: hot Feed 可由 `interaction.created/removed` consumer 增量维护 Redis ZSET。

## 2026-06-01 Optimization Stage E: Benchmark And Resume Evidence

### Stage Goal

把 Feed 性能优化整理成可复现、可核对、可写进简历的证据链。当前只引用已经真实执行过的压测结果，不为未复跑场景编造数字。

### Files Created

- `scripts/loadtest/feed_hot_anonymous.js`: 匿名热门 Feed 独立 k6 场景。
- `scripts/loadtest/run-k6.ps1`: PowerShell 封装脚本，用 Docker 版 k6 运行不同场景并保存结果。

### Files Modified

- `docs/12-load-test.md`: 增加一键运行入口、hot Feed 单场景命令、结果保存规则和本地复跑限制说明。
- `README.md`: 增加压测结果摘要，把项目首页从功能说明升级为可量化后端项目展示。
- `resume.md`: 增加 Feed SQL 查询数、RPS、P95 等真实数据。
- `docs/15-backend-optimization-log.md`: 追加 Stage E 复盘。

### Go Backend Notes

- **压测证据链**：性能数据必须包含环境、数据规模、命令、脚本、结果和限制，缺任一项都不适合直接写进简历。
- **容量压测 vs 用户行为压测**：当前 `SLEEP=0` 表示 VU 完成请求后立即下一次请求，偏容量上限，不代表普通用户真实停留行为。
- **缓存效果解释**：匿名 latest/hot 首页能被 Redis 短 TTL 缓存加速；登录态 Feed 仍需要实时 viewer state，因此主要收益来自批量 hydration 和索引。
- **数据诚实**：本轮没有在本机新增压测结果，因为 Docker daemon 未启动；README 和简历只引用 `docs/12-load-test.md` 已有云服务器真实结果。

### Java Spring Boot Comparison

- `scripts/loadtest/run-k6.ps1` 类似 Java 项目里用 Maven/Gradle task 或 shell 脚本封装 JMeter/k6 命令。
- `loadseed` 类似 Spring Boot 项目中的 data generator 或 CommandLineRunner，但 Go 这里作为独立 `cmd/loadseed` 更适合部署和压测环境复用。
- 压测报告中的环境和限制说明，类似生产压测报告里的 test scope / not SLA disclaimer。

### Verification

- `go test ./...` passed。
- `docker --version` 可用。
- `k6 version` 不可用，本机未安装 k6 CLI。
- Docker 版 k6 运行失败，错误为 Docker daemon 未启动：`failed to connect to the docker API at npipe:////./pipe/dockerDesktopLinuxEngine`。

### Follow-Up

- 在 Docker daemon 可用或云服务器环境中运行：
  - `.\scripts\loadtest\run-k6.ps1 -Scenario latest-anonymous -Vus 100 -Duration 2m -Sleep 0`
  - `.\scripts\loadtest\run-k6.ps1 -Scenario hot-anonymous -Vus 100 -Duration 2m -Sleep 0`
  - `.\scripts\loadtest\run-k6.ps1 -Scenario latest-auth -Vus 50 -Duration 2m -Sleep 0`
  - `.\scripts\loadtest\run-k6.ps1 -Scenario mixed -Vus 100 -Duration 2m -Sleep 0`
- 补跑 Redis disabled 配置，与 Redis enabled 做匿名首页对比。

## 2026-06-01 Optimization Stage F: Complete Comment Deletion Flow

### Stage Goal

补齐 API 文档中已经声明的 `DELETE /api/comments/{commentId}`，完善评论互动闭环，并让 `interaction.removed` 覆盖删除评论场景。

### Files Modified

- `internal/interaction/repository.go`: 新增 `DeleteComment`，在事务内 soft delete 评论、减少 `comments_count`、写 `interaction.removed`。
- `internal/interaction/handler.go`: 新增 `DELETE /api/comments/:commentId` 路由和 handler。
- `internal/interaction/repository_test.go`: 覆盖删除评论 outbox 事件和幂等行为。
- `internal/interaction/handler_test.go`: 覆盖作者删除、重复删除、非作者禁止删除。
- `docs/events-and-outbox.md`: 更新 `interaction.removed` 当前状态和删除评论 payload。
- `docs/cache-and-consistency.md`: 增加删除评论一致性规则。
- `docs/15-backend-optimization-log.md`: 追加 Stage F 复盘。

### Go Backend Notes

- **软删除幂等**：GORM 默认查询不返回 soft-deleted row，因此重复删除会查不到评论，应返回 `deleted=false`，不能继续扣计数。
- **权限检查在事务内**：先查当前未删除评论，再校验 `author_id`，非作者返回 `403`，不做任何写入。
- **事件生产条件**：只有首次真实删除写 `interaction.removed`，重复删除不写事件，避免通知、热榜、统计 consumer 重复处理。
- **缓存失效时机**：handler 只在 repository 事务成功且 `removed=true` 后失效 note cache。

### Java Spring Boot Comparison

- `DeleteComment` 类似 Spring `@Transactional` service 方法：查评论、校验作者、软删除、更新计数、写 outbox 都在一个事务里。
- GORM soft delete 类似 JPA `@SQLDelete + @Where` 或 MyBatis-Plus 逻辑删除。
- 显式返回 `(removed, noteID, error)` 类似 Java 中返回一个 result DTO，便于 handler 判断是否需要失效缓存。

### Verification

- `go test ./internal/interaction -run "TestRepositoryCreatesOutboxEventOnlyWhenCommentDeleted|TestDeleteComment" -v` passed。

### Follow-Up

- 可以继续补 `GET /api/users/{userId}/notes`、`DELETE /api/notes/{noteId}` 等 API 文档中声明但尚未实现的接口，进一步减少文档和代码偏差。

## 2026-06-01 Optimization Stage G: Public Author Notes And Note Removal

### Stage Goal

补齐 API 文档中已经声明的内容侧接口：

- `GET /api/users/{userId}/notes`: 查询作者公开内容列表。
- `DELETE /api/notes/{noteId}`: 作者软删除自己的内容。

本阶段同时修正一个可见性边界：非作者查看内容详情时必须同时满足 `published + public`，不能只判断 `published`。

### Files Modified

- `internal/note/handler.go`: 新增公开作者内容列表和删除自己内容的路由、错误处理、缓存失效注入。
- `internal/note/service.go`: 新增 `PublicNotes`、`DeleteOwn`，并修正 `Detail` 的公开可见性规则。
- `internal/note/repository.go`: 新增公开作者内容查询和 `status=removed` 条件更新。
- `internal/note/handler_test.go`: 用 TDD 增加公开列表、private detail、作者删除、非作者删除测试。
- `internal/cache/note_cache.go`: 扩展 service 装饰器接口，对新增方法透传。
- `cmd/server/main.go`: Redis 启用时把 cache invalidator 注入 note handler。
- `docs/cache-and-consistency.md`: 补充内容删除后的 Feed/Note cache 失效规则。
- `docs/15-backend-optimization-log.md`: 追加本阶段优化过程、方案和实现复盘。

### Go Backend Notes

- **路由契约闭环**：`docs/04-api-design.md` 已经声明的接口应该在代码中存在，后端项目不是只写功能，还要保证 API contract 可核对。
- **公开可见性统一**：公共详情、作者公开列表、Feed 都应以 `status=published AND visibility=public` 为准，避免 private 内容在某个读路径泄漏。
- **软删除状态流**：内容删除使用 `status=removed`，不直接物理删除。这样可以保留审计和治理历史，也符合内容社区项目的生命周期叙事。
- **幂等删除**：重复删除返回 `deleted=false`，不重复改状态，也不重复触发缓存失效。
- **缓存失效边界**：删除成功后失效 Feed 首页和 Note detail/count cache；Redis 失败不回滚业务，因为 MySQL 仍是事实来源。

### Java Spring Boot Comparison

- `note.Service.DeleteOwn` 类似 Spring `@Transactional` service 中的业务权限判断和状态流转方法，只是 Go 这里显式返回 `(deleted, error)`。
- `ErrForbidden` + handler 统一映射 HTTP 403，类似 Spring 里抛业务异常后由 `@ControllerAdvice` 转成统一错误响应。
- GORM 条件更新 `WHERE id=? AND status<>removed` 类似 MyBatis/JPA 中的乐观条件更新，用 `RowsAffected` 判断是否真的发生状态变化。
- `CacheInvalidator` 小接口类似 Java 中依赖一个 `CacheService` 接口，而不是让 note handler 直接依赖 Redis client。

### Key Code Paths

- 作者公开列表：

```text
GET /api/users/:userId/notes
  -> note.Handler.PublicNotes
  -> note.Service.PublicNotes
  -> note.Repository.FindPublicNotesByAuthorID
  -> toListDTO
```

- 作者删除内容：

```text
DELETE /api/notes/:noteId
  -> note.Handler.DeleteOwn
  -> note.Service.DeleteOwn
  -> note.Repository.MarkNoteRemoved
  -> InvalidateNoteCache + InvalidateFeedCache
```

- 公开详情可见性：

```text
GET /api/notes/:noteId
  -> author can see own non-removed content
  -> non-author can only see published + public content
  -> removed always returns note_not_found
```

### Common Pitfalls

- 不要只在列表页过滤 `visibility=public`，详情页也必须过滤，否则 private 内容可以被 ID 枚举访问。
- 不要把内容删除做成物理删除，否则 review task、interaction、outbox、后台审计都可能丢上下文。
- 不要对重复删除反复失效缓存或产事件；幂等请求没有真实状态变化。
- Gin 中 `/users/:userId/notes` 和 `/users/me/notes` 路由同层，新增动态路由后必须跑旧的 `MyNotes` 测试防回归。

### Verification

- `go test ./internal/note -run "TestNoteDetailHidesPrivatePublishedNoteFromNonAuthor|TestPublicAuthorNotesOnlyReturnsPublishedPublicNotes|TestDeleteOwnNote" -v` passed。
- `go test ./internal/note -v` passed。
- `go test ./internal/note ./internal/cache ./cmd/server` passed。
- `go test ./...` passed。

### Follow-Up

- 继续补齐 account API 文档和代码偏差：`DELETE /api/sessions/current`、`PATCH /api/users/me`、`GET /api/users/{userId}`。
- 管理后台侧可以继续补 `GET /api/admin/notes`，但不要扩成复杂后台系统。

## 2026-06-01 Optimization Stage H: Account Profile Contract

### Stage Goal

补齐 Account API 文档和代码之间的差异，让用户体系更完整：

- `DELETE /api/sessions/current`: 当前会话注销。
- `PATCH /api/users/me`: 更新当前用户资料。
- `GET /api/users/{userId}`: 查看公开资料。

### Files Modified

- `internal/account/dto.go`: 新增 `PublicUserDTO`，公开资料只包含非敏感字段。
- `internal/account/handler.go`: 新增 logout、更新资料、公开资料 handler 和路由。
- `internal/account/service.go`: 新增 `UpdateProfile`、`PublicProfile` 及资料校验，`PATCH` 未传字段保持原值。
- `internal/account/repository.go`: 新增 `UpdateProfile`。
- `internal/account/handler_test.go`: 用 TDD 覆盖资料更新、公开资料、logout 和错误场景。
- `docs/04-api-design.md`: 补充资料更新请求、公开资料响应和敏感字段限制。
- `docs/cache-and-consistency.md`: 补充公开资料缓存边界和 stateless JWT logout 说明。
- `docs/15-backend-optimization-log.md`: 追加本阶段优化过程、方案和实现复盘。
- `docs/openapi.yaml`: 同步 Account 相关 API 契约。

### Go Backend Notes

- **DTO 分层**：`UserDTO` 是当前登录用户视图，可以包含 email、role、status；`PublicUserDTO` 是匿名公开视图，不能包含这些字段。
- **资料更新校验**：`PATCH /api/users/me` 支持部分更新，未传字段保持原值；显式传 nickname 时不能为空且不超过 64 个字符；avatar_url 不超过 512；bio 不超过 512。校验放在 service 层，handler 只负责 JSON 和认证。
- **stateless JWT logout**：当前 logout 不做服务端 token 黑名单，只返回 `logged_out=true`，由前端删除本地 token。这个实现简单、清晰，但不能让已签发 token 立即失效。
- **Repository 返回最终状态**：更新资料后重新查一次用户，确保响应来自数据库最终状态，而不是只拼接请求体。

### Java Spring Boot Comparison

- `PublicUserDTO` 类似 Java 项目中给 Controller 返回的 VO，和数据库 Entity、当前用户 DTO 分离。
- `UpdateProfile` 类似 Spring service 中的业务校验方法，Controller 不直接操作 Repository。
- stateless JWT logout 类似 Spring Security 只清理客户端 token 的方案；如果要服务端强制失效，需要 Redis token blacklist 或 session store。
- `UpdateProfile` 的 `RowsAffected == 0` 判断类似 MyBatis update 返回影响行数后决定是否抛 `NotFoundException`。

### Key Code Paths

- 更新资料：

```text
PATCH /api/users/me
  -> account.Handler.UpdateProfile
  -> account.Service.UpdateProfile
  -> account.Repository.UpdateProfile
  -> account.ToUserDTO
```

- 公开资料：

```text
GET /api/users/:userId
  -> account.Handler.PublicProfile
  -> account.Service.PublicProfile
  -> account.ToPublicUserDTO
```

- 当前会话注销：

```text
DELETE /api/sessions/current
  -> AuthRequired middleware
  -> account.Handler.LogoutCurrentSession
  -> { "logged_out": true }
```

### Common Pitfalls

- 不要把 `UserDTO` 直接用于公开资料接口，否则会泄漏 email、role、status。
- 不要把 logout 讲成“服务端已吊销 token”；当前 stateless JWT 只能让客户端丢弃 token。
- 不要在 handler 里写大量业务校验，校验应在 service 层集中，便于测试和复用。
- 不要忘记动态路由 `/users/:userId` 可能影响 `/users/me`，新增后必须跑 account 全包测试。

### Verification

- `go test ./internal/account -run "TestAccountRoutes(UpdateCurrentUserProfile|RejectInvalidProfileUpdate|GetPublicProfile|PublicProfileNotFound|LogoutCurrentSession)" -v` passed。
- `go test ./internal/account -run "TestAccountRoutes(UpdateCurrentUserProfile|PatchProfileKeepsOmittedFields|RejectInvalidProfileUpdate|GetPublicProfile|PublicProfileNotFound|LogoutCurrentSession)" -v` passed。
- `go test ./internal/account ./internal/http ./cmd/server -v` passed。
- `go test ./...` passed。

### Follow-Up

- 管理后台继续补 `GET /api/admin/notes`，完善内容治理闭环。
- 后续如接入 Redis profile cache，应在 `PATCH /api/users/me` 成功后删除 `user:profile:{id}`。

## 2026-06-01 Optimization Stage I: Admin Note Search

### Stage Goal

补齐 API 文档中已经声明的 `GET /api/admin/notes`，让管理后台能按状态和关键词检索内容，完善内容治理闭环。

### Files Modified

- `internal/review/dto.go`: 新增 `AdminNoteDTO`、`AdminNoteListDTO`。
- `internal/review/handler.go`: 新增 `GET /api/admin/notes` 路由和 handler。
- `internal/review/service.go`: 新增 `SearchNotes`，校验状态过滤条件并组装后台 DTO。
- `internal/review/repository.go`: 新增 `SearchNotes` GORM 查询。
- `internal/review/handler_test.go`: 用 TDD 覆盖管理员检索、非管理员拒绝、非法状态拒绝。
- `docs/04-api-design.md`: 补充后台内容检索查询参数和状态枚举。
- `docs/15-backend-optimization-log.md`: 追加 Stage I 复盘。
- `docs/openapi.yaml`: 同步后台内容检索 API 契约。

### Go Backend Notes

- **后台接口边界**：`GET /api/admin/notes` 只解决治理后台的低频检索，不扩成搜索平台。
- **状态白名单**：service 层校验 `pending_review/published/rejected/removed`，非法状态返回 `400 invalid_status`，避免错误查询静默返回空列表。
- **轻量关键词查询**：当前用 `LOWER(title) LIKE ? OR LOWER(body) LIKE ?`，适合后台低频查询；如果后续做大规模搜索，再考虑 ES 或专门搜索服务。
- **后台 DTO**：返回状态、可见性、review_task_id、计数和时间字段，方便管理员理解内容生命周期。

### Java Spring Boot Comparison

- `SearchNotes` 类似 Spring Data JPA Specification 或 MyBatis 动态 SQL，根据可选条件拼接查询。
- status 白名单类似 Java enum 参数校验，避免 controller 直接把任意字符串传入 DAO。
- Admin DTO 类似后台管理 VO，和前台 NoteDTO 区分，避免前后台响应结构互相牵制。

### Key Code Paths

```text
GET /api/admin/notes?status=published&q=go
  -> AuthRequired
  -> review.Handler.requireAdmin
  -> review.Handler.SearchNotes
  -> review.Service.SearchNotes
  -> review.Repository.SearchNotes
  -> review.AdminNoteListDTO
```

### Common Pitfalls

- 不要把后台内容检索讲成“搜索系统”；当前只是 MySQL 条件查询。
- 不要省略 admin 权限校验；后台接口必须走 `requireAdmin`。
- 不要对非法 status 静默返回空列表，面试里这属于 API 契约不清。
- 不要直接复用前台 NoteDTO，后台需要 review_task_id 和治理状态字段。

### Verification

- `go test ./internal/review -run "TestAdminNotes" -v` passed。
- `go test ./internal/review -v` passed。
- `go test ./...` passed。

### Follow-Up

- 管理决策接口文档还写了 `remove/request manual review`，当前代码只支持 approve/reject。后续可选择实现 `remove`，但不要扩成复杂工作流。
