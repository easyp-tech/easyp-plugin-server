<!-- generated: 2026-04-03, template: core.md -->
# Packages

## Application Layer

### `internal/core`
**Domain types, interfaces, business logic, worker pool.**

| File | Description |
|------|-------------|
| `domain.go` | All domain types (`Plugin`, `PluginInfo`, `PluginFilter`, `GenerateCodeRequest`, `GenerateCodeResponse`, `CreatePluginRequest`, `UpdatePluginRequest`, `AuditEntry`), interfaces (`Registry`, `Plugin`, `Metrics`, `AuditLog`, `FeatureGate`, `CoreService`), sentinel errors |
| `core.go` | `Core` struct: `Generate()` (parse name → registry get → plugin generate → record metrics), `ListPlugins()`, `CreatePlugin()` (validate + MaxPlugins check + registry create), `UpdatePlugin()` (validate + registry update), `DeletePlugin()` (validate + registry delete) |
| `crud_test.go` | CRUD unit tests: preservation tests, create/update/delete success and error paths, validation table-driven tests. Manual mocks. |
| `pool.go` | `WorkerPool`: bounded goroutine pool implementing `Registry`, backpressure, retry on transient Docker errors, configurable timeout. Pass-through for Create/Update/Delete |
| `pool_test.go` | Config normalization, get/shutdown, retry logic, backpressure tests. Manual mocks. |

### `internal/api`
**gRPC transport layer — handlers and interceptors.**

| File | Description |
|------|-------------|
| `api.go` | `API` struct implementing `ServiceAPIServer`. `GenerateCode()`, `Plugins()`, `CreatePlugin()`, `UpdatePlugin()`, `DeletePlugin()`. `ErrorToStatus()` maps domain → gRPC codes |
| `audit_interceptor.go` | `AuditInterceptor`: non-blocking channel send of `core.AuditEntry` per request |
| `license_interceptor.go` | `LicenseInterceptor`: maps RPC method → `license.Feature`, denies with `PermissionDenied` if disabled |

## Adapters Layer

### `internal/adapters/registry`
**PostgreSQL plugin catalog + Docker container execution. Implements `core.Registry` and `core.Plugin`.**

### `internal/adapters/audit`
**PostgreSQL audit log storage + background channel worker. Implements `core.AuditLog`.**

### `internal/adapters/metrics`
**Prometheus business metrics. Implements `core.Metrics`. Also provides `DBCollector` and `BusinessMetricsCollector`.**

## Infrastructure Layer

### `internal/database`
**sqlx wrapper with automatic metrics and tracing.**

| File | Description |
|------|-------------|
| `sql.go` | `SQL` struct wrapping `sqlx.DB`. `NewSQL()`, `Tx()`, `NoTx()`, `NoTxContext()`. Pool defaults: 50 open, 50 idle, 60s lifetime, 10s idle time |
| `metrics.go` | `MetricCollector` interface, `NewMetrics()` — auto-generates Prometheus metrics for repository methods via reflection |
| `connectors/` | `CockroachDB`, `PostgresDB`, `Raw` DSN connectors |
| `migrations/` | SQL migration parser (`-- up` / `-- down` delimiters) and runner |
| `internal/` | Reflection helpers for metric auto-labeling |

### `internal/grpchelper`
**gRPC server/client factories and middleware.**

| File | Description |
|------|-------------|
| `server.go` | `NewServer()`: builds interceptor chain, creates `grpc.Server` + `health.Server` |
| `client.go` | gRPC client dial helper with standard options |
| `errors.go` | `errInternal` sentinel |
| `grpc_codes.go` | `GRPCCodesConverterHandler` type, `UnaryConvertCodesServerInterceptor`, `StreamConvertCodesServerInterceptor` |
| `grpc_logs.go` | `interceptorLogger` adapter: `slog.Logger` → `logging.Logger` |
| `logger.go` | Logging utilities |
| `metrics.go` | `NewServerMetrics()` — Prometheus gRPC server metrics |
| `trace_logging.go` | `TraceLoggingUnary/StreamServerInterceptor` — injects trace/span IDs into logger |

### `internal/license`
**PASETO v4 license management and feature gating.**

