# EasyP API Service Architecture

## Overview

EasyP API Service is a code generation service from Protocol Buffers using Docker containers for isolated plugin execution.

## Component Interaction Diagram

```
gRPC Client → API Layer (gRPC + interceptors) → Core (business logic) → WorkerPool → Registry → Docker
                                                      ↑                             → PostgreSQL
                                                      FeatureGate
                                                      ↑
MCP Client  → MCP Server (HTTP)               → Core  LicenseManager
```

## Components

### Transport Layer (`internal/api`)

A gRPC server with a chain of interceptors that process each request in the following order:

1. **Trace logging** — request trace logging
2. **Real IP** — extraction of the client's real IP address
3. **Prometheus metrics** — request metrics collection (grpc-ecosystem/go-grpc-middleware)
4. **Structured logging** — structured request logging
5. **Panic recovery** — panic interception with `panics_total` counter increment
6. **Validation** — incoming message validation
7. **Error code conversion** — conversion of internal errors to gRPC codes
8. **Rate limit interceptor** — per-client rate limiting via token bucket algorithm (uses `grpc-ecosystem/go-grpc-middleware/v2/interceptors/ratelimit`). Controlled by FeatureGate; when disabled, all requests pass through
9. **License interceptor** — license-based feature gating (Enterprise methods require a valid license; Community methods pass through)
10. **Audit interceptor** — non-blocking audit event recording

### Core (`internal/core`)

The `Core` struct is the central business logic component. Main methods:

- `Generate()` — code generation from protobuf files using the specified plugin
- `ListPlugins()` — retrieval of the list of available plugins

When `Generate()` is called:
- Parses the plugin name in `group/name:version` format
- Retrieves the plugin from the Registry
- Executes generation via the WorkerPool
- Records metrics (counters, histograms)

### WorkerPool (`internal/core/pool.go`)

A wrapper over the Registry interface that limits the concurrency of Docker container execution.

**Characteristics:**
- Bounded queue with backpressure mechanism — returns `ErrServerOverloaded` when the queue is full
- Retry logic for transient errors:
  - Docker exit codes: 125, 126, 127
  - Connection refused
  - Docker daemon issues
- Configurable parameters:
  - `workers` — number of workers (default: 4)
  - `queue_size` — task queue size (default: 16)
  - `generation_timeout` — generation timeout (default: 120s)
  - `max_retries` — maximum number of retries (default: 3)
  - `shutdown_timeout` — graceful shutdown timeout (default: 30s)

### Registry Adapter (`internal/adapters/registry`)

A plugin registry adapter with PostgreSQL storage.

- Executes plugins via `docker run --rm -i` with security constraints:
  - No network (`--network none`)
  - Memory limit: 128 MB
  - CPU limit: 1 core
- Docker configuration for each plugin is stored as JSONB in the `plugins` table

### Audit System (`internal/adapters/audit`)

An asynchronous audit system.

- Worker with a buffered channel (capacity: 1000 events)
- Writes to the `audit_log` table
- Non-blocking dispatch from the gRPC interceptor — when the channel overflows, the event is dropped (`audit_events_lost_total` is incremented)

### License System (`internal/license`)

A license-based feature gating system built on PASETO v4.public tokens (Ed25519 signatures).

**Components:**

- **Feature enum** — typed `int` constants defined with `iota`. Each feature is classified as Community or Enterprise. Methods: `String()`, `IsEnterprise()`, `Valid()`
- **LicenseManager** — parses and validates PASETO v4.public tokens using a public key embedded at build time via `-ldflags`. Caches parsed claims in memory. Runs an expiration watcher (60s ticker) that transitions to Community mode when the license expires
- **FeatureGate** — provides `Enabled(feature)`, `MaxWorkers()`, and `MaxPlugins()` based on the current cached claims. Increments the `easyp_license_feature_denied_total` Prometheus counter when an Enterprise feature is denied

**Tiers:**

| Tier | Features | MaxWorkers | MaxPlugins |
|------|----------|------------|------------|
| Community (default) | Code generation, plugin listing, MCP tools, rate limiting, plugin CRUD | 4 | 10 |
| Enterprise | All | 16 | -1 (unlimited) |

**Graceful degradation:** when no license is configured, the token is invalid, or the license expires at runtime, the system falls back to Community mode without interruption.

