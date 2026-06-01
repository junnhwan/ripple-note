# 知涟 Backend Design Borrowing Report

## Purpose

本文记录对本地其他后端项目的调研结论，目标不是把其他项目的复杂度搬进知涟，而是筛选能强化当前项目主线的设计：

- 内容状态流与治理工作流。
- 多场景 Feed 与复合游标分页。
- Redis 缓存与一致性设计。
- RabbitMQ + Outbox 最终一致性。
- 后续内容质量分析扩展边界。

知涟仍然定位为内容社区与 Feed 分发平台。外部项目只能作为工程设计参考，不能把项目扩张成短视频平台、复杂微服务系统、搜索推荐平台或全量风控系统。

## Investigation Scope

本次重点查看和对比了以下项目：

| 来源 | 主要参考价值 | 对知涟的适配结论 |
| --- | --- | --- |
| `D:\dev\learn_proj\java\zhiguang_be` | Java 后端分层、统一响应、错误码、认证、业务服务组织方式 | 最值得借鉴工程化边界和 API 规范，不直接照搬 Java 框架形态 |
| `D:\dev\learn_proj\java\hualvqing\DokiTV` | 视频类项目的媒体、异步、缓存、部署思路 | 只借鉴局部基础设施经验，不引入视频播放、长连接、弹幕等非目标能力 |
| `D:\dev\learn_proj\java\xiaohashu` | 小红书式内容社区、互动、缓存和 Feed 场景 | 与知涟产品形态相近，适合借鉴内容/互动/Feed 的边界设计 |
| `D:\dev\learn_proj\go\feedsystem_video_go` | Go Feed 读链路、缓存、排序和分页 | 可借鉴 Feed 性能优化思路，但避免视频领域耦合 |
| `D:\dev\learn_proj\go\GCFeed` | Go Feed 服务结构、游标分页、缓存分层 | 可借鉴轻量 Feed 设计和压测叙事 |
| `DOVideo-AI`、`AgentX`、`ThoughtCoding` 等抽样项目 | Agent、AI 编排、内容分析和工程组织参考 | 本轮暂缓，只作为远期参考 |

## Summary Recommendation

当前最应该借鉴的是“设计文档和边界意识”，不是更多技术栈。

知涟现在已经具备内容发布、审核流、Feed、互动、Redis 首页缓存、Outbox 和前端演示的雏形。下一步更有价值的是把这些能力讲清楚并收紧一致性规则：

- 补齐 API 错误码映射和业务错误规范。
- 明确缓存 key、TTL、失效时机、降级策略。
- 明确 Outbox event topic、payload、重试和幂等规则。
- 补充轻量限流，保护注册、登录、发布和互动写接口。
- 继续用 MySQL + Redis + RabbitMQ 的单体分层架构，不提前拆微服务。

## Designs Worth Borrowing Now

| 来源项目 | 可借鉴设计 | 适配到知涟的方式 | 阶段 | 风险/取舍 |
| --- | --- | --- | --- | --- |
| `zhiguang_be` | 统一响应、错误码、异常/错误分层 | 在 `docs/04-api-design.md` 后补错误码表，区分参数错误、认证错误、权限错误、状态冲突、资源不存在、内部错误 | 当前 | Go 不应照搬 Spring 全局异常体系，应保留显式 error return |
| `zhiguang_be` | handler/service/repository 分层 | 继续保持当前 Go 分层，避免 handler 直接拼 SQL 或承担事务编排 | 当前 | 不要为了“模式感”引入过多接口 |
| `xiaohashu` | 内容社区互动边界 | 点赞、收藏、评论、关注保持幂等，计数以 MySQL 为准，Redis 只做加速 | 当前 | 并发计数需要事务和唯一索引兜底 |
| `GCFeed` / `feedsystem_video_go` | Feed 缓存不能混入用户态 | 匿名首页缓存只保存公共内容，登录态的 `viewer_liked`、`viewer_favorited`、`viewer_following` 通过二次 hydration 得到 | 当前 | 如果缓存登录态页面，会出现串用户状态风险 |
| `GCFeed` / `feedsystem_video_go` | Feed 复合游标 | 知涟继续使用 `published_at + id`、`hot_score + id`，避免 offset 分页 | 当前 | 新增排序字段时必须同步 cursor 设计 |
| `zhiguang_be` / `xiaohashu` | Redis key 规范 | 形成独立缓存设计文档，记录 key、TTL、失效和降级行为 | 当前 | 文档必须跟实现同步，否则比没有文档更危险 |
| `zhiguang_be` | 基础限流 | 对登录、注册、发布、互动写接口加轻量 Redis 限流 | 当前 | 先做固定窗口即可，不引入复杂风控 |
| `DokiTV` | 部署与中间件连通性 | 参考 Docker Compose 中 MySQL/Redis/RabbitMQ 的健康检查和依赖顺序 | 当前/P1 | 不借视频域的播放、转码、长连接复杂度 |
| `xiaohashu` / `DokiTV` | 对象存储上传 | P1 可加入 MinIO/OSS 预签名上传，替换本地图片存储 | P1 | V1 本地上传更适合快速演示 |

## Designs To Borrow Later

这些能力对项目有价值，但不适合马上打断主线。

