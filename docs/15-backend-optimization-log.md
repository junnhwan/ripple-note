# 知涟 Backend Optimization Log

## Scope

本轮优化聚焦 Go 后端能力，不继续扩展 AI/审查 Agent。`知涟洞察` 和内部内容分析 API 保留为后续扩展点，当前简历和复习重点放在：

- 统一错误响应和 Redis 固定窗口限流。
- 匿名内容详情 cache-aside。
- RabbitMQ + Transactional Outbox 事件合约。
- 互动、审核决策和 Outbox worker 的可靠性边界。

## Why This Direction

参考 `zhiguang_be`、`GCFeed`、`feedsystem_video_go` 和 DokiTV 后，当前最值得借鉴的是后端工程化设计，而不是继续堆业务域：

| 借鉴来源 | 落地到知涟 | 取舍 |
| --- | --- | --- |
| Java 项目的错误码、Redis 频控 | `internal/ratelimit` + API 错误码表 | 不引入复杂风控或网关 |
| Feed 项目的游标分页和缓存意识 | 保留复合游标，补缓存一致性文档 | 不做复杂推荐系统 |
| MQ 项目的可靠投递思路 | Outbox 事件表、worker、retry、abandoned | 不拆微服务、不上 Kafka/Canal |
| 视频项目的部署和中间件组织 | 保留 Docker Compose 和健康检查思路 | 不引入视频转码/弹幕等偏题能力 |

## Implemented Stages

### Stage A: Error Codes And Lightweight Rate Limiting

目标：保护高风险写接口，并让错误响应更适合作为 API 契约讲解。

主要实现：

- 新增 `internal/ratelimit`。
- 使用 Gin middleware 按 `method + route full path` 匹配规则。
- 使用 Redis `INCR + EXPIRE` 实现固定窗口限流。
- Redis 故障时降级放行，避免业务正确性依赖 Redis。
- 覆盖注册、登录、发布、点赞、收藏、评论、关注接口。

关键测试：

- `internal/ratelimit/limiter_test.go`
- `internal/http/router_test.go`

复习重点：

- 为什么限流中间件放在全局 middleware 后仍要自己解析 Authorization。
- 为什么 Redis 限流失败选择放行而不是阻断。
- 为什么 `ratelimit` 包不依赖 `internal/http`，避免 Go import cycle。

### Stage C: Note Detail Cache-Aside

目标：让文档中的 `note:detail:{id}` 和 `note:counts:{id}` 进入真实读路径。

主要实现：

- 新增 `internal/cache/note_cache.go`。
- 匿名公开详情优先读 `note:detail:{id}`。
- cache miss 后回源 MySQL，再写入 detail 和 counts snapshot。
- 登录态详情绕过共享缓存，避免未来 viewer state 串用户。
- Redis 读写失败时回退原始 note service。

关键测试：

- `internal/cache/note_cache_test.go`

复习重点：

- Cache Aside 和 write-through/write-behind 的区别。
- 为什么共享缓存不能保存 `viewer_liked` 等用户态字段。
- 为什么 handler 依赖 `ServiceAPI` 接口后更容易接入装饰器。

### Stage D: Outbox Contract Tightening

目标：让 Outbox 从“有 worker”变成“核心写链路有事件合约和失败终态”。

主要实现：

- `review.Service.Decide` 在管理员审核决策事务内写入 `note.review_decided`。
- `interaction.Repository` 在真实状态变更时写入：
  - `interaction.created`: like、favorite、comment、follow。
  - `interaction.removed`: unlike、unfavorite、unfollow。
- 幂等写请求不重复产事件：
  - 重复点赞不重复写 `interaction.created`。
  - 重复取消不重复写 `interaction.removed`。
  - 重复关注不重复写 follow 事件。
- Outbox worker 增加：
  - `DefaultMaxRetries = 5`。
  - `StatusAbandoned = "abandoned"`。
  - 连续失败超过上限后清空 `next_retry_at`，停止自动重试。
- `cmd/server` 将 review、note、interaction 写路径接入同一个 `outbox.Helper`。

