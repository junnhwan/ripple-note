# 知涟 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a deployable content community and Feed distribution platform with account, content publishing, feed, interactions, review/governance workflow, React frontend, and Docker Compose demo.

**Architecture:** The backend uses Go + Gin + GORM with MySQL for durable state, Redis for cache/counters, and RabbitMQ for async events. The frontend uses React + Vite + TypeScript and calls the Gin API. `ripple-guard-agent` (`知涟洞察`) integrates through internal review/governance APIs.

**Tech Stack:** Go, Gin, GORM, MySQL, Redis, RabbitMQ, React, Vite, TypeScript, TailwindCSS, Docker Compose.

---

## File Structure To Create

```text
cmd/server/main.go
cmd/worker/main.go
cmd/seed/main.go
configs/config.local.yaml
configs/config.docker.yaml
internal/config/
internal/storage/
internal/cache/
internal/queue/
internal/http/
internal/middleware/
internal/auth/
internal/account/
internal/note/
internal/feed/
internal/interaction/
internal/social/
internal/review/
internal/admin/
internal/upload/
internal/event/
internal/notification/
internal/observability/
web/
docker-compose.yml
Dockerfile
README.md
```

## Task 1: Backend Skeleton

**Files:**

- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `internal/config/config.go`
- Create: `internal/http/router.go`
- Create: `internal/observability/logger.go`
- Create: `configs/config.local.yaml`

- [ ] Step 1: Initialize Go module.

Run:

```bash
go mod init ripple-note
```

- [ ] Step 2: Add Gin, GORM, MySQL driver, Redis, and RabbitMQ dependencies.

Run:

```bash
go get github.com/gin-gonic/gin gorm.io/gorm gorm.io/driver/mysql github.com/redis/go-redis/v9 github.com/rabbitmq/amqp091-go
```

- [ ] Step 3: Implement config loading and `/health`.

Expected:

```bash
go run ./cmd/server -config configs/config.local.yaml
curl http://127.0.0.1:8080/health
```

returns:

```json
{"status":"ok"}
```

- [ ] Step 4: Add a basic server test for health route.

Run:

```bash
go test ./internal/http -run TestHealth -v
```

## Task 2: Database Models And Migration

**Files:**

- Create: `internal/account/model.go`
- Create: `internal/note/model.go`
- Create: `internal/interaction/model.go`
- Create: `internal/social/model.go`
- Create: `internal/review/model.go`
- Create: `internal/event/model.go`
- Create: `internal/storage/mysql.go`

- [ ] Step 1: Define GORM models from `docs/03-data-model.md`.
- [ ] Step 2: Add `AutoMigrate` in development mode.
- [ ] Step 3: Add repository tests for creating a user, note, and review task.

Run:

```bash
go test ./internal/... -run TestRepository -v
```

## Task 3: Account And Auth

**Files:**

- Create: `internal/auth/password.go`
- Create: `internal/auth/jwt.go`
- Create: `internal/account/service.go`
- Create: `internal/account/handler.go`
- Modify: `internal/http/router.go`

- [ ] Step 1: Write tests for register, login, and current user.
- [ ] Step 2: Implement password hashing and JWT.
- [ ] Step 3: Add account routes.
- [ ] Step 4: Verify with HTTP requests.

Run:

```bash
go test ./internal/auth ./internal/account -v
```

## Task 4: Notes And Uploads

**Files:**

- Create: `internal/upload/service.go`
- Create: `internal/upload/handler.go`
- Create: `internal/note/service.go`
- Create: `internal/note/handler.go`
- Modify: `internal/http/router.go`

- [ ] Step 1: Add image upload endpoint using local disk storage.
- [ ] Step 2: Add note creation endpoint.
- [ ] Step 3: Creating a note must create a review task in the same transaction.
- [ ] Step 4: Verify note status is `pending_review`.

Run:

```bash
go test ./internal/upload ./internal/note ./internal/review -v
```

## Task 5: Review Workflow And Internal APIs