| 设计 | 借鉴来源 | 放到后续的原因 | 建议落地方式 |
| --- | --- | --- | --- |
| Refresh token 白名单和登录审计 | Java 后端项目常见认证体系 | 当前 JWT 已能支撑演示，过早做会拉长账户模块 | P1 增加 `refresh_tokens` / `login_audits`，Redis 保存有效 token 版本 |
| Redis ZSET 热门 Feed | Feed 项目、内容社区项目 | 当前 MySQL `hot_score` 基线足够解释，ZSET 适合压测优化阶段 | interaction event 更新 `feed:hot:zset`，定期回写或重算 |
| Following Feed Redis 索引 | Feed 项目 | 关注关系和内容量增长后才有收益 | 为每个用户维护 `feed:following:{user_id}` 或按 followee 聚合拉取 |
| 热门评论/楼中楼优化 | 内容社区项目 | V1 评论列表简单即可，先保证发布、展示、删除一致性 | P1 做 `comments_count` 缓存、热评排序和分页 |
| MinIO/OSS 预签名上传 | DokiTV、内容平台项目 | 当前本地上传利于学习 Gin multipart 和 Compose 部署 | P1 引入 `storage` 接口，保留本地实现，新增对象存储实现 |
| 消息死信队列和事件回放 UI | MQ 项目 | 当前 Outbox retry 已能讲清楚最终一致性 | P1 增加 abandoned/dead-letter 状态和 admin replay 命令 |
| Prometheus/pprof | Go 服务项目 | 当前先完成业务闭环和压测文档 | 压测阶段加入指标，支撑简历量化 |

## Designs Not Recommended For 知涟

以下设计会明显偏离当前项目边界，暂不建议引入。

| 不建议设计 | 原因 |
| --- | --- |
| Canal + Kafka 全量数据同步 | 对当前单体 MySQL + RabbitMQ 过重，会稀释 Outbox 学习重点 |
| 视频转码、播放、弹幕、长连接 | DokiTV 的视频域能力和知涟内容社区主线不一致 |
| 微服务拆分 | 项目规模和学习目标不需要，单体模块化更容易完成和讲清楚 |
| 复杂推荐系统或机器学习排序 | 现阶段 Feed 应保持可解释，优先游标、索引、缓存和基准排序 |
| Elasticsearch/RAG 搜索 | 搜索不是 P0/P1 主线，容易把项目扩张为搜索平台 |
| 复杂 Redis bitmap/SDS 计数系统 | 互动计数先由 MySQL 事务和唯一索引保证正确性，Redis 只做缓存或派生视图 |
| 全量风控平台 | 当前项目只保留平台治理边界和最终状态，不扩展成风控平台 |

## Suggested Landing Order

### 1. Documentation First

先补齐三个能提升架构可解释性的文档：

- `docs/13-backend-design-borrowing.md`: 本文，记录外部项目哪些能借、哪些不能借。
- `docs/cache-and-consistency.md`: Redis key、TTL、失效、降级、一致性规则。
- `docs/events-and-outbox.md`: RabbitMQ + Outbox topic、payload、worker、重试、幂等。

这一步不改变代码，但能让后续实现更稳定，也方便面试时说明“为什么这样设计”。

### 2. API Error Codes

借鉴 Java 项目的统一错误码意识，给知涟建立一张错误码表：

| 类别 | 示例 |
| --- | --- |
| 参数错误 | `INVALID_ARGUMENT` |
| 未认证 | `UNAUTHORIZED` |
| 权限不足 | `FORBIDDEN` |
| 资源不存在 | `NOT_FOUND` |
| 状态冲突 | `STATE_CONFLICT` |
| 频率限制 | `RATE_LIMITED` |
| 内部错误 | `INTERNAL_ERROR` |

实现时仍使用 Go 的显式 error return，不需要照搬 Java 的全局异常模型。

### 3. Cache Read Path Tightening

当前缓存重点应保持保守：

- 匿名 `latest/hot` Feed 首页缓存。
- 详情、计数、用户资料 cache key 先统一，再逐步接入 read-through。
- 审核决策、互动写入后主动失效相关 key。
- 登录态 viewer state 始终单独 hydration，不进入共享缓存。

### 4. Outbox Contract Tightening

Outbox 后续应补齐：

- event catalog。
- payload schema。
- retry/backoff 规则。
- abandoned/dead-letter 状态。
- consumer 幂等约束。

这比单纯“接上 RabbitMQ”更有简历价值，因为它能解释业务事务和消息投递之间的一致性问题。

### 5. Lightweight Protection

加入 Redis 限流，但控制范围：

- `POST /api/users`
- `POST /api/sessions`
- `POST /api/notes`
- interaction 写接口。
后续内部内容分析接口可以单独评估是否纳入限流，本轮暂缓。

限流只保护接口，不做复杂风控平台。

## Resume Story Mapping

| 简历故事线 | 本次借鉴能强化的点 |
| --- | --- |
| 内容状态流与治理工作流 | 从 Java 项目借鉴业务错误码、状态冲突表达、审计事件表意识 |
| 多场景 Feed 与复合游标分页 | 从 Go Feed 项目借鉴 cursor、hydration、缓存和压测叙事 |
| Redis 缓存与一致性设计 | 从内容社区项目借鉴 key 规范、TTL、主动失效和降级策略 |
| RabbitMQ + Outbox 最终一致性 | 从 MQ 项目借鉴 topic、payload、重试、死信和幂等消费者 |
| 后续内容分析扩展 | 暂缓展开，当前只保留平台状态和最终可见性边界 |

## Decision Record

- 采用外部项目的工程设计经验，不照搬技术复杂度。
- 知涟继续保持 Go 单体模块化架构。
- P0/P1 的重点仍是内容发布、治理、Feed、互动、缓存、Outbox、部署和压测。
- DokiTV 只作为中间件和部署参考，不作为产品形态参考。
- 内容分析集成暂缓，当前优先完善 Go 后端主线。
