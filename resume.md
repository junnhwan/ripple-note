<div alt="entry-title">
    <h3>知涟 - 内容社区与 Feed 分发平台</h3>
    <a href="https://github.com/junnhwan/ripple-note">github.com/junnhwan/ripple-note</a>
</div>

技术栈：Go, Gin, GORM, MySQL, Redis, RabbitMQ, JWT, Docker Compose, React, TypeScript

项目描述：知涟是一套面向内容发布、审核治理与 Feed 分发的社区平台，覆盖用户注册登录、内容发布、点赞收藏评论关注、多场景 Feed 浏览、后台审核、Redis 缓存、RabbitMQ + Outbox 异步事件和前端演示闭环。后端采用 Handler -> Service -> Repository 分层，核心业务状态以 MySQL 为准，Redis 与 RabbitMQ 作为性能优化和异步解耦组件。

项目亮点：

- **内容状态流与治理工作流**：设计 `pending_review -> published/rejected/removed` 内容状态和 `pending_agent/manual_required/admin_approved/admin_rejected` 审核任务状态，发布内容时在同一事务内创建内容、图片、标签和审核任务；提供后台审核接口，管理员审核通过/拒绝后同步更新内容状态、记录审核事件并失效相关缓存。

- **多场景 Feed 与复合游标分页**：实现最新流、热门流、关注流、标签流等 Feed 场景，采用 `published_at + id`、`hot_score + id` 等复合游标分页方式，避免 offset 分页在无限滚动下的重复、漏刷和深分页性能问题；Feed 查询只返回 `published + public` 内容，登录态通过批量 hydration 回填点赞、收藏、关注状态，将单页 SQL 查询数由约 121 次降至约 7 次。

- **Redis 缓存与一致性控制**：对匿名最新/热门 Feed 首页使用 Cache Aside 缓存和 30 秒短 TTL，登录态个性化字段不进入共享缓存，避免串用户状态；内容审核通过/拒绝、互动计数变化后主动失效 Feed 与 Note 相关 key，并在 Redis 不可用时回退 MySQL，保证业务正确性不依赖缓存。

- **接口保护与统一错误响应**：借鉴成熟 Java 后端项目的错误码与限流设计，补充 API 错误码分类，统一返回 `data/error/request_id`；基于 Redis `INCR + EXPIRE` 实现固定窗口限流，对注册、登录、发布、点赞、收藏、评论、关注等写接口进行分级保护，超限返回 `429 rate_limited`，Redis 故障时降级放行。

- **互动系统与计数一致性**：实现点赞、收藏、评论、关注等互动能力，通过事务同步维护行为记录和内容计数；点赞/收藏/关注使用唯一索引和软删除恢复保证接口幂等，重复操作不会重复加计数，取消操作不会把计数减成负数。

- **RabbitMQ + Outbox 最终一致性**：设计 `outbox_events` 事件表，内容发布、管理员审核决策和互动状态变更在业务事务内同步写入 `note.review_requested`、`note.review_decided`、`interaction.created/removed` 事件；独立 worker 轮询 pending/failed 事件并投递 RabbitMQ topic exchange，失败记录 `retry_count`、`next_retry_at`，超过重试上限进入 `abandoned`，避免业务写入成功但消息丢失造成状态分裂。

- **工程化交付与可演示闭环**：提供 React + Vite 前端 demo、Dockerfile、部署配置、种子数据脚本、OpenAPI 文档和学习日志，支持注册登录、发布内容、审核、Feed 展示、互动等完整演示流程；项目文档补充缓存一致性、Outbox 事件合约、外部项目借鉴分析和分阶段优化计划，便于面试展开讲解。

- **压测与性能优化证据**：使用 `loadseed` 构造 5 万笔记、60 万互动数据集，并通过 k6 在 4 vCPU / 3.8 GiB Docker Compose 单机环境压测；登录态 latest Feed RPS 由 107.58 提升至 676.49、P95 由 614.91ms 降至 106.90ms，匿名 latest Feed 达到 2683.62 RPS、P95 81.25ms，错误率均为 0.00%。

可展开追问：Feed 为什么不用 offset、登录态 Feed 如何避免串缓存、Outbox 如何解决消息与数据库事务一致性、interaction 事件如何避免重复投递副作用、Redis 限流失败为什么选择放行、Go 项目如何控制 handler/service/repository 边界。