关键测试：

- `internal/interaction/repository_test.go`
  - 验证互动状态变化才产事件。
  - 验证重复互动不重复产事件。
  - 验证 favorite/comment/follow 的 payload 和 aggregate。
- `internal/review/service_test.go`
  - 验证 admin decision 产生 `note.review_decided`。
- `internal/outbox/publisher_test.go`
  - 验证失败未超过上限时仍为 `failed`。
  - 验证超过上限后变为 `abandoned` 且不再被 `FindPending` 取出。

复习重点：

- Transactional Outbox 解决的是“数据库提交成功但 MQ 发布失败”的一致性问题。
- Outbox 事件必须和业务写入在同一个 MySQL 事务内提交。
- Worker 的投递语义是 at-least-once，consumer 必须幂等。
- `abandoned` 不是失败吞掉，而是停止自动重试，把问题交给监控和人工 replay。

### Stage E: Benchmark And Resume Evidence

目标：把 Feed 优化从“设计亮点”变成可核对的真实压测证据，并让 README 和简历只引用已有结果，不编造未执行数据。

主要实现：

- 补充 `scripts/loadtest/feed_hot_anonymous.js`，让 latest/hot 匿名场景都能独立运行。
- 新增 `scripts/loadtest/run-k6.ps1`，统一封装 Docker 版 k6 参数、场景选择和结果输出路径。
- 更新 `docs/12-load-test.md`：
  - 补充一键运行入口。
  - 补充 hot 匿名 Feed 手动命令。
  - 明确已有结果来自 2026-05-31 云服务器压测。
  - 明确 2026-06-01 本地 Docker daemon 未启动，因此没有新增本机压测数据。
- 更新 README 和 `resume.md`，只引用已有真实数据：
  - 匿名 latest Feed: 2683.62 RPS, P95 81.25ms。
  - 登录态 latest Feed: 676.49 RPS, P95 106.90ms。
  - 混合 Feed: 1293.32 RPS, P95 232.03ms。
  - 登录态 latest Feed 优化前后：RPS 107.58 -> 676.49，P95 614.91ms -> 106.90ms。

关键验证：

- `go test ./...` 验证后端代码仍可通过。
- 尝试使用 Docker 版 k6 校验脚本运行，失败原因是本机 Docker daemon 未启动，不是 k6 脚本业务断言失败。

复习重点：

- 简历性能数字必须能追溯到命令、环境、数据集和报告文件。
- 压测结果要注明部署环境和限制，不能宣称为生产 SLA。
- 没跑过的场景可以补脚本和模板，但不能补数字。

### Stage F: Complete Comment Deletion Interaction Flow

目标：补齐 API 文档中已经声明的 `DELETE /api/comments/{commentId}`，让评论互动闭环完整，并让 `interaction.removed` 覆盖删除评论场景。

主要实现：

- `interaction.Repository.DeleteComment`:
  - 只允许评论作者删除自己的评论。
  - 使用 GORM soft delete 删除评论。
  - 在同一事务内减少 `notes.comments_count`。
  - 在同一事务内写入 `interaction.removed`，`aggregate_type = comment`，`action = delete_comment`。
- `interaction.Handler.DeleteComment`:
  - 新增 `DELETE /api/comments/:commentId`。
  - 非作者删除返回 `403 forbidden`。
  - 删除成功后主动失效对应 note cache。
  - 重复删除返回 `deleted=false`，保持幂等。

关键测试：

- `TestRepositoryCreatesOutboxEventOnlyWhenCommentDeleted`
  - 验证首次删除评论写 `interaction.removed`。
  - 验证重复删除不重复产事件。
- `TestDeleteCommentIdempotentForAuthor`
  - 验证作者删除评论后 `comments_count` 减少。
  - 验证重复删除不会把计数减成负数。
- `TestDeleteCommentRejectsNonAuthor`
  - 验证非作者不能删除评论，计数不变。

复习重点：

