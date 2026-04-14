<!-- generated: 2026-04-14, template: bootstrap.md -->
# EasyP Service Documentation

This folder contains documentation to help LLMs and developers quickly understand the project context.

## Documentation Index

### Core
- [ARCHITECTURE.md](./ARCHITECTURE.md) — Application architecture, layers, request flow, design decisions
- [PACKAGES.md](./PACKAGES.md) — Reference of all packages with file descriptions
- [DOMAIN.md](./DOMAIN.md) — Domain model: entities, enums, interfaces
- [CODE_STYLE.md](./CODE_STYLE.md) — Project-specific code conventions

### Development
- [TOOLS.md](./TOOLS.md) — Build commands, dev environment setup, code generation
- [TESTING.md](./TESTING.md) — Testing conventions, patterns, mock generation

### Errors
- [ERRORS.md](./ERRORS.md) — Error architecture, business error catalog, wrapping conventions

### Auth & Licensing
- [AUTH.md](./AUTH.md) — PASETO v4 licensing system, feature gating, claims

### Data
- [DATABASE.md](./DATABASE.md) — PostgreSQL schema, migrations, connection management

### API
- [API.md](./API.md) — gRPC API endpoints, middleware stack, error format

### Infrastructure
- [DEPLOYMENT.md](./DEPLOYMENT.md) — Docker, docker-compose, CI/CD, health checks
- [OBSERVABILITY.md](./OBSERVABILITY.md) — OpenTelemetry, Prometheus, Grafana, tracing, profiling
- [SECURITY.md](./SECURITY.md) — Security model, input validation, OWASP mapping

### Feature Management
- [FEATURE_FLAGS.md](./FEATURE_FLAGS.md) — License-based feature gate system

### Background Processing
- [BACKGROUND_JOBS.md](./BACKGROUND_JOBS.md) — Worker pool, audit worker, async processing

### Clients
- [CLIENTS.md](./CLIENTS.md) — Go SDK, MCP protocol client

## Quick Facts

| Aspect | Technology |
|--------|------------|
| **Language** | Go 1.26+ |
| **Module** | `github.com/easyp-tech/service` |
| **Architecture** | Layered (API → Core → Adapters) with Decorator pattern |
| **API** | gRPC (protobuf), MCP (HTTP/JSON) |
| **Database** | PostgreSQL via `sqlx` |
| **Licensing** | PASETO v4.public tokens (Community / Enterprise tiers) |
| **Observability** | OpenTelemetry (traces + metrics), Prometheus, Pyroscope, Grafana |
| **Container Runtime** | Docker (plugin execution in isolated containers) |
| **Build Tool** | Taskfile (go-task) |
| **Config** | YAML file + environment variables (`go-envconfig`) |

## Project Structure

```
service/
├── api/generator/v1/       # Protobuf API contract + generated stubs + MCP bindings
├── cmd/
│   ├── main.go             # Service entry point
│   └── mcp-smoke/          # MCP smoke test client
├── internal/
│   ├── core/               # Domain types, interfaces, business logic, worker pool
│   ├── api/                # gRPC handlers, audit & license interceptors, MCP tools
│   ├── adapters/           # audit/, metrics/, registry/ — implement core interfaces
│   ├── database/           # sqlx wrapper (metrics/tracing), connectors, migrations
│   ├── grpchelper/         # gRPC server/client factories, middleware chain
│   ├── license/            # PASETO v4 management, FeatureGate, claims
│   ├── ratelimiter/        # Per-IP token bucket with FeatureGate
│   ├── telemetry/          # OTLP + Pyroscope, tracing decorators
│   ├── monitor/            # Context-aware slog logger
│   └── flags/              # CLI flag types
├── sdk/                    # Go client SDK (retry, health, filtering, interceptors)
├── migrate/                # Numbered SQL migration files (up / down)
├── registry/               # Plugin Dockerfiles (multi-stage, scratch, non-root)
└── configs/                # Observability stack configs (Alloy, Grafana, Loki, etc.)
```

## Running

```bash
task up                      # Full 14-service dev stack
task up-minimal              # Postgres + registry only (port 5433)
task down                    # Stop and clean volumes
task run                     # Full cycle: down → up → push → logs
task run-local               # go run from source against minimal stack
task local-push-required     # Build + push required plugin images
go test ./...                # Standard tests
easyp --cfg easyp.yaml generate  # Protobuf codegen (requires running service)
```

## Ports

| Port | Service | Protocol |
|------|---------|----------|
| 8080 (docker) / 23410 (local) | gRPC API | gRPC (H2) |
| 8081 / 23411 | Metrics | HTTP (`/metrics`) |
| 8082 / 23412 | Health | HTTP (`/health`) |
| 8083 / 23413 | MCP | HTTP (`/mcp`) |
| 5005 | Docker Registry | HTTP (v2 API) |
| 5432 / 5433 | PostgreSQL | TCP |
| 3000 | Grafana | HTTP |

## Key Interfaces

- `core.Registry` — Plugin storage and Docker execution
- `core.Plugin` — Code generation interface
- `core.CoreService` — Business logic facade (Generate, ListPlugins, CreatePlugin, UpdatePlugin, DeletePlugin)
- `core.Metrics` — Metrics collection
- `core.AuditLog` — Audit event persistence
- `core.FeatureGate` — License-based feature checks

## Adding New Features

1. Define domain types in `internal/core/domain.go`
2. Add business logic in `internal/core/core.go`
3. Add gRPC method to `api/generator/v1/generator.proto`
4. Generate stubs: `easyp --cfg easyp.yaml generate`
5. Implement handler in `internal/api/api.go`
6. Add error mapping in `api.ErrorToStatus()`
7. Add adapter implementation in `internal/adapters/`
8. Wire in `cmd/main.go`
9. Add migration in `migrate/` if schema changes needed
