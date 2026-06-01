# 知涟 Events And Outbox Design

## Purpose

本文定义知涟的 RabbitMQ + Transactional Outbox 设计，用来支撑内容治理、缓存失效、通知、统计和后续内容分析扩展中的异步事件流。

Outbox 的核心价值不是“用了 MQ”，而是解决业务数据库事务和消息发布之间的一致性问题。

## Design Principles

| 原则 | 说明 |
| --- | --- |
| 业务写入和 outbox 写入在同一个 MySQL 事务中 | 业务成功则事件一定可被后续投递，业务失败则不会留下孤儿事件 |
| RabbitMQ 不参与核心业务正确性 | MQ 暂时不可用时，业务表和 outbox 表仍保持一致，worker 后续重试 |
| 至少一次投递 | producer 侧允许重复投递，consumer 必须幂等 |
| topic 表达业务事件 | routing key 使用 topic，例如 `note.review_requested` |
| payload 稳定且向后兼容 | 新字段只追加，不随意改名或改变含义 |
| 失败可观察、可重试、可人工处理 | 当前已有 `abandoned` 终态，P1 可继续增加 replay/dead-letter 工具 |

## Current Implementation Snapshot

当前代码已经具备以下能力：

| 能力 | 状态 | 说明 |
| --- | --- | --- |
| `outbox_events` model | 已有 | `internal/outbox/model.go` |
| publish note 写 outbox | 已有 | note 发布事务中创建 `note.review_requested` |
| worker 轮询 outbox | 已有 | `internal/outbox/publisher.go` |
| retry failed events | 已有 | `failed` + `next_retry_at` 到期后重新处理 |
| abandon after max retries | 已有 | 默认失败超过 5 次后进入 `abandoned`，停止自动重试 |
| RabbitMQ publisher | 已有 | topic exchange，routing key = event topic |
| NopPublisher | 已有 | RabbitMQ 不启用时可本地演示 |
| review decision event | 已有 | admin 决策事务内创建 `note.review_decided` |
| interaction events | 已有 | like/favorite/comment/follow 成功创建 `interaction.created`，取消成功创建 `interaction.removed` |
| consumer 幂等规范 | 待补 | 需要在缓存、通知、内容分析等消费者中明确 |
| replay/dead-letter 管理工具 | P1 | 当前已有自动停止重试，人工 replay 工具待补 |

当前默认 RabbitMQ exchange 来自配置，默认值为：

```text
ripple-note-events
```

## Outbox Table

逻辑字段：

| 字段 | 含义 |
| --- | --- |
| `id` | outbox event id，全局递增，可作为消费幂等 key 的一部分 |
| `topic` | 事件名和 RabbitMQ routing key |
| `aggregate_type` | 聚合类型，如 `note`、`review_task`、`interaction` |
| `aggregate_id` | 聚合 ID |
| `payload` / `payload_json` | JSON 事件体。当前 Go model 字段为 `Payload` |
| `status` | `pending`、`sent`、`failed`、`abandoned` |
| `retry_count` | 已失败次数 |
| `next_retry_at` | 下一次允许重试时间 |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |

说明：既有数据模型文档中使用 `payload_json` 表达 JSON 事件体，当前代码字段名是 `Payload`，GORM 默认列名为 `payload`。后续如果做显式迁移，可以统一列名，但不影响 Outbox 模式本身。

## Event State Machine

当前状态机：

```text
pending
  -> sent
  -> failed

failed
  -> sent        after retry success
  -> failed      after retry failure
  -> abandoned   retry_count exceeds max retry
```

P1 可继续扩展：

```text
abandoned
  -> pending     manual replay
  -> dead_letter manual inspection
```

状态语义：

| 状态 | 含义 |
| --- | --- |
| `pending` | 已随业务事务落库，等待 worker 发布 |
| `sent` | worker 已成功发布到 RabbitMQ 或 NopPublisher |
| `failed` | 发布失败，等待 `next_retry_at` 后重试 |
| `abandoned` | 多次失败后停止自动重试，等待人工处理 |

## Worker Algorithm

当前 worker 逻辑可以概括为：

```text
every interval:
  events = find status = pending
           or status = failed and next_retry_at <= now
  for each event:
    publish to RabbitMQ exchange with routing key = topic
    if success:
      mark sent
    else:
      retry_count += 1
      if retry_count > 5:
        mark abandoned
      else:
        mark failed
        next_retry_at = now + backoff
```

当前重试间隔是固定 30s。P1 可改成指数退避：

```text
next_retry = now + min(30s * 2^retry_count, 30m)
```

当前已增加最大重试次数：

```text
if retry_count > 5:
  status = abandoned
```