- 评论删除和点赞/收藏取消一样，都属于“状态真实变化才产事件”。
- 软删除场景下，第二次删除查不到未删除记录，因此应该幂等返回，而不是继续扣计数。
- 写路径里的缓存失效应该在事务成功后执行，避免业务回滚但缓存先被删造成无意义抖动。

### Stage G: Complete Public Author Notes And Note Removal Flow

目标：补齐 API 文档中已经声明的 `GET /api/users/{userId}/notes` 和 `DELETE /api/notes/{noteId}`，并统一公共可见性规则。

优化方案：

- 公共内容可见性统一为 `notes.status = published AND notes.visibility = public`。
- 作者删除内容不做物理删除，而是把 `notes.status` 改为 `removed`，保留审计和生命周期信息。
- 删除接口只允许作者操作，非作者返回 `403 forbidden`。
- 重复删除返回 `deleted=false`，保持幂等。
- Redis 启用时，删除成功后主动失效：
  - `feed:latest:first-page`
  - `feed:hot:first-page`
  - `note:detail:{note_id}`
  - `note:counts:{note_id}`

主要实现：

- `note.Handler`:
  - 新增 `GET /api/users/:userId/notes`。
  - 新增 `DELETE /api/notes/:noteId`。
  - 注入可选 `CacheInvalidator`，只在真实删除成功后失效 Feed 和 Note cache。
- `note.Service`:
  - 新增 `PublicNotes`，负责分页参数归一化和 DTO 组装。
  - 新增 `DeleteOwn`，负责作者权限和幂等删除语义。
  - 修正 `Detail` 的公开可见性判断，非作者只能看到 `published + public`。
- `note.Repository`:
  - 新增 `FindPublicNotesByAuthorID`，按 `published_at DESC, id DESC` 返回作者公开内容。
  - 新增 `MarkNoteRemoved`，通过条件更新避免重复删除产生额外状态变更。
- `cache.NoteServiceCache`:
  - 对新增 service 方法做透传，避免 Redis 装饰器破坏接口。

关键测试：

- `TestPublicAuthorNotesOnlyReturnsPublishedPublicNotes`
  - 验证作者公开列表不会泄漏 pending/private/removed 或其他作者内容。
- `TestNoteDetailHidesPrivatePublishedNoteFromNonAuthor`
  - 验证 private published 内容只有作者本人可见。
- `TestDeleteOwnNoteSoftRemovesNote`
  - 验证作者删除后状态变为 `removed`，详情返回 404，重复删除幂等。
- `TestDeleteOwnNoteRejectsNonAuthor`
  - 验证非作者删除被拒绝且原状态不变。

复习重点：

- 文档中的 API 契约必须和路由实现对齐，否则面试演示时很容易被追问出缺口。
- `removed` 状态比物理删除更适合内容社区，因为它能保留治理、审计和运营后台所需的历史信息。
- 公开读路径要统一条件，不要详情页一套规则、列表页另一套规则。
- 缓存失效要跟随“真实状态变化”，重复删除这种幂等请求不需要反复删除缓存。

### Stage H: Complete Account Profile Contract

目标：补齐 API 文档中已经声明但未实现的账号接口，让账号模块从“能登录”升级为“有完整资料契约”。

主要实现：

- `account.Handler`:
  - 新增 `DELETE /api/sessions/current`。
  - 新增 `PATCH /api/users/me`。
  - 新增 `GET /api/users/:userId`。
- `account.Service`:
  - 新增 `UpdateProfile`，支持 PATCH 部分更新，未传字段保持原值，并校验 nickname、avatar_url、bio。
  - 新增 `PublicProfile`，只返回 active 用户的公开资料。
- `account.Repository`:
  - 新增 `UpdateProfile`，更新资料后重新读取用户，保证响应是数据库最终状态。
- `account.DTO`:
  - 新增 `PublicUserDTO`，只暴露 id、nickname、avatar_url、bio、created_at。

设计取舍：