| File | Description |
|------|-------------|
| `features.go` | `Feature` enum (8 values: `FeatureCodeGeneration`..`FeatureAudit`), `IsEnterprise()`, `String()` |
| `claims.go` | `Claims` struct (tier, features, limits, expiry), `CommunityDefaults()` |
| `manager.go` | `LicenseManager`: parse PASETO token, thread-safe claims access, expiration watcher (60s) |
| `gate.go` | `FeatureGate`: `Enabled(feature)`, `MaxWorkers()`, `MaxPlugins()` |
| `errors.go` | License-specific errors |
| `metrics.go` | `LicenseMetrics`: tier gauge, feature denied counter, expiration gauge |
| `claims_test.go` | Claims parsing and validation tests |
| `features_test.go` | Feature enum, community defaults, enterprise checks |

### `internal/mcpserver`
**MCP protocol server for plugin discovery.**

| File | Description |
|------|-------------|
| `server.go` | `New()`: creates MCP server with Streamable HTTP transport |
| `tools_plugins.go` | `plugins_list` tool: ListPlugins with group/name/version/tags filters |
| `server_test.go` | Integration test with `httptest.Server` + real MCP client |

### `internal/ratelimiter`
**Per-IP token bucket rate limiter with FeatureGate integration.**

| File | Description |
|------|-------------|
| `ratelimiter.go` | `RateLimiter` struct: `sync.Map` of per-IP `rate.Limiter`, Prometheus metrics, cleanup goroutine |
| `config.go` | `Config` struct (rps, burst, cleanup interval) |
| `extractor.go` | `KeyExtractor` interface, `PeerIPExtractor` default |

### `internal/telemetry`
**OpenTelemetry + Pyroscope initialization, tracing decorators.**

| File | Description |
|------|-------------|
| `telemetry.go` | `Init()`: OTLP exporter, tracer/meter providers, Pyroscope profiler, trace-enriched slog handler |
| `config.go` | `Config` (OTLP endpoint, service name, Pyroscope endpoint) |
| `tracing_core.go` | `TracingCore`: decorator for `core.CoreService`, adds spans for `Generate`, `ListPlugins`, `CreatePlugin`, `UpdatePlugin`, `DeletePlugin` |
| `tracing_registry.go` | `TracingRegistry`: decorator for `core.Registry`, adds spans for `Get`, `List`, `Create`, `Update`, `Delete` |
| `tracing_plugin.go` | `TracingPlugin`: decorator for `core.Plugin`, adds spans for `Generate`, `Info` |
| `trace_handler.go` | `TraceHandler`: slog handler that enriches log records with trace/span IDs |
| `trace_handler_test.go` | Tests for trace context propagation into logs |

### `internal/monitor`
**Context-aware structured logger.**

| File | Description |
|------|-------------|
| `log.go` | `WithContext()`, `FromContext()` — store/retrieve `slog.Logger` in `context.Context` |

### `internal/flags`
**CLI flag types.**

| File | Description |
|------|-------------|
| `file.go` | `File` type: `flag.Value` for reading config files with size limit |
| `level.go` | `Level` type: `flag.Value` for slog log levels |

## Client SDK

### `sdk`
**Go client SDK for EasyP service consumers.**

| File | Description |
|------|-------------|
| `client.go` | `Client` struct, `NewClient()`, `GenerateCode()`, `ListPlugins()`, `Close()` |
| `config.go` | Functional options: `WithInsecure`, `WithMaxRetries`, `WithGenerateCodeTimeout`, etc. |
| `retry.go` | `retryUnaryInterceptor`: exponential backoff with jitter for `Unavailable`, `ResourceExhausted`, `DeadlineExceeded` |
| `health.go` | `healthMonitor`: background goroutine checking gRPC health service |
| `filter.go` | `FilterPlugins()`: client-side filtering of plugin lists |
| `interceptors.go` | `LoggingInterceptor`, `MetricsInterceptor` for client-side observability |
| `doc.go` | Package documentation |

## Generated Code

### `api/generator/v1`
**Protobuf contract and generated stubs.**

- `generator.proto` — source of truth (5 RPCs: `GenerateCode`, `Plugins`, `CreatePlugin`, `UpdatePlugin`, `DeletePlugin`)
- `*.pb.go`, `*_grpc.pb.go` — generated gRPC stubs
- `generator.mcp.go` — auto-generated MCP tool bindings (protoc-gen-mcp)
