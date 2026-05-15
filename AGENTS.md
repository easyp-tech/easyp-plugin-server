# EasyP API Service

Centralized protobuf/gRPC plugin execution service. Accepts `CodeGeneratorRequest` via gRPC, runs plugins in isolated Docker containers, returns `CodeGeneratorResponse`.

**Module:** `github.com/easyp-tech/service` | **Go:** 1.26+ | **License:** Apache 2.0

## Architecture

See [docs/architecture.md](docs/architecture.md) for full request flow and component diagrams.

**Interceptor chain (order matters):** trace_logging → realip → prometheus → structured_logging → panic_recovery → validation → error_code_conversion → rate_limit → license → audit

| Pattern | Where | Purpose |
|---------|-------|---------|
| Decorator (tracing) | `telemetry.TracingCore`, `TracingRegistry`, `TracingPlugin` | Non-invasive OTel instrumentation |
| Worker Pool | `core.WorkerPool` | Bounded concurrency for Docker plugin execution |
| Feature Gate | `license.FeatureGate` | Two-tier licensing without hard coupling |
| Interceptor chain | `grpchelper.NewServer` + custom interceptors | Composable cross-cutting concerns |
| Repository pattern | `core.Registry` interface → `adapters/registry` | DB + Docker behind clean interface |
| Functional options | SDK `WithXxx()` options | Configurable client construction |

## Project Map

```
api/generator/v1/       # Protobuf API contract (.proto + generated stubs + MCP bindings)
cmd/main.go             # Service entry point
internal/
  core/                 # Domain types, interfaces, sentinel errors, worker pool
  api/                  # gRPC handlers, audit & license interceptors
  adapters/             # audit/ metrics/ registry/ — implement core interfaces
  database/             # sqlx wrapper (metrics/tracing), connectors, migrations
  grpchelper/           # gRPC server/client factories, middleware
  license/              # PASETO v4 management, FeatureGate, claims
  mcpserver/            # MCP protocol server (plugins_list, easyp_config_describe)
  ratelimiter/          # Per-IP token bucket with FeatureGate
  telemetry/            # OTLP + Pyroscope, tracing decorators
  monitor/              # Context-aware slog logger
  flags/                # CLI flag types
sdk/                    # Go client SDK (retry, health, filtering, interceptors)
migrate/                # Numbered SQL migration files (-- up / -- down)
registry/               # Plugin Dockerfiles (multi-stage, scratch, non-root)
configs/                # Observability stack configs (Alloy, Grafana, Loki, etc.)
```

## Build & Test

See [docs/development.md](docs/development.md) for full setup.

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

## Conventions

- **Domain types and interfaces** live in `internal/core/domain.go` — single source of truth
- **Business logic** in `internal/core/core.go` — thin, delegates to Registry
- **Tracing** via decorator pattern (`telemetry/tracing_*.go`), never mixed into business logic
- **Adapters** implement core interfaces; placed in `internal/adapters/`
- **Database access** always through `database.SQL` wrapper, never raw `sqlx.DB`
- **Domain errors** are sentinel: `core.ErrNotFound`, `core.ErrInvalidPluginName`, etc.
- `api.ErrorToStatus()` maps domain errors → gRPC status codes
- **Plugin name format:** `{group}/{name}:{version}` — validated by `^[a-z][a-z0-9-]*/[a-z][a-z0-9-]*:(v\d+\.\d+\.\d+|latest)$`
- **Plugin Docker images:** multi-stage (`golang:alpine` → `scratch`), static binary + `upx`, run as `nobody`, `--network=none --memory=128m --cpus=1.0`
- **Testing:** standard `go test`, no external framework; mocks defined in test files
- **Config priority:** CLI flags > env vars > YAML file. See [docs/configuration.md](docs/configuration.md)
- **Comments:** English only; every exported symbol must have a godoc comment starting with its name; no inline comments on `if`/`for`/`return` lines unless genuinely non-obvious. See [.spec/CODE_STYLE.md](.spec/CODE_STYLE.md) §11.

## Pitfalls & Gotchas

- **Docker socket required** — service mounts `/var/run/docker.sock` to execute plugin containers
- **Port 5432 conflict** — if postgres already runs locally, minimal stack uses port 5433 (`EASYP_POSTGRES_PORT`)
- **Plugin images must exist** — run `task local-push-registry` (or `local-push-required`) before the service can generate code
- **License:** `MockLicenseClient` always returns Enterprise (production placeholder; TODO: replace with real gRPC client when license server is ready)
- **`easyp generate` needs running service** — the generate command calls localhost:8080 gRPC
- **Migration order matters** — files are sorted by numeric prefix; never reorder
- **Audit channel capacity** — fixed at 1000; if exceeded, events are silently dropped (logged as warning)
- **WorkerPool `Get()` is non-blocking** — returns `ErrServerOverloaded` immediately if queue is full

## Documentation

**Agent-optimized specs:** `.spec/` directory — start with [.spec/README.md](.spec/README.md) for full index.