- `DELETE /api/sessions/current` 当前采用 stateless JWT logout，服务端只返回 `logged_out=true`，不维护 Redis token blacklist。原因是当前项目主线是内容社区和 Feed 分发，V1 不引入刷新令牌、token 黑名单或多端会话管理，避免账号模块膨胀。
- 公开资料接口不返回 email、role、status，避免把管理态字段暴露给匿名调用方。
- profile cache key 已在缓存文档中约定，但读路径暂不接入 Redis；后续如果接入 `user:profile:{id}`，资料更新成功后应主动删除该 key。

关键测试：

- `TestAccountRoutesUpdateCurrentUserProfile`
  - 验证登录用户可以更新 nickname、avatar_url、bio，并且 `/users/me` 返回最新值。
- `TestAccountRoutesPatchProfileKeepsOmittedFields`
  - 验证 PATCH 只传 bio 时，nickname 和 avatar_url 保持原值。
- `TestAccountRoutesRejectInvalidProfileUpdate`
  - 验证空 nickname、超长 bio 和未认证更新会被拒绝。
- `TestAccountRoutesGetPublicProfile`
  - 验证公开资料只包含公开字段，不泄漏 email、role、status。
- `TestAccountRoutesPublicProfileNotFound`
  - 验证不存在用户返回 404。
- `TestAccountRoutesLogoutCurrentSession`
  - 验证 logout 需要认证，并返回 `logged_out=true`。

复习重点：

- 简历项目里的 API contract 要和实现保持一致，尤其是账号、内容、互动这种演示必走路径。
- 公开 DTO 和内部/current user DTO 要分开，不能为了省事复用含敏感字段的结构。
- stateless JWT logout 只能让客户端丢弃 token，不能让已签发 token 在过期前服务端失效；面试中要主动说明这个边界。

### Stage I: Complete Admin Note Search

目标：补齐 `GET /api/admin/notes`，让管理后台不仅能处理 review task，也能按内容状态检索 note，支撑“内容状态流与治理工作流”的简历叙事。

实现范围：

- 支持管理员按 `status` 过滤：
  - `pending_review`
  - `published`
  - `rejected`
  - `removed`
- 支持 `q` 关键词在 title/body 上做轻量模糊匹配。
- 支持 `limit/offset` 分页，沿用后台列表类接口的 offset 方案。
- 返回 note 基础信息、状态、可见性、review_task_id、互动计数和时间字段。
- 非管理员访问仍由 admin middleware 返回 `403 forbidden`。

主要实现：

- `review.AdminNoteDTO` / `AdminNoteListDTO`：后台内容列表专用 DTO。
- `review.Repository.SearchNotes`：基于 GORM 组合 `status` 和 `q` 查询。
- `review.Service.SearchNotes`：校验 status、归一化分页参数、转换 DTO。
- `review.Handler.SearchNotes`：新增 `/api/admin/notes` 路由。

设计取舍：

- 当前只做后台治理需要的轻量检索，不引入 Elasticsearch、倒排索引或复杂搜索平台。
- 关键词查询使用 MySQL `LIKE`，适合后台低频检索和学习项目演示；如数据规模继续扩大，可作为 P1 延伸到搜索服务。
- `status` 使用白名单校验，避免管理后台传错状态时默默返回空数据。

关键测试：

- `TestAdminNotesSearchFiltersByStatusAndKeyword`
  - 验证 `status=published&q=go` 只返回匹配状态和关键词的内容。
- `TestAdminNotesRejectsNonAdmin`
  - 验证普通用户不能访问后台内容列表。
- `TestAdminNotesRejectsInvalidStatus`
  - 验证非法状态返回 `400 invalid_status`。

复习重点：

- 后台查询接口要先定义清晰过滤条件和状态枚举，避免把“所有查询需求”都塞进一个无边界接口。
- 状态流项目里，后台列表的价值是让 pending/published/rejected/removed 可观察，而不是做复杂搜索引擎。
- Handler -> Service -> Repository 分层仍然适用于后台接口：HTTP 参数、业务校验、数据库查询各在各层。

## Implementation Notes

### Package Boundaries

当前包依赖方向保持为：

```text
cmd/server
  -> internal/review
  -> internal/interaction
  -> internal/note
  -> internal/outbox
```

