# Feed 压测报告

> 本报告只记录真实执行结果。未完成的测试项保留为空，不编造数据。

## 目标

- 验证知涟 Feed 读路径在较大数据量下的吞吐、延迟和错误率。
- 对比匿名首页缓存、登录态 Feed 回填、混合流量三类场景。
- 产出可写进简历的量化结果，并说明测试环境、数据规模和限制。

## 环境

测试时间：2026-05-31 16:40-16:51 CST

| 项目 | 配置 |
| --- | --- |
| 服务器 | baiduyun / 106.12.190.62 |
| 操作系统 | Ubuntu 24.04.4 LTS / Linux 6.8.0-107-generic |
| CPU / 内存 | 4 vCPU / 3.8 GiB |
| 磁盘 | 40 GiB 系统盘，压测后约 12 GiB 可用 |
| Docker / Compose | Docker 28.2.2 / Docker Compose 2.37.1 |
| API 端口 | `18080 -> 8080` |
| Web 端口 | `18082 -> 80` |
| 数据库 | Compose 内部 MySQL 8.4 |
| 缓存 | Compose 内部 Redis 7.4 |
| MQ | Compose 内部 RabbitMQ 4 |
| 压测工具 | `grafana/k6` Docker 镜像 |

## 部署拓扑

```text
k6 / browser
  |
  | :18080
  v
ripple-note-api  --->  mysql
      |           --->  redis
      |           --->  rabbitmq
      |
outbox-worker

browser
  |
  | :18082
  v
nginx web  --->  api:8080
```

## 数据集

| 类型 | 数量 |
| --- | ---: |
| 用户 | 5,000 |
| 笔记 | 50,000 |
| 标签 | 100 |
| 图片 | 16,667 |
| 点赞 | 300,000 |
| 收藏 | 100,000 |
| 评论 | 100,000 |
| 关注 | 100,000 |

固定压测账号：

```text
loadtest@ripple.dev / loadtest123
```

## 执行命令

### 一键 k6 入口

仓库提供 PowerShell 封装脚本，适合在 Windows 本机或服务器上统一运行 k6 Docker 镜像并保存 summary：

```powershell
.\scripts\loadtest\run-k6.ps1 `
  -Scenario latest-anonymous `
  -BaseUrl http://127.0.0.1:18080 `
  -Vus 100 `
  -Duration 2m `
  -Sleep 0
```

支持的场景：

| Scenario | 脚本 | 说明 |
| --- | --- | --- |
| `latest-anonymous` | `feed_latest_anonymous.js` | 匿名最新 Feed |
| `hot-anonymous` | `feed_hot_anonymous.js` | 匿名热门 Feed |
| `latest-auth` | `feed_latest_auth.js` | 登录态最新 Feed，验证 viewer flags |
| `mixed` | `feed_mixed.js` | latest/hot/following 混合流量 |

结果默认写入：

```text
reports/loadtest/
```

### 手动部署和造数

部署：

```bash
docker compose -f docker-compose.deploy.yml -p ripple-note up -d --build
docker compose -f docker-compose.deploy.yml -p ripple-note ps
curl http://127.0.0.1:18080/health
```

造数：

```bash
docker compose -f docker-compose.deploy.yml -p ripple-note run --rm loadseed \
  -config /app/configs/config.deploy.yaml \
  -clean \
  -users 5000 \
  -notes 50000 \
  -likes 300000 \
  -favorites 100000 \
  -comments 100000 \
  -follows 100000 \
  -tags 100 \
  -batch-size 1000
```

匿名 Feed：

```bash
docker run --rm --network host \
  -e BASE_URL=http://127.0.0.1:18080 \
  -e VUS=100 \
  -e DURATION=2m \
  -e SLEEP=0 \
  -v /root/ripple-note/scripts/loadtest:/scripts \
  grafana/k6 run --quiet \
    --summary-trend-stats "avg,min,med,p(90),p(95),p(99),max" \
    /scripts/feed_latest_anonymous.js
```

匿名 hot Feed：

```bash
docker run --rm --network host \
  -e BASE_URL=http://127.0.0.1:18080 \
  -e VUS=100 \
  -e DURATION=2m \
  -e SLEEP=0 \
  -v /root/ripple-note/scripts/loadtest:/scripts \
  grafana/k6 run --quiet \
    --summary-trend-stats "avg,min,med,p(90),p(95),p(99),max" \
    /scripts/feed_hot_anonymous.js
```

登录态 Feed：

```bash
docker run --rm --network host \
  -e BASE_URL=http://127.0.0.1:18080 \
  -e VUS=50 \
  -e DURATION=2m \
  -e SLEEP=0 \
  -e LOGIN_EMAIL=loadtest@ripple.dev \
  -e LOGIN_PASSWORD=loadtest123 \
  -v /root/ripple-note/scripts/loadtest:/scripts \
  grafana/k6 run --quiet \
    --summary-trend-stats "avg,min,med,p(90),p(95),p(99),max" \
    /scripts/feed_latest_auth.js
