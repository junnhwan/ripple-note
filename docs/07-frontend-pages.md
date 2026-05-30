# 知涟 Frontend Pages

## Frontend Stack

- React
- Vite
- TypeScript
- TailwindCSS
- React Router
- TanStack Query or small local API hooks

The UI should be practical and demo-oriented. Avoid marketing pages. The first screen should be the feed.

## Pages

### Feed Page

Path: `/`

Purpose:

- Show latest approved notes.
- Support infinite scroll or "load more".
- Show author, images, title, preview, counts.
- Logged-in users can like, favorite, and follow.

API:

- `GET /api/feed/latest`
- `PUT /api/notes/{noteId}/like`
- `PUT /api/notes/{noteId}/favorite`

### Login And Register

Paths:

- `/login`
- `/register`

API:

- `POST /api/sessions`
- `POST /api/users`

### Publish Note

Path: `/publish`

Purpose:

- Upload images.
- Enter title, body, tags.
- Submit note into review.
- Show pending review result after publish.

API:

- `POST /api/uploads/images`
- `POST /api/notes`

### Note Detail

Path: `/notes/:noteId`

Purpose:

- Show full note content.
- Show images, author, tags, comments.
- Support comment creation.

API:

- `GET /api/notes/{noteId}`
- `GET /api/notes/{noteId}/comments`
- `POST /api/notes/{noteId}/comments`

### Profile

Paths:

- `/me`
- `/users/:userId`

Purpose:

- Show profile and public notes.
- `me` page also shows pending/rejected own notes.

API:

- `GET /api/users/me`
- `GET /api/users/{userId}`
- `GET /api/users/{userId}/notes`
- `GET /api/users/me/notes`

### Admin Review

Path: `/admin/review`

Purpose:

- List review tasks.
- Open task detail.
- Show Agent result and evidence.
- Approve or reject manually.

API:

- `GET /api/admin/review/tasks`
- `GET /api/admin/review/tasks/{taskId}`
- `PUT /api/admin/review/tasks/{taskId}/decision`

## UI States

Every main page needs:

- Loading state.
- Empty state.
- Error state.
- Auth-required state when relevant.

## Demo Flow

1. Register user.
2. Publish a normal note.
3. Note appears as pending.
4. Agent approves it.
5. Feed shows the note.
6. Publish a risky note.
7. Agent rejects or marks manual review.
8. Admin review page shows decision evidence.
