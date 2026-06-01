# 知涟 Project Optimization Plan

## Goal

把知涟从“功能能跑的 Go 项目”优化成更适合作为 Go 后端实习简历项目的内容社区与 Feed 分发平台。

优化重点不是堆技术，而是强化能在面试中讲清楚的后端能力：

- 内容状态流与治理工作流。
- 多场景 Feed 与复合游标分页。
- Redis 缓存、限流与一致性边界。
- RabbitMQ + Outbox 最终一致性。
- 可部署、可压测、可演示、可写进简历。

> 2026-06-01 范围调整：AI/审查 Agent 相关能力先作为未来扩展保留，不作为当前优化重点。后续阶段优先完善 Go 后端主线：事务、一致性、缓存、限流、Outbox、Feed 和压测证据。

## Current Evidence

当前项目已经具备以下基础：

| 能力 | 当前状态 | 证据 |
| --- | --- | --- |
| 账户与 JWT | 已实现 | `internal/account`、`internal/auth` |
| 内容发布与审核流 | 已实现 | `internal/note`、`internal/review` |
| 内部内容分析 API | 已有雏形，当前暂缓扩展 | `internal/review/internal_handler.go` |
| 多场景 Feed | 已实现 | `internal/feed` |
| 互动与关注 | 已实现 | `internal/interaction` |
| Redis Feed 首页缓存 | 已实现 | `internal/cache/feed_cache.go` |
| RabbitMQ + Outbox | 已实现基础能力 | `internal/outbox`、`cmd/worker` |
| React demo | 已实现 | `web/` |
| Docker 部署 | 已实现基础部署 | `Dockerfile`、`docker-compose.deploy.yml` |

新增文档已经补齐：

- `docs/13-backend-design-borrowing.md`: 外部项目设计借鉴与取舍。
- `docs/cache-and-consistency.md`: Redis key、TTL、失效和降级策略。
- `docs/events-and-outbox.md`: Outbox 事件目录、重试和幂等规则。

## External Design Borrowing

### Borrow Now

| 来源 | 借鉴点 | 在知涟中的落地 |
| --- | --- | --- |
| `zhiguang_be` | 统一响应、错误码、认证保护、Redis 频控意识 | 统一错误码文档，新增轻量 Redis 固定窗口限流 |
| `zhiguang_be` | 缓存一致性、热点保护、single-flight 设计 | 当前先做短 TTL + 主动失效，P1 再补 single-flight |
| `xiaohashu` 类社区项目 | 内容、互动、Feed 的领域边界 | 保持内容社区主线，不引入复杂推荐系统 |
| `GCFeed` / `feedsystem_video_go` | 复合游标、Feed hydration、压测叙事 | 保持 `published_at + id`、`hot_score + id` 游标，补压测结果 |
| `DokiTV` | Docker Compose、中间件健康检查、异步基础设施 | 只借部署和中间件组织，不引入视频域能力 |

### Do Not Borrow Now

| 不建议 | 原因 |
| --- | --- |
| Canal + Kafka 全量同步 | 对当前项目过重，会稀释 RabbitMQ + Outbox 主线 |
| 微服务拆分 | 实习简历项目更需要单体模块边界清晰、功能完整 |
| 视频播放、转码、弹幕 | 偏离内容社区与 Feed 分发定位 |
| 复杂机器学习推荐 | 当前 Feed 需要可解释排序和性能优化，不需要推荐平台化 |
| Redis SDS/Bitmap 计数全量落地 | 可作为面试延展，不适合作为当前 P0 实现 |

## Optimization Stages

### Stage A: API Error Codes And Lightweight Protection

目标：补齐统一错误码语义，并给高风险写接口加 Redis 固定窗口限流。

已实施：

- 新增 `internal/ratelimit`。
- 支持 `Store` 抽象、内存测试 store、Redis store。
- 对规则按 `method + route full path` 匹配。
- Redis 出错时降级放行，避免业务正确性依赖 Redis。
- 在 Redis 启用时对以下接口启用限流：
  - `POST /api/users`
  - `POST /api/sessions`
  - `POST /api/notes`
  - `PUT /api/notes/:noteId/like`
  - `PUT /api/notes/:noteId/favorite`
  - `POST /api/notes/:noteId/comments`
  - `PUT /api/users/me/following/:targetUserId`

