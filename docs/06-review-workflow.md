# 知涟 Review Workflow

## Purpose

知涟 must keep content visibility under platform control while allowing `ripple-guard-agent` (`知涟洞察`) to automate content quality/risk analysis.

The platform owns final note status. The Agent submits evidence and a recommendation.

## Note Status

| Status | Meaning |
| --- | --- |
| `draft` | User saved but not submitted |
| `pending_review` | Submitted and waiting for review |
| `published` | Visible in public feed |
| `rejected` | Not visible, user can see reason |
| `removed` | Removed by author or admin and hidden from public reads |

## Review Task Status

| Status | Meaning |
| --- | --- |
| `pending_agent` | Waiting for Agent review |
| `agent_passed` | Agent recommended pass |
| `agent_rejected` | Agent recommended reject |
| `manual_required` | Agent requested human review |
| `admin_approved` | Admin approved |
| `admin_rejected` | Admin rejected |
| `admin_removed` | Admin removed or took down the content |

## V1 Decision Policy

For demo simplicity:

- Agent `pass` publishes the note automatically.
- Agent `reject` rejects the note automatically.
- Agent `manual_review` leaves the note pending for admin.
- Admin can override any non-final or final Agent decision.
- Admin `remove` can also take down content after a previous admin approval; a repeated `remove` is treated as already decided.

This creates a clear automated demo. If stricter governance is needed later, Agent decisions can become recommendations only.

## State Transitions

```text
User creates note
  -> notes.status = pending_review
  -> review_tasks.status = pending_agent

Agent pass
  -> review_tasks.status = agent_passed
  -> notes.status = published
  -> notes.published_at = now

Agent reject
  -> review_tasks.status = agent_rejected
  -> notes.status = rejected

Agent manual_review
  -> review_tasks.status = manual_required
  -> notes.status = pending_review

Admin approve
  -> review_tasks.status = admin_approved
  -> notes.status = published

Admin reject
  -> review_tasks.status = admin_rejected
  -> notes.status = rejected

Admin remove
  -> review_tasks.status = admin_removed
  -> notes.status = removed
```

## Integration Contract

`知涟洞察` calls:

```text
GET /internal/review/tasks/pending
GET /internal/notes/{noteId}/review-context
PUT /internal/review/tasks/{taskId}/agent-result
```

Internal token is configured on both services:

```text
RIPPLE_INTERNAL_TOKEN=...
```

## Review Context Payload

知涟 should return enough evidence for the Agent:

```json
{
  "task": {
    "id": 1001,
    "source": "publish"
  },
  "note": {
    "id": 2001,
    "title": "title",
    "body": "body",
    "images": []
  },
  "author": {
    "id": 3001,
    "nickname": "author",
    "created_at": "2026-05-30T10:00:00Z",
    "published_notes_count": 12,
    "rejected_notes_count": 1
  },
  "recent_notes": []
}
```

## Agent Result Payload

```json
{
  "decision": "pass",
  "risk_level": "low",
  "categories": [],
  "reason": "No visible policy risk.",
  "evidence": [],
  "confidence": 0.91,
  "trace_id": "rg_trace_123"
}
```

## Failure Handling

- If Agent callback is invalid, keep task `pending_agent`.
- If Agent times out, admin can manually review.
- If 知涟洞察 is down, 知涟 still works; notes remain pending.
- If duplicate callbacks arrive, the API should be idempotent by `task_id` and final state.

## Admin UI Requirements

The admin review page should show:

- Note title, body, images.
- Author profile and previous rejection count.
- Agent decision, risk level, categories, confidence.
- Evidence list and trace ID.
- Approve, reject, remove actions.
