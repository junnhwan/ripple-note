# 知涟 Cache And Consistency Design

## Purpose

本文定义知涟 Redis 缓存策略，包括缓存对象、key 命名、TTL、失效时机、降级行为和一致性边界。

缓存的目标是降低 Feed 和内容读取压力，而不是改变业务事实来源。知涟的核心业务状态仍以 MySQL 为准。

## Design Principles

| 原则 | 说明 |
| --- | --- |
| MySQL 是事实来源 | Redis 只保存可重建的读模型、计数快照或派生索引 |
| 缓存必须可降级 | Redis 不可用时，API 应回退到 MySQL，不改变业务语义 |
| 共享缓存不能包含用户态 | `viewer_liked`、`viewer_favorited`、`viewer_following` 不能写入匿名/公共缓存 |
| 短 TTL + 主动失效 | Feed 首页使用短 TTL，审核/互动等写路径主动删除相关 key |
| 只缓存高频读路径 | 优先 Feed 首页、内容详情、计数、用户公开资料，不缓存低频组合查询 |
| 先保证正确性，再优化命中率 | 宁可缓存少一点，也不要出现已拒绝内容仍长期可见 |

## Current Implementation Snapshot

当前代码中的缓存边界：

| 能力 | 状态 | 说明 |
| --- | --- | --- |
| Redis client wrapper | 已有 | `internal/cache/redis.go` 提供 JSON get/set/delete |
| 匿名 latest Feed 首页缓存 | 已有 | `feed:latest:first-page`，只在 `viewerID == 0` 且无 cursor 时使用 |
| 匿名 hot Feed 首页缓存 | 已有 | `feed:hot:first-page`，只在 `viewerID == 0` 且无 cursor 时使用 |
| following Feed 缓存 | 不缓存 | 用户强相关，当前直接走 MySQL |
| tag Feed 缓存 | 不缓存 | tag 组合多，当前直接走 MySQL |
| note detail/count key | 已接入 | 匿名公开详情走 cache-aside，并同时写入计数快照 |
| profile key | 已约定 | 已有 key helper，读路径可后续接入 |
| 审核后失效 Feed 和 Note key | 已有 | admin 决策后删除 Feed 首页和 note 相关 key；后续内容分析回调复用同一规则 |
| 互动后失效 Note key | 已有 | like/favorite/comment 后删除 note detail/count key |

## Cache Key Catalog

| Key | 内容 | TTL | 当前状态 | 失效时机 |
| --- | --- | --- | --- | --- |
| `feed:latest:first-page` | 匿名 latest Feed 第一页 DTO | 30s | 已接入 | note 发布、拒绝、移除、审核决策后删除 |
| `feed:hot:first-page` | 匿名 hot Feed 第一页 DTO | 30s | 已接入 | note 状态变化、互动导致热度变化后删除或等待 TTL |
| `note:detail:{id}` | 已发布 note 详情 DTO | 2m | 已接入匿名详情 | note 内容变更、状态变更、审核决策、移除 |
| `note:counts:{id}` | like/favorite/comment 计数快照 | 1m | 已接入匿名详情写入 | like、unlike、favorite、unfavorite、comment、delete comment |
| `user:profile:{id}` | 用户公开资料 DTO | 5m | key 已约定，读路径待补 | 用户修改昵称、头像、简介、状态 |
| `rate:auth:register:ip:{ip}` | 注册限流计数 | 1h | 已接入 | TTL 到期自动清理 |
| `rate:auth:login:ip:{ip}` | 登录限流计数 | 1m | 已接入 | TTL 到期自动清理 |
| `rate:publish:user:{user_id}` | 发布限流计数 | 1m | 已接入 | TTL 到期自动清理 |
| `rate:interaction:{action}:user:{user_id}` | 互动限流计数 | 1m | 已接入 | TTL 到期自动清理 |
| `feed:hot:zset` | hot Feed 排名索引 | 视实现而定 | P1 | interaction event 增量更新，定期重算 |

TTL 取值应在代码中集中定义。当前 `internal/cache/feed_cache.go` 中：

```text
FeedTTL    = 30s
DetailTTL  = 2m
CountsTTL  = 1m
ProfileTTL = 5m
```

## Feed Cache Rule

Feed 缓存只缓存匿名第一页：

```text
if cursor == "" && viewerID == 0:
  read/write Redis first-page cache
else:
  query MySQL and hydrate directly
```

