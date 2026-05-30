# 知涟 Commit Plan

Use one commit per development stage. The development roadmap is stage-based, but commit messages should describe the completed feature or technical capability directly. Do not write commit messages such as `feat: stage1` or `feat: implement stage 2`.

## Commit Message Rules

Use conventional prefixes:

```text
docs:
feat:
fix:
refactor:
test:
chore:
build:
perf:
```

Good commit messages:

```text
feat: add backend skeleton and health endpoint
feat: add account registration and login
feat: add content publishing and upload workflow
feat: add review task state transitions
feat: expose internal content analysis APIs
feat: add cursor-based feed endpoints
feat: add content interactions and follow graph
feat: add Redis caching for feed and content reads
feat: publish domain events through outbox
feat: add React frontend for content community demo
build: add Docker Compose demo stack
perf: benchmark and optimize feed reads
```

Bad commit messages:

```text
feat: stage1
feat: stage 2 account
feat: finish roadmap stage
update files
wip
```

## One-Commit-Per-Stage Plan

### Backend Foundation

Commit after the backend skeleton, API contract, config loading, logging, unified response, request ID middleware, optional MySQL wiring, `/health`, tests, and learning log are complete.

Suggested message:

```text
feat: add backend skeleton and health endpoint
```

### Account And Authentication

Commit after user model/repository, registration, login, JWT issuing, auth middleware, current-user API, tests, and learning log are complete.

Suggested message:

```text
feat: add account registration and login
```

### Content Publishing And Uploads

Commit after content/media/tag models, local image upload, content publishing, content detail, author/my-content listing, tests, and learning log are complete.

Suggested message:

```text
feat: add content publishing and upload workflow
```

### Review And Governance Workflow

Commit after review task models, audit events, publish-time review task creation, admin review APIs, content status transitions, tests, and learning log are complete.

Suggested message:

```text
feat: add review task state transitions
```

### 知涟洞察 Integration

Commit after internal token middleware, pending task API, review context API, agent result callback API, idempotency tests, and learning log are complete.

Suggested message:

```text
feat: expose internal content analysis APIs
```

### Feed

Commit after cursor encoding, latest feed, hot feed, following feed, tag feed, hydration, pagination tests, and learning log are complete.

Suggested message:

```text
feat: add cursor-based feed endpoints
```

### Interactions And Social Graph

Commit after like, favorite, comment, follow APIs, idempotency handling, tests, and learning log are complete.

Suggested message:

```text
feat: add content interactions and follow graph
```

### Redis Cache

Commit after Redis client setup, feed first-page cache, content detail/profile/count cache, invalidation behavior, tests, and learning log are complete.

Suggested message:

```text
feat: add Redis caching for feed and content reads
```

### RabbitMQ And Outbox

Commit after outbox model, event publisher, worker delivery path, RabbitMQ wiring, retry behavior, tests, and learning log are complete.

Suggested message:

```text
feat: publish domain events through outbox
```

### Frontend

Commit after React app shell, API client, auth pages, feed/detail pages, publishing page, admin review page, build verification, and learning log are complete.

Suggested message:

```text
feat: add React frontend for content community demo
```

### Deployment And Demo Data

Commit after Dockerfiles, Docker Compose, seed command, README quick start, screenshots, and learning log are complete.

Suggested message:

```text
build: add Docker Compose demo stack
```

### Benchmark And Resume Polish

Commit after benchmark scripts, measured results, README/resume notes, and learning log are complete.

Suggested message:

```text
perf: benchmark and optimize feed reads
```

## Practical Guidance

- Commit after the whole stage is implemented and verified.
- Do not commit broken compilation unless explicitly creating a temporary checkpoint branch.
- Do not mention stage numbers in commit messages.
- Prefer messages that tell a reviewer what product or architecture capability was added.
- Keep generated binaries and local notes out of git.
- `AGENTS.md` and `MEMORY.md` are intentionally local and ignored.

