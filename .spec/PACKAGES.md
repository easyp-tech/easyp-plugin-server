<!-- generated: 2026-04-14, template: core.md -->
# Packages

## Application Layer

### `internal/core`
**Domain types, interfaces, and business logic.**

| File | Description |
|------|-------------|
| `domain.go` | All domain types, interfaces (`Registry`, `Plugin`, `CoreService`, `Metrics`, `AuditLog`, `FeatureGate`), sentinel errors, Feature enum, audit constants |
| `core.go` | `Core` struct implements `CoreService` — Generate, ListPlugins, CreatePlugin, UpdatePlugin, DeletePlugin |
| `pool.go` | `WorkerPool` wraps `Registry` to limit Docker concurrency. Non-blocking `Get()`, configurable workers/queue/timeout/retries |

Key details:
- Single source of truth for domain types
- `Core` delegates to `Registry` + `Metrics` + `FeatureGate`
- `WorkerPool` classifies transient vs permanent errors for retry logic

## Adapters Layer

### `internal/adapters/audit`
**Async audit log writer.**

| File | Description |
|------|-------------|
| `audit.go` | `Store` implements `core.AuditLog` — saves entries to `audit_log` PostgreSQL table |
| `worker.go` | `Worker` reads from buffered channel (cap 1000), batch writes, Prometheus metrics |

Key details:
- Non-blocking channel send from interceptor
- Graceful shutdown with timeout; returns lost event count
- Metrics: `audit_events_lost_total`, `audit_queue_depth`

### `internal/adapters/metrics`
**Prometheus metrics collectors.**

| File | Description |
|------|-------------|
| `metrics.go` | `Metrics` implements `core.Metrics` — `generated_plugin_code_total`, `generation_duration_seconds`, `generation_errors_total`, `generation_retries_total` |
| `business_collector.go` | `BusinessMetricsCollector` — `plugins_total`, `plugins_by_group`, `audit_log_total`, `audit_log_by_operation`, `plugin_versions_count`, `audit_log_last_24h` |
| `db_collector.go` | `DBCollector` — `db_open_connections`, `db_idle_connections`, `db_wait_count_total`, `db_wait_duration_seconds_total` |

### `internal/adapters/registry`
**Plugin storage + Docker execution.**

| File | Description |
|------|-------------|
| `registry.go` | `Registry` implements `core.Registry` — SQL CRUD + Docker container execution for code generation |

Key details:
- Docker flags: `--network=none`, `--memory=128m`, `--cpus=1.0`, `--user=nobody`
- Plugin config stored as JSONB in `plugins` table
- Supports `latest` version alias
- Tags filtered via PostgreSQL array operators (`@>`)

## Transport Layer

### `internal/api`
**gRPC handlers and interceptors.**

| File | Description |
|------|-------------|
| `api.go` | `API` implements `generator.ServiceAPIServer` — GenerateCode, Plugins, CreatePlugin, UpdatePlugin, DeletePlugin, ErrorToStatus |
| `api_test.go` | API handler tests |
| `audit_interceptor.go` | `AuditInterceptor` — maps gRPC methods to audit operations, records duration/status/metadata |
| `license_interceptor.go` | `LicenseInterceptor` — checks FeatureGate per gRPC method |
| `mcp.go` | MCP server setup with StreamableHTTP transport |
| `mcp_tools.go` | MCP tool handlers (plugin listing, config description) |

### `internal/grpchelper`
**gRPC server/client factories and middleware.**

| File | Description |
|------|-------------|
| `server.go` | `NewServer()` — creates gRPC server with full interceptor chain, reflection, health service |
| `client.go` | gRPC client factory |
| `errors.go` | `errInternal` sentinel |
| `grpc_codes.go` | `UnaryConvertCodesServerInterceptor` / `StreamConvertCodesServerInterceptor` |
| `grpc_logs.go` | `interceptorLogger()` adapter for slog → gRPC logging middleware |
| `logger.go` | Logging interceptor |
| `metrics.go` | `NewServerMetrics()` — Prometheus gRPC metrics |
| `trace_logging.go` | `TraceLoggingUnaryServerInterceptor` — injects trace_id into slog context |

