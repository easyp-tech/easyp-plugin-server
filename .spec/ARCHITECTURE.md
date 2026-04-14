<!-- generated: 2026-04-14, template: core.md -->
# Architecture

## 1. Overview

Centralized protobuf/gRPC plugin execution service using a layered architecture (Transport → Application → Adapters → Infrastructure) with the Decorator pattern for cross-cutting concerns.

```
┌──────────────────────────────────────────────────────────┐
│  Transport Layer                                          │
│  gRPC server + interceptor chain, MCP HTTP, Metrics HTTP │
├──────────────────────────────────────────────────────────┤
│  Application Layer (core)                                 │
│  Business logic, domain types, worker pool               │
├──────────────────────────────────────────────────────────┤
│  Adapters Layer                                           │
│  audit/, metrics/, registry/ — implement core interfaces │
├──────────────────────────────────────────────────────────┤
│  Infrastructure                                           │
│  PostgreSQL (sqlx), Docker (plugin containers), OTLP     │
└──────────────────────────────────────────────────────────┘
```

## 2. Component Deep Dive

### Transport Layer — `internal/api/`

| File | Description |
|------|-------------|
| `api.go` | gRPC handler implementations (`ServiceAPIServer`) |
| `audit_interceptor.go` | Audit logging interceptor (non-blocking channel send) |
| `license_interceptor.go` | License-based feature gating interceptor |
| `mcp.go` | MCP protocol server setup |
| `mcp_tools.go` | MCP tool handlers for plugin listing |

### Transport Layer — `internal/grpchelper/`

| File | Description |
|------|-------------|
| `server.go` | gRPC server factory with full interceptor chain |
| `client.go` | gRPC client factory |
| `errors.go` | Internal error sentinel |
| `grpc_codes.go` | Domain error → gRPC status code converter |
| `grpc_logs.go` | Structured logging adapter for gRPC middleware |
| `logger.go` | Logging interceptor |
| `metrics.go` | Prometheus server metrics factory |
| `trace_logging.go` | Trace ID injection into logs |

### Application Layer — `internal/core/`

| File | Description |
|------|-------------|
| `domain.go` | All domain types, interfaces, sentinel errors |
| `core.go` | Business logic (`Core` struct implements `CoreService`) |
| `pool.go` | `WorkerPool` — bounded concurrency for Docker execution |

### Adapters Layer — `internal/adapters/`

| Package | Implements | Description |
|---------|-----------|-------------|
| `audit/` | `core.AuditLog` | PostgreSQL audit log storage + async worker |
| `metrics/` | `core.Metrics` | Prometheus business metrics + DB pool metrics |
| `registry/` | `core.Registry` | PostgreSQL plugin storage + Docker container execution |

### Infrastructure

| Package | Description |
|---------|-------------|
| `internal/database/` | `sqlx` wrapper with metrics, tracing, transactions |
| `internal/license/` | PASETO v4 license management, FeatureGate, claims |
| `internal/telemetry/` | OTLP init, Pyroscope, tracing decorators |
| `internal/ratelimiter/` | Per-IP token bucket rate limiter |
| `internal/monitor/` | Context-attached `slog` logger |

## 3. Directory Structure

```
service/
├── api/generator/v1/           # Proto contract + generated Go stubs + MCP bindings
├── cmd/
│   ├── main.go                 # Entry point: config → wiring → 4 HTTP servers
│   └── mcp-smoke/main.go      # MCP smoke test
├── internal/
│   ├── core/
│   │   ├── domain.go           # Types, interfaces, errors (single source of truth)
│   │   ├── core.go             # Business logic
│   │   └── pool.go             # WorkerPool (bounded Docker concurrency)
│   ├── api/
│   │   ├── api.go              # gRPC handlers
│   │   ├── audit_interceptor.go
│   │   ├── license_interceptor.go
│   │   ├── mcp.go              # MCP setup
│   │   └── mcp_tools.go        # MCP tool handlers
│   ├── adapters/
│   │   ├── audit/              # AuditLog impl (Store + Worker)
│   │   ├── metrics/            # Prometheus collectors
│   │   └── registry/           # Registry impl (DB + Docker)
│   ├── database/               # sqlx wrapper, connectors, migrations
│   ├── grpchelper/             # gRPC server/client, interceptor chain
│   ├── license/                # PASETO manager, FeatureGate, claims
│   ├── ratelimiter/            # Per-IP token bucket
│   ├── telemetry/              # OTLP, Pyroscope, tracing decorators
│   ├── monitor/                # Context slog logger
│   └── flags/                  # CLI flag types
├── sdk/                        # Go client SDK
├── migrate/                    # SQL migrations (1-4)
├── registry/                   # Plugin Dockerfiles
└── configs/                    # Observability configs
```

