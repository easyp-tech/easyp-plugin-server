<!-- generated: 2026-05-15, template: bootstrap.md -->
# EasyP Service Documentation

This folder contains documentation to help LLMs and developers quickly understand the project context.

## Documentation Index

### Core
- [ARCHITECTURE.md](./ARCHITECTURE.md) — project architecture overview, layers, data flow
- [PACKAGES.md](./PACKAGES.md) — reference of all internal packages and their responsibilities
- [DOMAIN.md](./DOMAIN.md) — domain model: entities, interfaces, value objects
- [CODE_STYLE.md](./CODE_STYLE.md) — project-specific code conventions and patterns

### Development
- [TOOLS.md](./TOOLS.md) — dev environment setup, commands, CI/CD cheatsheet
- [TESTING.md](./TESTING.md) — testing conventions, patterns, mock generation

### Auth & Security
- [AUTH.md](./AUTH.md) — PASETO v4 licensing, FeatureGate, two-tier access control
- [SECURITY.md](./SECURITY.md) — input validation, Docker isolation, secrets management

### Infrastructure
- [OBSERVABILITY.md](./OBSERVABILITY.md) — OpenTelemetry, Prometheus, Pyroscope, Grafana stack
- [DATABASE.md](./DATABASE.md) — PostgreSQL access, migrations, sqlx wrapper
- [DEPLOYMENT.md](./DEPLOYMENT.md) — Docker, docker-compose, GoReleaser

### API & Clients
- [API.md](./API.md) — gRPC API contract, error mapping, interceptor chain
- [CLIENTS.md](./CLIENTS.md) — Go SDK, MCP server
- [ERRORS.md](./ERRORS.md) — domain error catalog, gRPC status mapping

### Feature Management
- [FEATURE_FLAGS.md](./FEATURE_FLAGS.md) — FeatureGate, community vs enterprise tiers
- [BACKGROUND_JOBS.md](./BACKGROUND_JOBS.md) — WorkerPool, audit worker, graceful shutdown

## Quick Facts

| Aspect | Technology |
|--------|------------|
| **Language** | Go 1.26+ |
| **Module** | `github.com/easyp-tech/service` |
| **Architecture** | Layered (Core → Adapters → API) with decorator pattern |
| **API** | gRPC (protobuf `CodeGeneratorRequest` / `CodeGeneratorResponse`) |
| **MCP** | Streamable HTTP (`/mcp`) via `go-sdk` |
| **Database** | PostgreSQL (sqlx + lib/pq) |
| **Auth** | PASETO v4.public license tokens |
| **Observability** | OpenTelemetry (OTLP) + Prometheus + Pyroscope |
| **Task Runner** | Taskfile v3 |
| **CI** | GitHub Actions + GoReleaser |
| **Linter** | golangci-lint v2 (all linters enabled, exhaustruct/wsl disabled) |
| **License** | Apache 2.0 |

## Project Structure

```
service/
├── cmd/main.go              # Service entry point (gRPC + HTTP servers)
├── api/generator/v1/        # Protobuf API contract (.proto + generated stubs + MCP bindings)
├── internal/
│   ├── core/                # Domain types, interfaces, sentinel errors, worker pool
│   ├── api/                 # gRPC handlers, audit & license interceptors
│   ├── adapters/
│   │   ├── audit/           # Async audit log writer (channel + background worker)
│   │   ├── metrics/         # Prometheus metrics adapters (business + DB pool)
│   │   └── registry/        # Plugin registry (PostgreSQL + Docker execution)
│   ├── database/            # sqlx wrapper (metrics/tracing), connectors, migrations
│   ├── grpchelper/          # gRPC server/client factories, middleware stack
│   ├── license/             # PASETO v4 manager, FeatureGate, mock client
│   ├── mcpserver/           # MCP protocol server (plugins_list, easyp_config_describe)
│   ├── ratelimiter/         # Per-IP token bucket with FeatureGate integration
│   ├── telemetry/           # OTLP + Pyroscope init, tracing decorators
│   ├── monitor/             # Context-aware slog logger
│   └── flags/               # CLI flag types (File, Level)
├── sdk/                     # Go client SDK (retry, health, filtering, interceptors)
├── migrate/                 # Numbered SQL migration files (-- up / -- down)
├── registry/                # Plugin Dockerfiles (multi-stage, scratch, non-root)
├── configs/                 # Observability stack configs (Alloy, Grafana, Loki, etc.)
├── Taskfile.yml             # Task runner commands
├── docker-compose.yml       # 14-service dev stack
├── easyp.yaml               # Protobuf lint + code generation config
├── .golangci.yml            # Linter configuration
└── .spec/                   # This documentation directory
```

## Running

```bash
# Full dev stack (14 services: postgres, registry, grafana, loki, alloy, etc.)
task up

# Minimal stack (postgres + docker registry only, port 5433)
task up-minimal

# Stop and clean volumes
task down

# Full cycle: down → up → push → logs
task run

# Run from source against minimal stack
task run-local

# Build and push required plugin images
task local-push-required

# Run tests
go test ./...

# Protobuf codegen (requires running service)
easyp --cfg easyp.yaml generate

# Lint Go code
golangci-lint run ./...
```

## Ports

| Port | Service | Protocol |
|------|---------|----------|
| 8080 (default 23410) | gRPC API | gRPC (H2) |
| 8081 (default 23411) | Metrics | HTTP (`/metrics`) |
| 8082 (default 23412) | Health | HTTP (`/health`) |
| 8083 (default 23413) | MCP | HTTP (`/mcp`) |
| 5005 | Docker Registry | HTTP (v2 API) |
| 5432/5433 | PostgreSQL | TCP |

> Ports in parentheses are defaults from config structs; docker-compose maps them to standard ports (8080–8083).

## Key Interfaces

| Interface | Package | Purpose |
|-----------|---------|---------|
| `core.Service` | `internal/core` | Business logic contract (Generate, ListPlugins, CRUD) |
| `core.Registry` | `internal/core` | Plugin storage + Docker execution |
| `core.Plugin` | `internal/core` | Single plugin: Generate + Info |
| `core.Metrics` | `internal/core` | Generation metrics collection |
| `core.FeatureGate` | `internal/core` | Feature availability checks |
| `core.AuditLog` | `internal/core` | Audit event persistence |
| `core.LicenseClient` | `internal/core` | License server communication |

## Adding New Features

1. Define domain types/interfaces in `internal/core/domain.go`
2. Implement business logic in `internal/core/core.go`
3. Add adapter implementation in `internal/adapters/`
4. Add gRPC handler in `internal/api/api.go`
5. Update proto contract in `api/generator/v1/`
6. Add tracing decorator in `internal/telemetry/tracing_*.go`
7. Add migration in `migrate/` if database changes needed
8. Update tests