**Integration:**
- `FeatureGate` is injected into `Core` via constructor. The `core.FeatureGate` interface uses `Enabled(feature int)` (not `license.Feature`) to avoid circular dependencies. A `featureGateAdapter` in `cmd/main.go` bridges `license.FeatureGate` (Feature type) to `core.FeatureGate` (int type)
- `LicenseInterceptor` in the gRPC chain checks the FeatureGate for Enterprise methods and returns `PERMISSION_DENIED` when the feature is not enabled. Community methods (those without a method-to-feature mapping) pass through without checks

### Rate Limiter (`internal/ratelimiter`)

A per-client rate limiting system integrated as a gRPC interceptor.

**Algorithm:** Token bucket via `golang.org/x/time/rate`. Each client gets an independent bucket identified by IP address (extracted via `KeyExtractor` abstraction).

**Components:**

- **Config** — `RequestsPerSecond` (token refill rate), `Burst` (max tokens), `CleanupInterval` (stale bucket cleanup period)
- **KeyExtractor** — `func(ctx context.Context) string` abstraction for client identification. Default implementation `PeerIPExtractor` extracts IP from `peer.FromContext()`. Extensible to API key, tenant ID, etc.
- **RateLimiter** — implements `ratelimit.Limiter` interface from `grpc-ecosystem/go-grpc-middleware/v2`. Per-client buckets stored in `sync.Map`. Background goroutine cleans up stale buckets
- **Prometheus metrics** — `easyp_rate_limit_requests_total` (allowed/denied), `easyp_rate_limit_active_clients`

**Behavior:**
- Controlled by FeatureGate (`FeatureRateLimiting`). When disabled, all requests pass through
- Empty key from KeyExtractor → fail-open (request allowed)
- Denied requests return `RESOURCE_EXHAUSTED` with `X-RateLimit-*` headers in gRPC metadata
- Allowed requests include `X-RateLimit-*` headers in response metadata

### Metrics (`internal/adapters/metrics`)

Prometheus-based metrics:

- **Generation counters and histograms**: count, duration, errors, retries
- **DB connection pool metrics**: open/idle connections, waits
- **Business metrics**: PostgreSQL queries for plugin counts, audit records

### Telemetry (`internal/telemetry`)

Decorator pattern for tracing:

- `TracingCore` — decorator for Core
- `TracingRegistry` — decorator for Registry
- `TracingPlugin` — decorator for Plugin

**Stack:**
- OTLP exporter → Alloy → Mimir (metrics) / Loki (logs) / Tempo (traces)
- Pyroscope for continuous profiling
- `TraceHandler` enriches slog with trace/span IDs

### MCP Server (`internal/mcpserver`)

A Model Context Protocol server with Streamable HTTP transport.

**Tools:**
- `plugins_list` — plugin search with filtering
- `easyp_config_describe` — easyp.yaml schema description

### Database (`internal/database`)

An SQL wrapper over sqlx:

- OpenTelemetry query tracing
- Connection pool: 50 max open, 50 max idle, 60s lifetime, 10s idle timeout
- Pool metrics
- Transaction support with panic interception

## Code Generation Request Flow

1. The gRPC client sends a `GenerateRequest` with protobuf files and the plugin name
2. The request passes through the interceptor chain (validation, logging, metrics, license check, audit)
3. `Core.Generate()` parses the plugin name (`group/name:version`)
4. Core checks the FeatureGate for feature availability and applies license-based limits
5. Core requests the plugin from the Registry (PostgreSQL)
6. The task is submitted to the WorkerPool
7. The WorkerPool checks queue availability (backpressure)
8. A worker executes `docker run --rm -i` with security constraints
9. Protobuf data is passed to the container's stdin, the result is read from stdout
10. On transient errors, a retry is performed (up to `max_retries` attempts)
11. The result is returned to the client via gRPC, metrics and audit are recorded

## Design Patterns

| Pattern | Usage |
|---------|-------|
| **Decorator** | Tracing: `TracingCore`, `TracingRegistry`, `TracingPlugin` wrap interfaces, adding spans |
| **Worker Pool** | Limiting Docker container concurrency with a queue and backpressure |
| **Adapter** | Adapters for metrics, audit, registry — isolate infrastructure from business logic. `featureGateAdapter` bridges `license.FeatureGate` to `core.FeatureGate` to avoid circular dependencies |
| **Middleware Chain** | gRPC interceptor chain for cross-cutting concerns |
| **Feature Gate** | `FeatureGate` controls feature availability based on the current license tier |