```

混合流量：

```bash
docker run --rm --network host \
  -e BASE_URL=http://127.0.0.1:18080 \
  -e VUS=100 \
  -e DURATION=2m \
  -e SLEEP=0 \
  -v /root/ripple-note/scripts/loadtest:/scripts \
  grafana/k6 run --quiet \
    --summary-trend-stats "avg,min,med,p(90),p(95),p(99),max" \
    /scripts/feed_mixed.js
```

## 结果

| 场景 | VUs | 时长 | RPS | 平均延迟 | P95 | P99 | 错误率 | 备注 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| 匿名 latest Feed | 100 | 2m | 2683.62 | 35.34ms | 81.25ms | 111.61ms | 0.00% | Redis 首页缓存，`SLEEP=0` |
| 登录态 latest Feed 优化前 | 50 | 2m | 107.58 | 463.33ms | 614.91ms | 685.61ms | 0.00% | N+1 回填，baseline commit `48bca04`，`SLEEP=0` |
| 登录态 latest Feed | 50 | 2m | 676.49 | 72.71ms | 106.90ms | 126.44ms | 0.00% | 批量回填作者/标签/图片/互动状态，`SLEEP=0` |
| 混合 Feed | 100 | 2m | 1293.32 | 75.47ms | 232.03ms | 291.44ms | 0.00% | latest/hot/following 混合，`SLEEP=0` |
| 匿名 hot Feed | - | - | - | - | - | - | - | 已补独立 k6 脚本，待下次同环境补跑 |

说明：2026-06-01 本地复核时，Windows 机器可用 Docker CLI，但 Docker daemon 未启动，Docker 版 k6 未能执行；因此本次没有新增本机压测数据。上表只保留 2026-05-31 在云服务器上真实执行过的结果，未补跑的 hot 单场景不填数字。

## 前后对比

登录态 latest Feed 使用相同数据集、相同 k6 脚本、相同 50 VUs / 2m / `SLEEP=0` 参数，对比优化前 N+1 回填版本和优化后批量回填版本。

| 指标 | 优化前 | 优化后 | 变化 |
| --- | ---: | ---: | ---: |
| SQL 查询数 / 页 | 约 121 次 | 约 7 次 | 减少约 94.2% |
| RPS | 107.58 | 676.49 | 提升约 6.29 倍 |
| 平均延迟 | 463.33ms | 72.71ms | 降低约 84.3% |
| P95 | 614.91ms | 106.90ms | 降低约 82.6% |
| P99 | 685.61ms | 126.44ms | 降低约 81.6% |
| 错误率 | 0.00% | 0.00% | 持平 |

原始结果文件保存在服务器：

```text
/root/ripple-note/reports/loadtest/feed_latest_anonymous_100vu_2m.txt
/root/ripple-note/reports/loadtest/feed_latest_auth_baseline_50vu_2m.txt
/root/ripple-note/reports/loadtest/feed_latest_auth_50vu_2m.txt
/root/ripple-note/reports/loadtest/feed_mixed_100vu_2m.txt
```

后续新增结果建议按以下命名保存：

```text
reports/loadtest/<scenario>_<vus>vu_<duration>_<timestamp>.txt
```

## 已做优化

- Feed 使用游标分页，避免 `OFFSET` 在深分页和数据变更下的性能与一致性问题。
- 为 `latest`、`hot`、`following`、标签 Feed 增加复合索引，匹配筛选条件和排序字段。
- Feed 列表回填采用批量查询：作者、标签、图片、点赞状态、收藏状态、关注状态按 ID 集合一次拉取，避免每页 N+1 查询。
- Redis 缓存匿名 `latest/hot` 首页，适合未登录高频访问场景；登录态 Feed 保留实时用户状态。
- 通过 `loadseed` 构造万级用户、笔记和互动数据，用接近真实业务的读路径做压测。

## 限制

- 当前压测是单机 Docker Compose 部署，不代表多节点生产集群上限。
- k6 与服务部署在同一台机器时会共享 CPU、内存和网络栈，结果更适合做项目量化和优化对比，不应宣称为生产 SLA。
- 若从公网压测，需要确认云安全组已开放 `18080`，并区分公网网络延迟与服务端处理延迟。
- 本次压测使用 `SLEEP=0`，表示每个虚拟用户完成一次请求后立即发起下一次请求，偏向容量压测，不代表普通用户真实停留时间。
- 本地复跑需要 Docker daemon 正常运行，或者直接安装 k6 CLI。

## 简历表述草稿

```text
负责知涟 Feed 读链路性能优化，基于游标分页、MySQL 复合索引、Redis 首页缓存和批量回填策略，将登录态 Feed 单页 SQL 查询数由约 121 次降至 7 次；在 5 万笔记、60 万互动数据集下，登录态 Feed RPS 由 107 提升至 676、P95 由 615ms 降至 107ms，匿名 Feed 达到 2683 RPS、P95 81ms，错误率均为 0%。
```