这样做的原因：

- 匿名首页是最高频读取场景。
- 第一页对性能收益最大，且短 TTL 能控制新内容延迟。
- 登录态需要 viewer state，不能直接复用共享缓存。
- 后续分页受 cursor 影响，缓存组合数量会快速膨胀。

## Viewer State Hydration

共享 Feed 缓存不得包含用户态字段。

正确的读取流程：

```text
Feed page IDs
  -> load notes/images/tags/authors/counts
  -> if viewer logged in:
       batch load liked/favorited/following state
  -> assemble response
```

匿名缓存中可以保存：

- note id、title、body preview、images。
- author public profile。
- public stats。
- ranking fields and next cursor。
- 已发布公开 note detail。
- like/favorite/comment 计数快照。

匿名缓存中不能保存：

- 当前用户是否点赞。
- 当前用户是否收藏。
- 当前用户是否关注作者。
- 当前用户是否有管理权限。
- 任何 token、email、internal trace 信息。

## Consistency Rules

### Content Status Controls Visibility

只有满足以下条件的内容才能进入公共 Feed 和公共详情页：

```text
notes.status = published
notes.visibility = public
```

以下状态不得进入公共 Feed：

- `draft`
- `pending_review`
- `rejected`
- `removed`

### Review Decision Invalidation

admin 决策成功后，应在数据库事务完成后执行缓存失效。后续内容分析回调如果重新启用，也复用同一规则：

```text
review decision committed
  -> delete feed:latest:first-page
  -> delete feed:hot:first-page
  -> delete note:detail:{note_id}
  -> delete note:counts:{note_id}
```

原因：

- 审核通过会让新内容从不可见变为可见。
- 审核拒绝或移除会让内容从可见变为不可见。
- Feed 首页最容易受新内容和状态变更影响。
- Note detail 不能继续返回旧状态。

如果缓存删除失败，不应回滚已经成功的业务事务，但必须记录日志。短 TTL 会作为兜底。

### Interaction Count Consistency

like/favorite/comment 的正确性由 MySQL 事务和唯一索引保证：

```text
interaction write transaction
  -> insert/restore/delete interaction row
  -> update notes count
  -> commit
  -> delete note:counts:{note_id}
  -> delete note:detail:{note_id}
```

Redis count cache 只能作为读优化，不能作为唯一事实来源。

计数常见坑：

- 重复点赞不能重复加一。
- 重复取消不能减到负数。
- soft delete 恢复时只能加一次。
- comment 删除后要同步减少 `comments_count`。
- 热榜分数如果依赖互动计数，要同时考虑 `feed:hot:first-page` 或 `feed:hot:zset` 更新。

### Public Profile Consistency

用户公开资料缓存只保存公开字段：

- user id。
- nickname。
- avatar url。
- bio。
- public stats。

不得缓存：

- password hash。
- email。
- role 权限细节。
- token 状态。

用户修改资料后删除：

```text
user:profile:{id}
```

Feed 中作者信息可以接受短时间旧值，但资料页应优先删除后重建。

## Cache-Aside Flow

读路径采用 cache-aside：

```text
read API
  -> Redis GET
  -> hit: return cached DTO
  -> miss:
       query MySQL
       assemble DTO
      Redis SET with TTL
      return DTO
```

当前接入范围：

- 匿名 `GET /api/notes/{noteId}` 会读取 `note:detail:{id}`。
- 缓存 miss 后从 MySQL 组装 `NoteDTO`，只在 `status=published` 且 `visibility=public` 时写入 `note:detail:{id}`。
- 同一次 miss 回源会写入 `note:counts:{id}`，保存 like/favorite/comment 计数快照。
- 登录态 detail 暂不读共享 detail cache，避免后续加入 viewer state 后出现串用户状态。

写路径采用主动失效：

```text
write API
  -> MySQL transaction
  -> commit
  -> Redis DEL affected keys
```

不建议在 V1 写路径同步重建缓存，因为：

- 增加写请求延迟。
- 多个写操作并发时容易互相覆盖。
- 需要维护更多序列化 DTO 逻辑。

## Failure And Degrade Behavior