`review` 和 `interaction` 通过本地接口接收 `OutboxEventCreator`，而不是暴露具体 helper 类型作为业务依赖。这样做的好处：

- 测试可以传入真实 outbox helper，也可以后续传入 fake。
- 业务包不需要知道 RabbitMQ publisher 的存在。
- 事务边界由 service/repository 控制，事件写入只是一条同事务数据库记录。

### Transaction Boundaries

审核决策：

```text
begin tx
  find review_task
  update review_task final status
  update notes.status / published_at
  insert review_task_events
  insert outbox_events(note.review_decided)
commit
invalidate redis cache
```

互动写入：

```text
begin tx
  insert/restore/delete interaction row
  update notes counter
  insert outbox_events(interaction.created/removed)
commit
invalidate note detail/count cache
```

作者删除内容：

```text
find note
check author_id
if status != removed:
  update notes.status = removed
invalidate feed first-page cache
invalidate note detail/count cache
```

账号资料更新：

```text
PATCH /api/users/me
  -> validate profile fields
  -> update users.nickname/avatar_url/bio
  -> return current user DTO
```

后台内容检索：

```text
GET /api/admin/notes?status=published&q=go
  -> require admin
  -> validate status filter
  -> query notes by status and title/body keyword
  -> return admin note list DTO
```

### Failure Policy

| 场景 | 当前策略 |
| --- | --- |
| outbox insert 失败 | 回滚业务事务 |
| RabbitMQ publish 失败 | 标记 `failed`，设置 `next_retry_at` |
| 连续失败超过 5 次 | 标记 `abandoned`，停止自动重试 |
| mark sent 失败 | 可能重复投递，依赖 consumer 幂等 |
| Redis 失效失败 | 不回滚业务事务，依赖短 TTL 兜底 |

## Verification Commands

阶段内已使用过的关键命令：

```bash
go test ./internal/ratelimit -run Test -v
go test ./internal/http -run TestRouterAppliesRateLimiter -v
go test ./internal/cache ./internal/note ./cmd/server
go test ./internal/interaction -run TestRepositoryCreatesOutbox -v
go test ./internal/outbox -run TestWorker -v
go test ./internal/review -run TestServiceDecideCreatesReviewDecidedOutboxEvent -v
go test ./internal/interaction ./internal/outbox ./cmd/server -v
go test ./...
```

提交前必须重新运行：

```bash
gofmt -w <changed-go-files>
go test ./...
```

## Resume Mapping

可以写进简历的后端表达：

- 基于 Gin/GORM/MySQL 实现内容发布、审核治理、互动和多场景 Feed，采用 Handler -> Service -> Repository 分层。
- 采用复合游标分页实现 latest/hot/following/tag Feed，避免 offset 深分页在无限滚动中的重复和漏刷。
- 使用 Redis cache-aside 缓存匿名 Feed 首页和公开内容详情，登录态 viewer state 通过批量 hydration 回填，避免共享缓存串用户。
- 基于 Redis `INCR + EXPIRE` 实现固定窗口限流，保护注册、登录、发布和互动写接口，Redis 故障时降级放行。
- 采用 Transactional Outbox 保证业务写入和事件发布一致性，内容发布、审核决策、互动行为在同一事务内写入 outbox event，worker 负责异步投递 RabbitMQ。
- 为 Outbox worker 设计 `pending/sent/failed/abandoned` 状态流，失败事件按 `next_retry_at` 重试，超过上限后进入人工处理状态。

## Good Follow-Up Questions

- 为什么 Outbox 比业务事务里直接调用 RabbitMQ 更可靠？
- Outbox 为什么只能保证 at-least-once，不能保证 exactly-once？
- interaction 幂等和 Outbox 幂等分别在哪里做？
- Redis 作为缓存和 MySQL 作为事实来源的边界怎么划分？
- 为什么登录态 Feed 不直接缓存完整 DTO？
- 如果要做 hot Feed Redis ZSET，interaction event consumer 应该如何设计？
- 如果要做人工 replay，`abandoned` event 需要哪些 admin/CLI 能力？