## RabbitMQ Routing

建议统一使用 topic exchange：

```text
exchange = ripple-note-events
routing_key = event.topic
```

建议队列：

| Queue | Binding | 用途 |
| --- | --- | --- |
| `ripple.review` | `note.review_requested` | 触发或记录内容分析任务 |
| `ripple.cache` | `note.*`、`interaction.*`、`follow.*` | 异步缓存失效或派生索引维护 |
| `ripple.notification` | `note.review_decided`、`interaction.created`、`follow.created` | 用户通知 |
| `ripple.audit` | `#` | 事件审计和调试 |

V1 可以只有 publisher，没有完整 consumer。只要 Outbox 表和 worker 语义清晰，后续 consumer 可以增量接入。

## Event Catalog

### `note.review_requested`

| 字段 | 内容 |
| --- | --- |
| Producer | note publish transaction |
| Aggregate | `note` |
| Timing | 用户发布内容并创建 review task 后 |
| Current status | 已实现 |
| Consumers | 内容分析扩展触发器、review worker、audit |

Payload:

```json
{
  "note_id": 1001,
  "author_id": 2001
}
```

语义：

- note 已进入 `pending_review`。
- review task 已创建并处于 `pending_agent`。
- 该事件可以用来触发后续内容分析扩展，但当前后端主线的正确性不依赖外部消费者立即消费。

### `note.review_decided`

| 字段 | 内容 |
| --- | --- |
| Producer | admin review decision；后续可扩展为内容分析 callback |
| Aggregate | `note` |
| Timing | 内容状态被决策后 |
| Current status | admin 决策已实现 |
| Consumers | cache invalidation、notification、audit、stats |

Payload:

```json
{
  "note_id": 1001,
  "task_id": 3001,
  "author_id": 2001,
  "decision": "approve",
  "actor_type": "admin",
  "actor_id": 9001,
  "note_status": "published"
}
```

语义：

- 当前已实现 admin `approve/reject` 决策事件。
- `actor_type` 当前为 `admin`，后续接入内容分析扩展时可增加其他 actor。
- 事件消费者不能直接推断所有通过都来自同一来源，应以 payload 中的 `actor_type` 和 `decision` 为准。

### `interaction.created`

| 字段 | 内容 |
| --- | --- |
| Producer | like/favorite/comment/follow 写操作 |
| Aggregate | `note`、`comment` 或 `follow` |
| Timing | 互动写事务成功后 |
| Current status | 已实现 |
| Consumers | count cache、hot ranking、notification、audit |

Payload:

```json
{
  "note_id": 1001,
  "user_id": 2001,
  "action": "like"
}
```

`action` 示例：

- `like`
- `favorite`
- `comment`
- `follow`

说明：

- 只有真实状态变化才产生事件；重复点赞、重复收藏、重复关注不会重复产生事件。
- comment 事件的 aggregate 是 `comment`，payload 额外包含 `comment_id`。
- follow 事件的 aggregate 是 `follow`，payload 包含 `follower_id` 和 `followee_id`。

### `interaction.removed`

| 字段 | 内容 |
| --- | --- |
| Producer | unlike/unfavorite/delete comment/unfollow |
| Aggregate | `note`、`comment` 或 `follow` |
| Timing | 互动取消事务成功后 |
| Current status | 已实现 unlike/unfavorite/delete comment/unfollow |
| Consumers | count cache、hot ranking、notification cleanup、audit |

Payload:

```json
{
  "note_id": 1001,
  "user_id": 2001,
  "action": "unlike"
}
```

删除评论时 aggregate 为 `comment`，payload 额外包含 `comment_id`，`action = "delete_comment"`。

### `follow.created` / `follow.removed`

| 字段 | 内容 |
| --- | --- |
| Producer | follow/unfollow 写操作 |
| Aggregate | `follow` |
| Timing | 关注关系变更成功后 |
| Current status | 由 `interaction.created` / `interaction.removed` 承载 |
| Consumers | following feed index、notification、audit |

Payload:

```json
{
  "follower_id": 2001,
  "followee_id": 3001
}
```

### `notification.created`

| 字段 | 内容 |
| --- | --- |
| Producer | review/interaction/follow consumer |
| Aggregate | `notification` |
| Timing | 生成用户可见通知时 |
| Current status | P1 |
| Consumers | frontend notification list、audit |

Payload:

```json
{
  "recipient_id": 2001,
  "type": "review_result",
  "source_id": 1001
}
```

## Transaction Boundaries

### Publish Note

正确流程：

