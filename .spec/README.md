<!-- generated: 2026-04-03, template: bootstrap.md -->
# EasyP Service Documentation

This folder contains documentation optimized for AI agent (LLM) consumption. For human-readable docs see [`docs/`](../docs/).

## Documentation Index

### Core
- [ARCHITECTURE.md](./ARCHITECTURE.md) — layered architecture, interceptor chain, data flow diagrams
- [PACKAGES.md](./PACKAGES.md) — all packages grouped by layer with file tables
- [DOMAIN.md](./DOMAIN.md) — domain types, interfaces, plugin naming, version resolution
- [CODE_STYLE.md](./CODE_STYLE.md) — conventions per layer, naming, error propagation, imports

### Development
- [TOOLS.md](./TOOLS.md) — dev setup, task commands, code generation, CI/CD
- [TESTING.md](./TESTING.md) — test patterns, mocking strategy, integration tests
- [ERRORS.md](./ERRORS.md) — error architecture, sentinel error catalog, gRPC mapping, retry policy

### Auth & Licensing
- [AUTH.md](./AUTH.md) — PASETO v4 license tokens, two-tier model, FeatureGate

### Infrastructure
- [DATABASE.md](./DATABASE.md) — PostgreSQL schema, migrations, connection pool, query patterns
- [API.md](./API.md) — gRPC endpoint reference, interceptor stack, MCP tools
- [DEPLOYMENT.md](./DEPLOYMENT.md) — Docker, CI/CD pipelines, environments, health checks
- [OBSERVABILITY.md](./OBSERVABILITY.md) — Grafana stack (Alloy, Mimir, Loki, Tempo), Pyroscope, tracing decorators

### Clients
- [CLIENTS.md](./CLIENTS.md) — Go SDK: retry, health monitoring, filtering, interceptors

### Security & Features
- [SECURITY.md](./SECURITY.md) — trust boundaries, Docker sandboxing, input validation, OWASP mapping
- [FEATURE_FLAGS.md](./FEATURE_FLAGS.md) — FeatureGate: 8 features, two tiers, resource limits
- [BACKGROUND_JOBS.md](./BACKGROUND_JOBS.md) — WorkerPool (Docker execution) and Audit Worker

### Agent Rules
- [agent-rules.md](./agent-rules.md) — mandatory rules for AI agents working on this project

## Quick Facts

| Aspect | Technology |
|--------|------------|
| **Language** | Go 1.26+ |
| **Architecture** | Layered (Transport → Application → Adapters → Infrastructure) |
| **API** | gRPC (protobuf), MCP (Streamable HTTP) |
| **Database** | PostgreSQL 17.7 (sqlx + lib/pq) |
| **Auth** | PASETO v4.public license tokens (Community / Enterprise) |
| **Observability** | OpenTelemetry → Alloy → Mimir + Loki + Tempo, Pyroscope |
| **Container Runtime** | Docker (plugin execution in sandboxed containers) |
| **Build** | Task (Taskfile.yml), goreleaser for releases |
| **License** | Apache 2.0 |

## Project Structure

```
cmd/main.go              # Service entry point
internal/
  core/                  # Domain types, interfaces, sentinel errors, worker pool
  api/                   # gRPC handlers, audit & license interceptors
  adapters/              # audit/ metrics/ registry/ — implement core interfaces
  database/              # sqlx wrapper (metrics/tracing), connectors, migrations
  grpchelper/            # gRPC server/client factories, middleware
  license/               # PASETO v4 management, FeatureGate, claims
  mcpserver/             # MCP protocol server
  ratelimiter/           # Per-IP token bucket with FeatureGate
  telemetry/             # OTLP + Pyroscope, tracing decorators
  monitor/               # Context-aware slog logger
  flags/                 # CLI flag types
sdk/                     # Go client SDK
api/generator/v1/        # Protobuf contract + generated stubs
migrate/                 # Numbered SQL migration files
registry/                # Plugin Dockerfiles
configs/                 # Observability stack configs
```

## Running

```bash
task up                  # Full 14-service dev stack
task up-minimal          # Postgres + registry only (port 5433)
task down                # Stop and clean volumes
task run                 # Full cycle: down → up → push → logs
task run-local           # go run from source against minimal stack
task local-push-required # Build + push required plugin images
go test ./...            # Standard tests
easyp --cfg easyp.yaml generate  # Protobuf codegen (requires running service)
```

## Ports

| Port | Service | Protocol |
|------|---------|----------|
| 8080 | gRPC API | gRPC (H2) |
| 8081 | Metrics | HTTP (`/metrics`) |
| 8082 | Health | HTTP (`/health`) |
| 8083 | MCP | HTTP (`/mcp`) |
| 5005 | Docker Registry | HTTP (v2 API) |
| 5432/5433 | PostgreSQL | TCP |
| 3000 | Grafana | HTTP |
| 4317 | Alloy (OTLP) | gRPC |

## Key Interfaces

Defined in `internal/core/domain.go`:

- `Registry` — plugin catalog + Docker execution (`Get`, `List`)
- `Plugin` — code generator plugin (`Generate`, `Info`)
- `Metrics` — business metrics collection (`GenerateCode`, `ObserveGenerationDuration`, `IncGenerationErrors`)
- `AuditLog` — audit event storage (`Save`)
- `FeatureGate` — license feature checks (`Enabled`, `MaxWorkers`, `MaxPlugins`)
- `CoreService` — business logic facade (`Generate`, `ListPlugins`)

## Adding a New Feature

Use the **spec-driven-dev** skill for structured feature development:

1. Tell the agent: "I want to add feature X"
2. Pipeline: Explore → Requirements → Design → Implementation Plan
3. Each phase requires explicit approval before advancing
4. See `.agents/skills/spec-driven-dev/SKILL.md` for details