**Files:**

- Create: `internal/review/service.go`
- Create: `internal/review/handler_internal.go`
- Create: `internal/admin/review_handler.go`
- Modify: `internal/http/router.go`

- [ ] Step 1: Implement pending task list for Agent.
- [ ] Step 2: Implement review context endpoint.
- [ ] Step 3: Implement Agent result callback.
- [ ] Step 4: Implement admin review decision.
- [ ] Step 5: Test pass, reject, manual review, and admin override transitions.

Run:

```bash
go test ./internal/review ./internal/admin -v
```

## Task 6: Feed

**Files:**

- Create: `internal/feed/cursor.go`
- Create: `internal/feed/service.go`
- Create: `internal/feed/handler.go`
- Modify: `internal/http/router.go`

- [ ] Step 1: Write cursor encode/decode tests.
- [ ] Step 2: Implement latest feed.
- [ ] Step 3: Implement hot feed baseline.
- [ ] Step 4: Implement following feed.
- [ ] Step 5: Add Redis first-page cache for anonymous latest feed.

Run:

```bash
go test ./internal/feed -v
```

## Task 7: Interactions And Social

**Files:**

- Create: `internal/interaction/service.go`
- Create: `internal/interaction/handler.go`
- Create: `internal/social/service.go`
- Create: `internal/social/handler.go`

- [ ] Step 1: Implement like and unlike with idempotency.
- [ ] Step 2: Implement favorite and unfavorite.
- [ ] Step 3: Implement comments.
- [ ] Step 4: Implement follow and unfollow.
- [ ] Step 5: Update counters transactionally in V1.

Run:

```bash
go test ./internal/interaction ./internal/social -v
```

## Task 8: Worker And Events

**Files:**

- Create: `cmd/worker/main.go`
- Create: `internal/event/outbox.go`
- Create: `internal/event/publisher.go`
- Create: `internal/notification/service.go`

- [ ] Step 1: Implement outbox event model and publisher.
- [ ] Step 2: Publish review and interaction events.
- [ ] Step 3: Add worker process that can consume placeholder queues.
- [ ] Step 4: Keep V1 business correctness independent of RabbitMQ availability where possible.

Run:

```bash
go test ./internal/event ./internal/notification -v
```

## Task 9: Frontend

**Files:**

- Create: `web/package.json`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`
- Create: `web/src/api/client.ts`
- Create: `web/src/pages/FeedPage.tsx`
- Create: `web/src/pages/LoginPage.tsx`
- Create: `web/src/pages/RegisterPage.tsx`
- Create: `web/src/pages/PublishPage.tsx`
- Create: `web/src/pages/NoteDetailPage.tsx`
- Create: `web/src/pages/ProfilePage.tsx`
- Create: `web/src/pages/AdminReviewPage.tsx`

- [ ] Step 1: Scaffold React + Vite.
- [ ] Step 2: Add API client and auth token storage.
- [ ] Step 3: Build feed, auth, publish, detail, profile, and admin review pages.
- [ ] Step 4: Verify full demo flow in browser.

Run:

```bash
cd web
npm install
npm run build
```

## Task 10: Docker Compose And Demo

**Files:**

- Create: `docker-compose.yml`
- Create: `Dockerfile`
- Create: `web/Dockerfile`
- Create: `README.md`

- [ ] Step 1: Add Dockerfile for Go API and worker.
- [ ] Step 2: Add Dockerfile for React static build.
- [ ] Step 3: Add MySQL, Redis, RabbitMQ, API, worker, and web services.
- [ ] Step 4: Add README quick start and demo account notes.

Run:

```bash
docker compose up -d --build
curl http://127.0.0.1:8080/health
```

## Completion Criteria

- `go test ./...` passes.
- `npm run build` passes under `web/`.
- `docker compose up -d --build` starts the full stack.
- Browser demo can publish a note, process review, and display approved content in feed.
- Internal APIs are ready for `ripple-guard-agent` (`知涟洞察`).