| 场景 | 期望行为 |
| --- | --- |
| Redis 未启用 | 所有接口直接走 MySQL |
| Redis GET 超时或失败 | 记录日志，回退 MySQL |
| Redis SET 失败 | 忽略缓存写入错误，返回 MySQL 查询结果 |
| Redis DEL 失败 | 记录日志，依赖短 TTL 兜底 |
| 缓存数据 JSON 反序列化失败 | 删除该 key，回退 MySQL |
| 热点 key 同时失效 | P1 用 singleflight 或互斥锁降低击穿 |

## TTL And Jitter

当前 TTL 已经较保守。P1 可增加随机抖动，避免大量 key 同时过期：

```text
real_ttl = base_ttl + random(0, base_ttl * 20%)
```

适合加入 jitter 的 key：

- `note:detail:{id}`
- `note:counts:{id}`
- `user:profile:{id}`

Feed 首页 TTL 很短，也可以保持固定 30s，便于压测和观察。

## Hot Feed Evolution

当前 hot Feed 可以先使用 MySQL：

```text
ORDER BY hot_score DESC, id DESC
```

P1 可升级为 Redis ZSET：

```text
feed:hot:zset
member = note_id
score = hot_score
```

更新来源：

- `interaction.created`
- `interaction.removed`
- `note.review_decided`
- 定时重算任务

注意事项：

- Redis ZSET 只负责候选集合和排序。
- 最终返回前仍要从 MySQL 校验 `status = published` 和 `visibility = public`。
- 删除、拒绝、移除内容时必须从 ZSET 移除或在读取时过滤。

## Rate Limiting Design

当前已加入轻量 Redis 固定窗口限流，目标是保护高风险写接口，而不是构建完整风控系统。

| Key | 保护接口 | 建议窗口 |
| --- | --- | --- |
| `rate:auth:register:ip:{ip}` | 注册 | 5 次 / 1h |
| `rate:auth:login:ip:{ip}` | 登录 | 10 次 / 1m |
| `rate:publish:user:{user_id}` | 发布内容 | 10 次 / 1m |
| `rate:interaction:like:user:{user_id}` | 点赞 | 60 次 / 1m |
| `rate:interaction:favorite:user:{user_id}` | 收藏 | 60 次 / 1m |
| `rate:interaction:comment:user:{user_id}` | 评论 | 20 次 / 1m |
| `rate:interaction:follow:user:{user_id}` | 关注 | 30 次 / 1m |

实现采用固定窗口：

```text
INCR key
if count == 1: EXPIRE key window
if count > limit: reject with RATE_LIMITED
```

降级规则：

- Redis 启用时才注入限流中间件。
- Redis `INCR/EXPIRE` 失败时记录能力可后续补充，但请求当前直接放行，避免业务正确性依赖 Redis。
- 用户态限流在全局中间件阶段直接解析 Authorization token；解析失败时降级为 IP 维度限流。
- 当前不引入滑动窗口、令牌桶、Redisson 或复杂风控。
- AI/Agent 相关内部接口当前暂缓优化，因此本阶段不对内容分析 callback 配置限流规则。

## Testing Checklist

新增或调整缓存逻辑时，应至少验证：

- 匿名 latest 第一页第二次请求命中缓存。
- 登录态 Feed 不读取匿名缓存中的 viewer state。
- pending/rejected/removed 内容不会出现在缓存 Feed 中。
- admin approve 后 latest/hot 首页缓存被删除。
- admin reject 后 note detail 和 Feed 缓存被删除；后续内容分析 reject 复用同一规则。
- like/favorite/comment 后 `note:counts:{id}` 被删除。
- Redis 关闭时 Feed、详情和互动 API 仍能正常走 MySQL。
- Redis 关闭时限流不影响核心业务接口。
- 超出写接口限流时返回 `429` 和 `rate_limited`。
- 缓存中出现损坏 JSON 时能回退 MySQL。
- Hot cursor 在相同 `hot_score` 下使用 `id` 打破排序并且不重复。

## Interview Explanation

可以这样解释知涟缓存设计：

```text
知涟没有把 Redis 当作事实来源，而是使用 cache-aside 缓存高频读模型。
匿名 Feed 首页使用 30 秒短 TTL，审核和互动写路径主动失效相关 key。
登录态的点赞、收藏、关注状态不进入共享缓存，而是在 Feed hydration 阶段批量补齐，避免串用户状态。
如果 Redis 故障，系统回退 MySQL，业务正确性不依赖缓存可用性。
```
