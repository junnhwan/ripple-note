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
