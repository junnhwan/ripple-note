# 知涟 Ripple Note

一个内容社区与 Feed 分发平台，涵盖内容发布、Feed 分发、互动、审核治理、缓存、异步事件和前端演示闭环。

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go · Gin · GORM |
| 数据库 | MySQL · Redis |
| 消息队列 | RabbitMQ |
| 前端 | React · TypeScript · Vite · Tailwind CSS |
| 部署 | Docker Compose |

## 核心设计与实现

### Feed 流 — Cursor 分页 + 多维度排序

- 基于 `(published_at, id)` 和 `(hot_score, id)` 的**复合 cursor 分页**，避免 OFFSET 深翻页性能退化；cursor 编码为 Base64 JSON，客户端无需理解内部结构。
- 四种 Feed 视图：最新、热门（按 hot_score 降序）、关注（按关注列表过滤 author_id）、按标签（JOIN note_tags）。
- Feed 列表回填采用批量查询：作者、标签、图片、点赞状态、收藏状态、关注状态按 ID 集合一次拉取，避免每页 N+1 查询。
- **Trade-off**：热门排序使用数据库内 hot_score 字段而非 Redis Sorted Set，简化架构；适合中小数据量场景，若数据量增长可切换为 ZSET + 缓存。

### 缓存策略 — 分层 + 选择性缓存

- Redis 缓存层位于 Service 之上（装饰器模式）：`FeedCache` 包裹 `FeedService`，无需修改业务代码。
- **选择性缓存**：仅缓存匿名用户首页（无 cursor）的最新/热门 Feed（TTL 30s）；关注 Feed 和标签 Feed 因用户维度/组合过多，穿透到数据库。
- 笔记详情和计数分别缓存（TTL 2min / 1min），审核决策后主动失效 Feed + 详情缓存。

### 点赞/收藏 — 软删除恢复代替硬删

- 使用 GORM `Unscoped()` 查询含软删除记录；取消点赞后标记 `deleted_at`，重新点赞时恢复（`UPDATE deleted_at = NULL`），避免反复 INSERT/DELETE 导致的主键碎片。
- 计数更新在事务内通过 `gorm.Expr("likes_count + 1")` 原子递增，保证一致性。

### 领域事件 — Transactional Outbox

- 业务操作（发布内容、审核决策、互动状态变更）在同一数据库事务内写入 `outbox_events` 表，保证**本地事务与事件写入的原子性**。
- 独立 Worker 进程轮询 pending/failed 事件（5s 间隔），批量投递到 RabbitMQ；投递失败标记 `failed` + 30s 后重试，超过重试上限进入 `abandoned`，避免消息丢失。
- **Trade-off**：相比直接发 MQ，Outbox 模式增加了一次 DB 写入和轮询开销，但消除了"事务提交但消息未发"的不一致风险。

### 接口保护 — Redis 固定窗口限流

- 基于 Redis `INCR + EXPIRE` 实现固定窗口限流，保护注册、登录、发布、点赞、收藏、评论和关注等高风险写接口。
- 限流按 route full path 匹配，用户态 key 直接解析 Bearer JWT；Redis 异常时降级放行，避免核心业务正确性依赖 Redis。

### 审核流程 — 平台状态机 + 后台决策

- 内容发布时在事务内创建 `review_task`，内容先进入 `pending_review`，不会进入公共 Feed。
- 管理员审核时在同一事务内更新 task 状态 + 内容状态 + 写入审计事件（`review_task_events`）和 `note.review_decided` outbox event，事务提交后失效缓存。
- 审核决策使用状态检查（若已决策则返回 `ErrAlreadyDecided`），防止重复操作。

### 认证鉴权 — JWT + 中间件分层

- JWT（HS256）签发携带 `user_id` + `role`，支持 Bearer Token 鉴权。
- 中间件分为 `AuthRequired`（强制鉴权）和 `OptionalAuth`（可选鉴权，Feed 首页需同时服务匿名和登录用户），登录用户的 Feed 额外注入 `viewer_liked` / `viewer_favorited` / `viewer_following` 状态。

## 演示截图

| 首页 Feed 流 | 笔记详情 |
|:---:|:---:|
| ![首页](docs/screenshots/feed.png) | ![详情](docs/screenshots/detail.png) |

| 发布笔记 | 登录 |
|:---:|:---:|
| ![发布](docs/screenshots/publish.png) | ![登录](docs/screenshots/login.png) |

| 个人中心 | 内容审核 |
|:---:|:---:|
| ![个人中心](docs/screenshots/profile.png) | ![审核](docs/screenshots/review.png) |

## 压测结果

压测报告见 [docs/12-load-test.md](docs/12-load-test.md)。当前已记录 2026-05-31 在 4 vCPU / 3.8 GiB 云服务器、Docker Compose 单机部署上的真实结果：

| 场景 | 数据集 | VUs / 时长 | RPS | P95 | P99 | 错误率 |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| 匿名 latest Feed | 5 万笔记、60 万互动 | 100 / 2m | 2683.62 | 81.25ms | 111.61ms | 0.00% |
| 登录态 latest Feed | 5 万笔记、60 万互动 | 50 / 2m | 676.49 | 106.90ms | 126.44ms | 0.00% |
| 混合 Feed | 5 万笔记、60 万互动 | 100 / 2m | 1293.32 | 232.03ms | 291.44ms | 0.00% |

登录态 latest Feed 在批量回填优化后，单页 SQL 查询数由约 121 次降至约 7 次，RPS 由 107.58 提升至 676.49，P95 由 614.91ms 降至 106.90ms。该结果用于项目复盘和简历量化，不宣称为生产 SLA。

## 快速启动

```bash
# 启动后端
go run ./cmd/server

# 启动前端
cd web && npm install && npm run dev
```

需要 MySQL、Redis、RabbitMQ。配置文件在 `configs/`。

## 项目结构

```
├── cmd/
│   ├── server/      # HTTP API 服务
│   ├── worker/      # 异步事件消费
│   └── seed/        # 数据填充
├── internal/        # 业务逻辑
│   ├── account/     # 用户账号
│   ├── auth/        # 认证鉴权 (JWT)
│   ├── note/        # 笔记 CRUD
│   ├── feed/        # Feed 流聚合
│   ├── interaction/ # 点赞/收藏/评论/关注
│   ├── review/      # 审核流程
│   ├── outbox/      # 领域事件发件箱
│   ├── cache/       # Redis 缓存层
│   ├── storage/     # 图片存储
│   └── ...
├── web/             # React 前端
├── configs/         # 配置文件
└── docs/            # 设计文档
```

## License

MIT
