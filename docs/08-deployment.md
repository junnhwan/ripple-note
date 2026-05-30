# 知涟 Deployment

## Deployment Target

V1 deployment is Docker Compose on a single machine. The goal is reliable local and resume demo deployment, not production Kubernetes.

## Services

| Service | Port | Description |
| --- | --- | --- |
| `mysql` | `13306:3306` | Business database |
| `redis` | `6379:6379` | Cache and counters |
| `rabbitmq` | `15672:15672`, `5672:5672` | Async events |
| `api` | `8080:8080` | Gin API |
| `worker` | none | RabbitMQ consumers |
| `web` | `5173:80` | React build served by nginx |

## Environment

```text
APP_ENV=docker
HTTP_ADDR=:8080
MYSQL_DSN=ripple:password@tcp(mysql:3306)/ripple_note?parseTime=true&charset=utf8mb4&loc=Local
REDIS_ADDR=redis:6379
RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/
JWT_SECRET=change-me
RIPPLE_INTERNAL_TOKEN=change-me
UPLOAD_DIR=/data/uploads
PUBLIC_BASE_URL=http://127.0.0.1:8080
```

## Expected Commands

```bash
docker compose up -d --build
```

Health checks:

```bash
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:5173
```

Stop:

```bash
docker compose down
```

Reset data:

```bash
docker compose down -v
```

## Demo Data

Add a seed command in P0:

```bash
go run ./cmd/seed -config configs/config.local.yaml
```

The seed should create:

- One admin user.
- Several normal users.
- Published notes.
- Pending review notes.
- Rejected notes.
- Interaction counts.

## Production-Like Improvements For P1

- Add MinIO for image storage.
- Add Prometheus metrics.
- Add structured access logs.
- Add backup notes for MySQL volume.
- Add nginx reverse proxy for API and Web under one host.