| Topic | Spec (agent) | Docs (human) |
|-------|-------------|--------------|
| Architecture | [.spec/ARCHITECTURE.md](.spec/ARCHITECTURE.md) | [docs/architecture.md](docs/architecture.md) |
| Packages | [.spec/PACKAGES.md](.spec/PACKAGES.md) | — |
| Domain model | [.spec/DOMAIN.md](.spec/DOMAIN.md) | — |
| Code style | [.spec/CODE_STYLE.md](.spec/CODE_STYLE.md) | — |
| Tools & commands | [.spec/TOOLS.md](.spec/TOOLS.md) | [docs/development.md](docs/development.md) |
| Testing | [.spec/TESTING.md](.spec/TESTING.md) | — |
| Errors | [.spec/ERRORS.md](.spec/ERRORS.md) | — |
| Auth & licensing | [.spec/AUTH.md](.spec/AUTH.md) | [docs/licensing.md](docs/licensing.md) |
| Database | [.spec/DATABASE.md](.spec/DATABASE.md) | [docs/database.md](docs/database.md) |
| API | [.spec/API.md](.spec/API.md) | [docs/api/generator/v1/generator.md](docs/api/generator/v1/generator.md) |
| Deployment | [.spec/DEPLOYMENT.md](.spec/DEPLOYMENT.md) | [docs/deployment.md](docs/deployment.md) |
| Observability | [.spec/OBSERVABILITY.md](.spec/OBSERVABILITY.md) | [docs/monitoring.md](docs/monitoring.md) |
| Clients (SDK) | [.spec/CLIENTS.md](.spec/CLIENTS.md) | [docs/mcp.md](docs/mcp.md) |
| Security | [.spec/SECURITY.md](.spec/SECURITY.md) | — |
| Feature flags | [.spec/FEATURE_FLAGS.md](.spec/FEATURE_FLAGS.md) | — |
| Background jobs | [.spec/BACKGROUND_JOBS.md](.spec/BACKGROUND_JOBS.md) | — |
| Configuration | — | [docs/configuration.md](docs/configuration.md) |

## Key Dependencies

| Dependency | Purpose |
|---|---|
| `google.golang.org/grpc` | gRPC framework |
| `google.golang.org/protobuf` | Protobuf serialization |
| `github.com/jmoiron/sqlx` + `github.com/lib/pq` | PostgreSQL access |
| `aidanwoods.dev/go-paseto` | PASETO v4.public license tokens |
| `github.com/modelcontextprotocol/go-sdk` | MCP protocol |
| `go.opentelemetry.io/otel` | OpenTelemetry tracing + metrics |
| `github.com/grafana/pyroscope-go` | Continuous profiling |
| `github.com/prometheus/client_golang` | Prometheus metrics |
| `github.com/grpc-ecosystem/go-grpc-middleware/v2` | gRPC middleware |
| `github.com/sethvargo/go-envconfig` | Environment variable config binding |
| `golang.org/x/time` | Token bucket rate limiter |

## Ports

| Port | Service | Protocol |
|------|---------|----------|
| 8080 | gRPC API | gRPC (H2) |
| 8081 | Metrics | HTTP (`/metrics`) |
| 8082 | Health | HTTP (`/health`) |
| 8083 | MCP | HTTP (`/mcp`) |
| 5005 | Docker Registry | HTTP (v2 API) |
| 5432/5433 | PostgreSQL | TCP |

## Skills

This project includes agent skills in `.agents/skills/`. Each skill provides domain-specific knowledge and workflows.

| Skill | Path | When to Use |
|-------|------|-------------|
| **sdd** | `.agents/skills/sdd/SKILL.md` | New features, "implement X", "build X", spec-first approach. 6-phase pipeline with human approval gates. |
| **protobuf-expert-skill** | `.agents/skills/protobuf-expert-skill/SKILL.md` | Writing/reviewing `.proto` files, configuring `easyp.yaml`, lint rules, code generation plugins, proto dependencies, breaking changes, debugging easyp errors. |
| **protoc-gen-mcp-skill** | `.agents/skills/protoc-gen-mcp-skill/SKILL.md` | Building MCP servers from protobuf definitions, generating MCP tools from proto files, adding MCP annotations, `protoc-gen-mcp` code generation. |

### Spec-Driven Development

**Pipeline:** Explore → Requirements → Design → Task Plan → Implementation → Review. Each phase requires explicit human approval before advancing.

**How to start:** tell the agent "I want to add feature X" — the skill activates automatically via keyword matching.

### Protobuf Expert

Covers the full EasyP CLI toolkit: `easyp lint`, `easyp generate`, `easyp breaking`, `easyp mod`, `easyp init`. Includes lint rule selection, managed mode configuration, CI/CD integration, and migration from buf.build.

### protoc-gen-mcp

Generates type-safe Go MCP tool bindings (`*.mcp.go`) from annotated protobuf services. Proto is the source of truth — define service once in `.proto`, generate both `*.pb.go` and `*.mcp.go`, implement the handler interface, and serve.
