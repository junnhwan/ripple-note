# 知涟 Learning Log

## 2026-05-30 Stage 1: Backend Skeleton And API Contract

### Stage Goal

建立 Go 后端项目骨架，让服务可以从配置文件启动，并提供带统一响应格式和 request ID 的 `GET /health` 接口。此阶段不实现账号、内容发布或 Feed 业务，只铺好后续模块复用的基础设施。

### Files Created

- `go.mod`, `go.sum`: Go module and dependency lock files.
- `docs/openapi.yaml`: Stage 1 API contract for unified response and `/health`.
- `configs/config.local.yaml`: Local runtime configuration.
- `cmd/server/main.go`: HTTP server entrypoint with graceful shutdown.
- `internal/config`: YAML config loading, defaults, validation.
- `internal/http`: Gin router and unified JSON response helpers.
- `internal/middleware`: request ID and access logging middleware.
- `internal/observability`: structured logger setup.
- `internal/storage`: optional GORM MySQL connection setup.

### Go Backend Notes

- `cmd/server` is the executable entrypoint; reusable code stays under `internal`.
- `internal` packages cannot be imported from outside this module, which helps protect service boundaries.
- Gin middleware runs in registration order. Stage 1 uses recovery, request ID, then access logging.
- Unified response makes later handlers return a consistent shape:

```json
{
  "data": {},
  "error": null,
  "request_id": "req_..."
}
```

- `slog` is Go's standard structured logging package. It avoids committing to a third-party logging framework early.
- GORM/MySQL connection exists now, but `configs/config.local.yaml` keeps `mysql.enabled: false` so `/health` can run before Docker Compose and schema work are ready.

### Java Spring Boot Comparison

- `cmd/server/main.go` is close to `public static void main` plus Spring Boot application startup.
- `internal/http/router.go` manually wires routes, similar to what Spring MVC does through annotations and auto-configuration.
- Gin middleware is conceptually similar to Servlet filters or Spring interceptors.
- `internal/config` replaces part of Spring Boot's `application.yml` binding, but the binding/default/validation logic is explicit Go code.
- Go does not provide dependency injection by default. Stage 1 wires dependencies manually to keep the project easy to read.

### Verification

- `go test ./internal/http -run TestHealth -v`
- `go test ./...`
- `go run ./cmd/server -config configs/config.local.yaml`
- `GET http://127.0.0.1:8080/health` returned:

```json
{
  "data": {
    "status": "ok"
  },
  "error": null,
  "request_id": "req_verify_stage1"
}
```

### Next Stage

Stage 2 should add account and authentication:

- `users` model and migration.
- Register/login/current-user APIs.
- Password hashing.
- JWT signing and auth middleware.
