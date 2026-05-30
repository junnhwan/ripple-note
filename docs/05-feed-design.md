# 知涟 Feed Design

## Feed Goals

- Stable cursor pagination without duplicate or missing items during normal refresh.
- Only published and public notes appear in public feeds.
- Login state can add viewer-specific flags such as liked, favorited, and following.
- V1 ranking is explainable and implementable before complex recommendation.

## Feed Scenes

| Scene | Path | Sort | Storage |
| --- | --- | --- | --- |
| Latest | `/api/feed/latest` | `published_at desc, id desc` | MySQL + Redis cache |
| Following | `/api/feed/following` | `published_at desc, id desc` | MySQL in V1 |
| Hot | `/api/feed/hot` | `hot_score desc, id desc` | MySQL baseline, Redis ZSET in P1 |
| Tag | `/api/tags/{tagName}/feed` | `published_at desc, id desc` | MySQL join |

## Cursor Strategy

Use opaque base64 JSON cursors.

Latest cursor:

```json
{
  "published_at": "2026-05-30T10:00:00Z",
  "id": 12345
}
```

Query:

```sql
WHERE status = 'published'
  AND visibility = 'public'
  AND (
    published_at < ?
    OR (published_at = ? AND id < ?)
  )
ORDER BY published_at DESC, id DESC
LIMIT ?
```

Hot cursor:

```json
{
  "hot_score": 983.2,
  "id": 12345
}
```

Query:

```sql
WHERE status = 'published'
  AND visibility = 'public'
  AND (
    hot_score < ?
    OR (hot_score = ? AND id < ?)
  )
ORDER BY hot_score DESC, id DESC
LIMIT ?
```

## Hydration

Feed query first returns note IDs and ranking fields. Hydration loads:

- Note title, body preview, status, published time.
- First images.
- Author profile.
- Counts.
- Viewer flags: liked, favorited, following.

Hydration should keep frontend DTO stable:

```json
{
  "id": 1,
  "title": "note",
  "body_preview": "preview",
  "images": [],
  "author": {},
  "stats": {},
  "viewer_state": {}
}
```

## Redis Cache Plan

V1 cache should be conservative.

| Key | TTL | Purpose |
| --- | --- | --- |
| `feed:latest:first-page` | 10s | Anonymous latest first page |
| `note:detail:{id}` | 5m | Approved note detail |
| `note:counts:{id}` | 5m | Like/favorite/comment counters |
| `user:profile:{id}` | 10m | Public profile |

Do not cache personalized following feed in V1 unless needed.

## Hot Score

V1 hot score can be simple:

```text
hot_score = likes_count * 3 + favorites_count * 4 + comments_count * 5 + freshness_score
```

`freshness_score` decays over time. Keep the formula documented and deterministic.

P1 can move hot feed to Redis ZSET:

```text
feed:hot:zset
```

Workers update score on interaction events.

## Consistency Rules

- Creating a note does not enter feed until approved.
- Rejecting or removing a note must invalidate `note:detail:{id}` and first-page feed cache.
- Like/favorite/comment updates can use DB transaction plus async count reconciliation.
- Admin removal takes priority over agent pass.

## Tests To Add

- Latest feed cursor does not duplicate records.
- Latest feed skips pending and rejected notes.
- Hot feed cursor handles equal `hot_score`.
- Hydration returns viewer flags only for logged-in users.
- Cache invalidation happens after status changes.