## 4. Key Design Decisions

1. **Layered architecture with clean interfaces**
   - Domain types and interfaces defined once in `core/domain.go`
   - Business logic in `core/core.go` delegates to `Registry` interface
   - Adapters are swappable without touching business logic

2. **Decorator pattern for tracing**
   - `TracingCore`, `TracingRegistry`, `TracingPlugin` in `telemetry/`
   - Wraps existing interfaces with OpenTelemetry spans
   - Zero tracing code in business logic or adapters

3. **Worker pool for Docker parallelism**
   - `WorkerPool` wraps `Registry` and limits concurrent Docker containers
   - Non-blocking `Get()` returns `ErrServerOverloaded` when queue full
   - Configurable workers, queue size, generation timeout, retries

4. **Interceptor chain for cross-cutting concerns**
   - Order: trace_logging → realip → prometheus → logging → recovery → validation → code_conversion → rate_limit → license → audit
   - Extra interceptors appended after built-in chain
   - Each interceptor is independently testable

5. **PASETO v4 licensing with FeatureGate**
   - Community mode (defaults) when no license key provided
   - Enterprise mode with configurable features, worker limits, plugin limits
   - `FeatureGate` interface decouples business logic from licensing details

6. **gRPC as primary API with MCP secondary**
   - Protobuf defines the contract (`generator.proto`)
   - MCP handler exposes same functionality over HTTP for LLM integration
   - Both use the same `CoreService` interface

## 5. Data Flow

### GenerateCode Request

```
gRPC Client (SDK or easyp CLI)
  → net.Listener :8080
    → gRPC Server
      → TraceLogging (inject trace_id into slog)
        → RealIP (extract client IP from headers)
          → Prometheus (record metrics)
            → Logging (structured request/response logs)
              → Recovery (panic → codes.Internal)
                → Validator (protobuf field validation)
                  → CodeConversion (domain error → gRPC status)
                    → RateLimit (per-IP token bucket)
                      → License (check FeatureGate)
                        → Audit (record operation to channel)
                          → API.GenerateCode() handler
                            → Core.Generate()
                              ├─ Parse group/name/version from plugin_name
                              ├─ WorkerPool.Get() [enqueue job]
                              │   └─ Worker goroutine:
                              │       → Registry.Get() [SQL query]
                              │       ← Return Plugin
                              ├─ poolPlugin.Generate() [retry with timeout]
                              │   └─ plugin.Generate()
                              │       └─ docker run --rm -i \
                              │            --network=none --memory=128m --cpus=1.0 \
                              │            <registry>/<group>/<name>:<version>
                              │            (stdin: CodeGeneratorRequest, stdout: CodeGeneratorResponse)
                              └─ Metrics.GenerateCode() [record counter]
                            ← GenerateCodeResponse
                          ← AuditInterceptor [async write to channel → Worker → DB]
                        ← gRPC Response to client
```

### Dependency Wiring (cmd/main.go)

```
Config (YAML or ENV)
  → Telemetry (TracerProvider + MeterProvider + Pyroscope)
  → Database (sqlx + migrations)
  → Registry (DB + Docker)
  → Metrics Adapters (Prometheus collectors)
  → AuditWorker + AuditStore + AuditChannel
  → LicenseManager + FeatureGate
  → RateLimiter
  → TracingRegistry (decorator)
  → WorkerPool (limits Docker parallelism)
  → Core (business logic)
  → TracingCore (decorator)
  → API (gRPC handlers)
  → 4 servers: gRPC(:8080), Metrics(:8081), Health(:8082), MCP(:8083)
```