## Infrastructure

### `internal/database`
**SQL wrapper with metrics and tracing.**

| File | Description |
|------|-------------|
| `sql.go` | `SQL` wraps `sqlx.DB` — `NoTx()`, `Tx()`, `NoTxContext()` with automatic metrics and error wrapping |
| `metrics.go` | DAL metrics registration factory |
| `connectors/` | PostgreSQL and CockroachDB connection string builders |
| `migrations/` | Migration parser and runner (up/down) |

Key details:
- Connection pool: MaxLifetime 60s, MaxIdleTime 10s, MaxOpen/Idle 50
- Retries on connection with exponential backoff
- Transaction wraps with defer rollback on panic

### `internal/license`
**PASETO v4 license management.**

| File | Description |
|------|-------------|
| `manager.go` | `LicenseManager` — parse/verify PASETO v4.public tokens, expiration watcher (60s), metrics |
| `gate.go` | `FeatureGate` — checks Enabled/MaxWorkers/MaxPlugins from claims |
| `claims.go` | `Claims` struct — tier, features, limits, timestamps |
| `features.go` | `feature` enum with Enterprise detection, string names |
| `errors.go` | License-specific sentinel errors |
| `metrics.go` | `license_valid`, `license_expiry_timestamp_seconds` gauges |

### `internal/telemetry`
**OpenTelemetry and profiling setup.**

| File | Description |
|------|-------------|
| `telemetry.go` | `Init()` — OTLP gRPC exporter, TracerProvider, MeterProvider (15s), Pyroscope profiler |
| `config.go` | `Config` struct (OTLP endpoint, service name, Pyroscope endpoint) |
| `tracing_core.go` | `TracingCore` decorator for `CoreService` |
| `tracing_registry.go` | `TracingRegistry` decorator for `Registry` |
| `tracing_plugin.go` | `TracingPlugin` decorator for `Plugin` |
| `trace_handler.go` | Trace context slog handler |

### `internal/ratelimiter`
**Per-IP token bucket rate limiter.**

| File | Description |
|------|-------------|
| `ratelimiter.go` | `RateLimiter` — per-IP buckets via `golang.org/x/time/rate`, feature-gated, cleanup goroutine, Prometheus metrics |
| `config.go` | `Config` — requests per second, burst, cleanup interval |
| `extractor.go` | `PeerIPExtractor` — extract client IP from gRPC peer context |

### `internal/monitor`
**Context-aware logging.**

| File | Description |
|------|-------------|
| `log.go` | `WithContext()` / `FromContext()` — attach/retrieve `*slog.Logger` from context |

### `internal/flags`
**CLI flag types.**

| File | Description |
|------|-------------|
| `file.go` | `File` flag type — reads config file with size limit |
| `level.go` | `Level` flag type — slog level parsing |

## Generated Code

### `api/generator/v1`
**Protobuf-generated stubs.**

| File | Description |
|------|-------------|
| `generator.proto` | API contract: ServiceAPI with 5 RPCs |
| `generator.pb.go` | Generated protobuf types |
| `generator_grpc.pb.go` | Generated gRPC client/server stubs |
| `generator.mcp.go` | Generated MCP tool bindings |

## Client SDK

### `sdk/`
**Go client SDK.**

| File | Description |
|------|-------------|
| `client.go` | `Client` — NewClient, GenerateCode, ListPlugins, Close |
| `config.go` | Functional options: WithInsecure, WithMaxRetries, WithTimeout, etc. |
| `filter.go` | `PluginFilter` for client-side filtering |
| `health.go` | Background health monitor (30s interval) |
| `interceptors.go` | Client-side logging and metrics interceptors |
| `retry.go` | Retry with exponential backoff (max 3) |
| `doc.go` | Package documentation |