验收：

```bash
go test ./internal/ratelimit ./internal/http ./internal/cache
go test ./...
```

### Stage B: Error Code Contract Tightening

目标：把 API 错误码从“散落字符串”收敛成可解释的错误码表。

计划：

- 在 `docs/04-api-design.md` 增加错误码表。
- 逐步把代码中的错误码整理为：
  - `invalid_argument`
  - `unauthorized`
  - `forbidden`
  - `not_found`
  - `state_conflict`
  - `rate_limited`
  - `internal_error`
- 保留业务细分错误码，例如 `note_not_found`、`invalid_credentials`，但文档要说明其上位类别。

验收：

```bash
rg '"[a-z_]+".*Error|httpapi.Error' internal
go test ./...
```

### Stage C: Cache Read Path Tightening

目标：把已约定的 `note:detail:{id}`、`note:counts:{id}` 逐步接入读路径。

已实施：

- 新增 `internal/cache/note_cache.go`，以装饰器模式包住 note service。
- 匿名公开详情接入 `note:detail:{id}` cache-aside。
- cache miss 回源后写入 `note:counts:{id}` 计数快照。
- 保持登录态 viewer state 不进入共享缓存。
- Redis 失败时回退 MySQL。
- `internal/note/handler.go` 改为依赖 `ServiceAPI` 接口，便于注入缓存装饰器。

验收：

```bash
go test ./internal/cache ./internal/note ./cmd/server
go test ./...
```

### Stage D: Outbox Contract Tightening

目标：让 RabbitMQ + Outbox 不只是“有 worker”，而是有完整事件合约。

已实施：

- 管理员审核决策事务内写入 `note.review_decided`。
- 点赞、收藏、评论、关注等互动创建成功时写入 `interaction.created`。
- 取消点赞、取消收藏、取消关注成功时写入 `interaction.removed`。
- 重复点赞、重复收藏、重复关注、重复取消等幂等请求不重复写事件。
- Outbox worker 增加 `DefaultMaxRetries = 5` 和 `abandoned` 终态，超过重试次数后停止自动投递。
- API server 将 review、note、interaction 写路径接入同一个 `outbox.Helper`，保证业务表和事件表在同一个 MySQL 事务内提交。

验收：

```bash
go test ./internal/interaction ./internal/outbox ./internal/review ./cmd/server -v
go test ./...
```

### Stage E: Benchmark And Resume Evidence

目标：给简历中的性能描述提供真实数据，不编数字。

计划：

- 使用 `scripts/loadtest` 跑匿名 latest/hot Feed。
- 对比 Redis enabled/disabled 两组数据。
- 记录 P95/P99、吞吐和错误率。
- 更新 `docs/12-load-test.md` 和 README。

验收：

- 压测命令、环境、数据量、结果可复现。
- 简历只写真实数据。

## Implementation Rules

- 每个阶段先补测试，再写实现。
- 每个阶段结束前运行 `gofmt` 和 `go test ./...`。
- 涉及前端时运行 `npm run build`。
- 只有文档和简历可以直接编辑；业务行为变更必须配测试。
- 不把 DokiTV、AI 项目、搜索项目的非主线功能搬进知涟。

## Resume Mapping

| 简历亮点 | 对应实现/文档 |
| --- | --- |
| Handler -> Service -> Repository 分层 | `internal/account`、`internal/note`、`internal/feed` |
| 内容治理状态机 | `internal/review`、`docs/06-review-workflow.md` |
| 多场景 Feed + 复合游标 | `internal/feed`、`docs/05-feed-design.md` |
| Redis Cache Aside + 主动失效 | `internal/cache`、`docs/cache-and-consistency.md` |
| Redis 固定窗口限流 | `internal/ratelimit`、本计划 Stage A |
| RabbitMQ + Outbox | `internal/outbox`、`docs/events-and-outbox.md` |
| 后续内容分析扩展 | `internal/review/internal_handler.go`，当前暂缓展开 |