```text
begin transaction
  insert notes(status = pending_review)
  insert note_images
  upsert tags and note_tags
  insert review_tasks(status = pending_agent)
  insert outbox_events(topic = note.review_requested)
commit
```

不能这样做：

```text
insert notes
commit
publish RabbitMQ directly
```

原因：如果业务提交成功后 RabbitMQ 发布失败，就会出现“内容已待审但事件丢失”的不一致。

### Review Decision

建议流程：

```text
begin transaction
  update review_tasks
  insert review_task_events
  update notes.status / published_at
  insert outbox_events(topic = note.review_decided)
commit
delete affected Redis keys
```

缓存失效可以同步执行，也可以由 `note.review_decided` consumer 执行。当前直接主动失效更简单；P1 可以转为事件驱动失效。

### Interaction Write

建议流程：

```text
begin transaction
  insert/restore/delete interaction row
  update notes counters
  insert outbox_events(topic = interaction.created or interaction.removed)
commit
delete note count/detail cache
```

如果后续 hot Feed 使用 Redis ZSET，可以由 interaction event consumer 异步更新 hot score。

## Idempotency Rules

Outbox 是至少一次投递，因此消费者必须幂等。

| 场景 | 幂等 key | 规则 |
| --- | --- | --- |
| Outbox event consumer | `event.id` 或 `topic + aggregate_id + payload hash` | 已处理则跳过 |
| 后续内容分析 callback | `task_id + final status` | 重复 callback 返回当前状态，不重复改状态 |
| Like/favorite | `user_id + note_id` | 重复 like 不重复加计数 |
| Unlike/unfavorite | `user_id + note_id` | 重复 unlike 不重复减计数 |
| Notification | `recipient_id + type + source_id` | 同一源事件只创建一次通知 |
| Hot ranking update | `event.id` | 同一事件只应用一次分数变化 |

建议 P1 增加 consumer offset/processed table：

```text
processed_events
  event_id
  consumer_name
  processed_at
```

对于只做缓存删除的 consumer，可以接受重复执行，因为 `DEL key` 天然幂等。

## Error Handling

| 错误 | 处理 |
| --- | --- |
| outbox insert 失败 | 回滚业务事务 |
| RabbitMQ 连接失败 | worker 启动失败或使用 NopPublisher，根据环境配置决定 |
| publish 失败 | 标记 `failed`，增加 `retry_count`，设置 `next_retry_at` |
| publish 连续失败超过 5 次 | 标记 `abandoned`，清空 `next_retry_at`，停止自动重试 |
| mark sent 失败 | 日志告警，下一轮可能重复发布，consumer 必须幂等 |
| payload JSON 非法 | 标记 failed 或 abandoned，记录 event id |
| consumer 处理失败 | nack/retry 或记录失败表，不能吞掉错误 |

## Observability

建议记录以下日志和指标：

- pending outbox event 数量。
- failed outbox event 数量。
- publish 成功数和失败数。
- publish latency。
- retry_count 分布。
- abandoned/dead-letter 数量。
- 每个 topic 的吞吐。

P1 可以加入 admin 或命令行工具：

```text
list failed events
replay event by id
mark event abandoned
dump event payload
```

## Testing Checklist

新增或调整 Outbox 逻辑时，应至少验证：

- 发布 note 成功时，业务表和 `outbox_events` 在同一事务内提交。
- 发布 note 中途失败时，不留下 outbox event。
- RabbitMQ publisher 成功时，event 被标记为 `sent`。
- publisher 失败时，event 被标记为 `failed`，并设置 `next_retry_at`。
- publisher 连续失败超过最大重试次数时，event 被标记为 `abandoned`，且不会被 `FindPending` 再次取出。
- `next_retry_at` 未到期时，worker 不处理 failed event。
- `next_retry_at` 到期后，worker 会重试 failed event。
- mark sent 失败导致重复发布时，consumer 幂等逻辑不重复产生业务副作用。
- admin 审核决策会在同一事务内写入 `note.review_decided`。
- interaction 重复操作不会重复增加或减少计数。
- interaction 幂等请求不会重复写 outbox event。

## Interview Explanation

可以这样解释知涟 Outbox 设计：

```text
知涟没有在业务事务里直接调用 RabbitMQ，而是把业务写入和 outbox event 写入放在同一个 MySQL 事务中。
worker 轮询 pending/failed 事件，发布到 RabbitMQ topic exchange，成功后标记 sent，失败后记录 retry_count 和 next_retry_at。
这保证了业务状态和待发布事件不会分裂。由于投递语义是至少一次，所有消费者都按 event id 或业务唯一键做幂等。
RabbitMQ 不可用时不会破坏核心业务正确性，事件可以在 broker 恢复后继续投递。
```
