# EasyP API Service Architecture

## Overview

EasyP API Service is a code generation service from Protocol Buffers using Docker containers for isolated plugin execution.

## Component Interaction Diagram

```
gRPC Client → API Layer (gRPC + interceptors) → Core (business logic) → WorkerPool → Registry → Docker
                                                                                    → PostgreSQL
MCP Client  → MCP Server (HTTP)               → Core
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
8. **Audit interceptor** — non-blocking audit event recording

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
2. The request passes through the interceptor chain (validation, logging, metrics, audit)
3. `Core.Generate()` parses the plugin name (`group/name:version`)
4. Core requests the plugin from the Registry (PostgreSQL)
5. The task is submitted to the WorkerPool
6. The WorkerPool checks queue availability (backpressure)
7. A worker executes `docker run --rm -i` with security constraints
8. Protobuf data is passed to the container's stdin, the result is read from stdout
9. On transient errors, a retry is performed (up to `max_retries` attempts)
10. The result is returned to the client via gRPC, metrics and audit are recorded

## Design Patterns

| Pattern | Usage |
|---------|-------|
| **Decorator** | Tracing: `TracingCore`, `TracingRegistry`, `TracingPlugin` wrap interfaces, adding spans |
| **Worker Pool** | Limiting Docker container concurrency with a queue and backpressure |
| **Adapter** | Adapters for metrics, audit, registry — isolate infrastructure from business logic |
| **Middleware Chain** | gRPC interceptor chain for cross-cutting concerns |
