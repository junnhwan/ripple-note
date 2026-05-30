# 知涟 Product Scope

## Positioning

知涟 is a content community and Feed distribution platform for learning Go backend engineering through a realistic product. The repository name is `ripple-note`. It focuses on content publishing, feed distribution, social interactions, review/governance workflow, and deployable full-stack demos.

The project should feel like a compact content community with Xiaohongshu/Zhihu-style discovery and interaction, not a short-video platform. The core backend value is content lifecycle, feed pagination, cache design, async events, and quality-analysis integration with `ripple-guard-agent` (`知涟洞察`).

## Goals

- Build a complete Go backend service with user, note, feed, interaction, review, and admin workflows.
- Build a React frontend that can demonstrate the main product loop end to end.
- Expose stable integration APIs for `ripple-guard-agent` (`知涟洞察`).
- Provide Docker Compose deployment for demo and resume presentation.
- Keep the first version focused enough to finish and polish.

## Non-Goals For V1

- No short-video upload, transcoding, streaming, or playback optimization.
- No complex recommendation model or machine learning ranking service.
- No full-text search engine in V1.
- No private messaging in V1.
- No payment, creator monetization, or enterprise moderation console.
- No Kubernetes deployment in V1.

## Target Users

- Anonymous visitor: browse public approved notes.
- Registered user: publish image-and-text notes, like, favorite, comment, follow authors.
- Admin reviewer: inspect review tasks, make manual decisions, remove violating content.
- Agent service: pull pending review tasks and submit automated review results.

## V1 User Stories

- As a user, I can register, log in, and update my profile.
- As a user, I can publish a note with title, body, tags, and images.
- As a user, I can see my note enter `pending_review` after publishing.
- As a visitor, I can browse approved notes in the latest feed.
- As a logged-in user, I can like, favorite, comment, and follow authors.
- As an admin, I can view pending and risky review tasks.
- As an admin, I can approve, reject, or remove content.
- As `知涟洞察`, I can fetch pending tasks and submit content quality/risk decisions.

## P0 Scope

P0 must be demoable through the browser and Docker Compose.

| Area | Capability |
| --- | --- |
| Account | Register, login, logout, current user, public profile |
| Content | Create note, upload images, get note detail, list author notes |
| Feed | Latest feed, following feed, hot feed baseline |
| Interaction | Like, favorite, comment, follow |
| Review | Pending review state, review task table, admin decision, agent decision callback |
| Frontend | Login, feed, publish, note detail, profile, admin review |
| Deployment | API, Web, MySQL, Redis, RabbitMQ through Docker Compose |

## P1 Scope

- Tag feed and tag pages.
- Notification list for review result, likes, comments, and follows.
- Hot feed Redis ranking optimization.
- Outbox event replay and dead-letter retry.
- Prometheus metrics and pprof.
- Basic admin dashboard for content statistics.

## Product Boundary With 知涟洞察

知涟 owns:

- User and note data.
- Review task creation and state transitions.
- Final content visibility.
- Admin override decisions.
- Public and internal integration APIs.

知涟洞察 owns:

- Automated review orchestration.
- LLM and tool calling logic.
- Evidence validation.
- Review decision generation.
- Agent traces and evaluation reports.

Integration is HTTP in V1. RabbitMQ subscription can be added later after both services are stable.
