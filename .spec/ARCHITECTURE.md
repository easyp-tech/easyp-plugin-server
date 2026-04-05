<!-- generated: 2026-04-03, template: core.md -->
# Architecture

## Overview

Layered gRPC service with decorator-based tracing, bounded worker pool for Docker plugin execution, and composable interceptor chain.

```
gRPC Client
  │
  ▼
gRPC Server (interceptor chain)
  │
  ▼
API (handlers)
  │
  ▼
TracingCore (decorator)
  │
  ▼
Core (business logic)
  │
  ├──────────────────────┐
  ▼                      ▼
WorkerPool            ListPlugins
(backpressure,        (direct pass-through)
 retry, timeout)
  │
  ▼
TracingRegistry (decorator)
  │
  ▼
Registry (PostgreSQL + Docker exec)
  │
  ├──────────────┐
  ▼              ▼
database.SQL   Docker containers
(sqlx+metrics) (stdin/stdout protobuf)

Side systems:
├── License:     PASETO v4.public → Claims → FeatureGate → LicenseInterceptor
├── Audit:       AuditInterceptor → chan(1000) → Worker → Store (PostgreSQL)
├── RateLimiter: per-IP token bucket with FeatureGate integration
├── Telemetry:   OTLP (traces+metrics) → Alloy → Mimir/Loki/Tempo + Pyroscope
└── MCP Server:  Streamable HTTP transport for plugin discovery tools
```

## Interceptor Chain

Order matters — each interceptor wraps the next:

| # | Interceptor | Package | Purpose |
|---|-------------|---------|---------|
| 1 | `TraceLogging` | `grpchelper` | Injects trace/span IDs into logger context |
| 2 | `realip` | `go-grpc-middleware` | Extracts real client IP from headers |
| 3 | `prometheus` | `go-grpc-middleware` | gRPC request metrics (latency, codes) |
| 4 | `logging` | `go-grpc-middleware` | Structured request/response logging |
| 5 | `recovery` | `go-grpc-middleware` | Panic recovery → `codes.Internal` + counter |
| 6 | `validator` | `go-grpc-middleware` | Protobuf field validation |
| 7 | `error_code_conversion` | `grpchelper` | Domain errors → gRPC status codes |
| 8 | `rate_limit` | `ratelimiter` | Per-IP token bucket (10 rps, 20 burst) |
| 9 | `license` | `api` | Feature gate enforcement per method |
| 10 | `audit` | `api` | Non-blocking audit event capture |

Interceptors 1-7 are built in `grpchelper.NewServer()`. Interceptors 8-10 are passed as `extraUnary`/`extraStream` from `cmd/main.go`.

## Component Deep Dive

### Transport Layer — `internal/api/`

| File | Description |
|------|-------------|
| `api.go` | gRPC handler: `GenerateCode`, `Plugins`. Converts proto ↔ domain. `ErrorToStatus` maps domain errors → gRPC codes |
| `audit_interceptor.go` | Non-blocking audit: captures operation type, duration, caller IP, sends to channel |
| `license_interceptor.go` | Maps RPC methods → required `license.Feature`, denies if `FeatureGate.Enabled()` returns false |

### Application Layer — `internal/core/`

| File | Description |
|------|-------------|
| `domain.go` | ALL domain types, interfaces (`Registry`, `Plugin`, `Metrics`, `AuditLog`, `FeatureGate`, `CoreService`), sentinel errors |
| `core.go` | `Core` struct: `Generate()` parses plugin name, calls `Registry.Get()`, runs `Plugin.Generate()`, records metrics |
| `pool.go` | `WorkerPool`: bounded goroutine pool wrapping `Registry`, backpressure (non-blocking `Get`), retry on transient errors, timeout |

### Adapters Layer — `internal/adapters/`

| Package | Implements | Description |
|---------|-----------|-------------|
| `adapters/registry` | `core.Registry`, `core.Plugin` | PostgreSQL plugin catalog + Docker `exec` for code generation |
| `adapters/audit` | `core.AuditLog` | PostgreSQL audit storage + background channel worker |
| `adapters/metrics` | `core.Metrics` | Prometheus business metrics, DB pool collectors |

### Infrastructure Layer

| Package | Description |
|---------|-------------|
| `database` | `SQL` wrapper over `sqlx.DB` with auto-metrics, tracing, transaction helpers |
| `grpchelper` | gRPC server/client factories, interceptor chain builder, error conversion |
| `license` | PASETO v4 token management, `FeatureGate`, `Claims`, expiration watcher |
| `ratelimiter` | Per-IP token bucket with `FeatureGate` bypass for enterprise |
| `telemetry` | OTLP + Pyroscope init, tracing decorators (`TracingCore`, `TracingRegistry`, `TracingPlugin`) |
| `mcpserver` | MCP protocol server (Streamable HTTP) for plugin discovery |
| `monitor` | Context-aware `slog.Logger` |
| `flags` | CLI flag types (`File` reader, log `Level`) |

## Data Flow: GenerateCode Request

```
gRPC Request (CodeGeneratorRequest + plugin_name)
  → Interceptor chain (trace_logging → realip → prometheus → logging → recovery → validator → error_codes → rate_limit → license → audit)
    → API.GenerateCode()
      → TracingCore.Generate() [creates span "core.Generate"]
        → Core.Generate()
          → getGroup(), getNameAndVersion() — parse plugin_name
          → WorkerPool.Get(group, name, version) [creates span "pool.Get"]
            → non-blocking queue send (or ErrServerOverloaded)
            → worker goroutine picks up job
              → TracingRegistry.Get() [creates span "registry.Get"]
                → Registry.Get() — SQL lookup + Docker container reference
            → returns poolPlugin (wraps Plugin with timeout + retry)
          → poolPlugin.Generate(CodeGeneratorRequest)
            → context.WithTimeout(cfg.GenerationTimeout)
            → Plugin.Generate() — Docker exec: stdin protobuf → stdout protobuf
            → on transient error (exit 125/126/127): retry up to MaxRetries
            → records duration + error metrics
        ← GenerateCodeResponse
      ← proto GenerateCodeResponse
    ← Audit interceptor records: operation, plugin, caller IP, duration, status
  ← gRPC Response
```

## Key Design Decisions

1. **Decorator pattern for tracing** — `TracingCore`, `TracingRegistry`, `TracingPlugin` wrap business interfaces without modifying them. Tracing code never leaks into `core/`.

2. **Worker pool as Registry decorator** — `WorkerPool` implements `core.Registry`, wrapping the real registry. This gives bounded concurrency for Docker execution without changing the call chain.

3. **Non-blocking backpressure** — `WorkerPool.Get()` uses a `select` with `default` on the job channel. If the queue is full, it immediately returns `ErrServerOverloaded` (gRPC `ResourceExhausted`) rather than blocking.

4. **FeatureGate with int parameters** — `core.FeatureGate` uses `int` for feature identifiers to avoid cyclic imports between `core` and `license` packages. Adapter in `cmd/main.go` bridges the types.

5. **Composable interceptor chain** — Base interceptors (trace, realip, prometheus, logging, recovery, validator, error_codes) are built in `grpchelper.NewServer()`. Business interceptors (rate_limit, license, audit) are injected as extras from `cmd/main.go`.

6. **Audit event buffering** — Audit interceptor sends events to a buffered channel (capacity 1000). A background goroutine consumes and persists to PostgreSQL. Non-blocking send drops events on overflow (logged as warning).
