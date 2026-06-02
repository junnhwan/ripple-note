## 知涟 - 内容社区与 Feed 分发平台

个人项目 / 后端开发｜2026.05 - 至今
GitHub: https://github.com/junnhwan/ripple-note

**技术栈**：Go, Gin, GORM, MySQL, Redis, RabbitMQ, JWT, Docker Compose, React/Vite

**项目简介**：知涟是一套面向内容发布、审核治理与 Feed 分发的内容社区平台，覆盖用户注册登录、内容发布、点赞/收藏/评论/关注、多场景 Feed、后台审核、Redis 缓存、RabbitMQ + Outbox 异步事件和 Docker Compose 部署。后端采用 **Handler -> Service -> Repository** 分层，核心业务状态以 MySQL 为准，Redis 和 RabbitMQ 分别承担读性能优化与异步解耦。

**个人职责 / 技术亮点**：

- **后端分层架构与业务闭环**：基于 Gin 设计 RESTful API，按账号、内容、Feed、互动、审核、缓存、Outbox 拆分业务包；实现注册登录、个人资料、内容发布、公开详情、作者主页、后台内容检索等核心接口，保持 Handler 负责协议转换、Service 承载业务规则、Repository 封装 GORM 查询。

- **内容状态流与审核治理**：设计 `pending_review -> published/rejected/removed` 内容状态流和 `pending_agent/manual_required/admin_approved/admin_rejected/admin_removed` 审核任务状态流；发布内容时在同一事务内创建 note、图片、标签和 review task，管理员 `approve/reject/remove` 时同步更新内容状态、记录 `review_task_events`、写入 `note.review_decided` outbox event，并在事务提交后失效相关缓存。

- **多场景 Feed 与复合游标分页**：实现 latest、hot、following、tag 等 Feed 场景，使用 `published_at + id`、`hot_score + id` 复合游标替代 offset 深分页，降低无限滚动下重复/漏刷风险；Feed 公共查询只返回 `published + public` 内容，登录态通过批量 hydration 回填点赞、收藏、关注状态，将单页 SQL 查询数由约 **121 次降至 7 次**。

- **Redis Cache Aside 与一致性控制**：对匿名 latest/hot Feed 首页设置 30s 短 TTL 缓存，对公开 note detail 和互动计数设置独立缓存 key；登录态个性化字段不进入共享缓存，避免串用户状态；审核决策、内容下架、点赞/收藏/评论状态变化后主动失效 Feed 与 Note 相关缓存，Redis 异常时回退 MySQL，保证业务正确性不依赖缓存。

- **互动系统与计数幂等**：实现点赞、收藏、评论、关注等互动能力，通过唯一索引、软删除恢复和条件更新保证写接口幂等；重复点赞/收藏/关注不重复加计数，取消操作不会把计数减成负数，删除评论采用 soft delete 并只在首次删除时减少 `comments_count` 和写入 `interaction.removed` 事件。

- **RabbitMQ + Transactional Outbox**：设计 `outbox_events` 事件表，内容发布、管理员审核决策和互动状态变更在业务事务内同步写入 `note.review_requested`、`note.review_decided`、`interaction.created/removed` 事件；独立 worker 轮询 `pending/failed` 事件投递 RabbitMQ topic exchange，失败记录 `retry_count` 和 `next_retry_at`，超过 5 次进入 `abandoned`，避免业务写入成功但消息丢失导致状态分裂。

- **接口保护与工程化交付**：统一 API 响应为 `data/error/request_id`，补充错误码文档；基于 Redis `INCR + EXPIRE` 实现固定窗口限流，保护注册、登录、发布、点赞、收藏、评论、关注等高风险写接口，Redis 故障时降级放行；提供 Dockerfile、Docker Compose、seed/loadseed、API 设计文档、缓存一致性文档、Outbox 事件合约和分阶段学习日志。

- **压测与性能优化证据**：使用 `loadseed` 构造 **5 万笔记、60 万互动** 数据集，并通过 k6 在 4 vCPU / 3.8 GiB Docker Compose 单机环境压测；登录态 latest Feed RPS 从 **107.58 提升至 676.49**，P95 从 **614.91ms 降至 106.90ms**；匿名 latest Feed 达到 **2683.62 RPS / P95 81.25ms**，错误率均为 **0.00%**。

**可展开追问**：Feed 为什么不用 offset、登录态 Feed 如何避免串缓存、Outbox 如何解决数据库事务与消息发送一致性、interaction 事件如何避免重复副作用、Redis 限流失败为什么选择放行、Go 项目如何控制 Handler/Service/Repository 边界。
