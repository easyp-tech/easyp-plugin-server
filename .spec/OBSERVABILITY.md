<!-- generated: 2026-05-15, template: infrastructure.md -->
# Observability

OpenTelemetry, Prometheus, and Pyroscope stack for EasyP Service.

## Overview

```
Application
  → OpenTelemetry SDK (traces + metrics)
    → OTLP exporter → Alloy collector
      → Tempo (traces)
      → Mimir (metrics)
  → Prometheus client
    → /metrics endpoint → Alloy scraper
  → Pyroscope SDK
    → Pyroscope server (continuous profiling)
  → slog (structured JSON logs)
    → stdout → Alloy → Loki
```

## Initialization (`internal/telemetry`)

```go
shutdownTelemetry, telLog, err := telemetry.Init(ctx, telCfg, baseHandler)
```

`Init()` sets up:
1. OTLP trace provider (gRPC exporter to `cfg.OTLPEndpoint`)
2. OTLP metric provider (gRPC exporter)
3. Pyroscope profiler (HTTP to `cfg.PyroscopeEndpoint`)
4. Trace-aware slog handler (injects `trace_id`, `span_id` into log records)

Returns a `shutdownTelemetry` function for graceful cleanup.

## Tracing Decorators

Non-invasive tracing via decorator pattern:

| Decorator | Wraps | Package |
|-----------|-------|---------|
| `TracingCore` | `core.Service` | `internal/telemetry/tracing_core.go` |
| `TracingRegistry` | `core.Registry` | `internal/telemetry/tracing_registry.go` |
| `TracingPlugin` | `core.Plugin` | `internal/telemetry/tracing_plugin.go` |

Each decorator creates OTel spans with relevant attributes (plugin name, version, etc.) and passes through errors without modification.

## Prometheus Metrics

### gRPC Metrics (`internal/grpchelper`)

Standard grpc-prometheus metrics:
- `grpc_server_handled_total` — RPCs by method and code
- `grpc_server_handling_seconds` — RPC duration histogram
- `grpc_server_msg_received_total` / `grpc_server_msg_sent_total`

### Application Metrics

| Metric | Type | Source |
|--------|------|--------|
| `panics_total` | Counter | `cmd/main.go` (panic recovery) |
| `pool_active_workers` | Gauge | `core/pool.go` |
| `pool_queue_depth` | Gauge | `core/pool.go` |
| `pool_rejected_total` | Counter | `core/pool.go` |
| `pool_jobs_total` | Counter | `core/pool.go` |
| `db_*_connections` | Gauge | `adapters/metrics/db_collector.go` |
| Business metrics (plugin counts) | Gauge | `adapters/metrics/business_collector.go` |
| Audit worker metrics | Counter/Gauge | `adapters/audit/worker.go` |
| License metrics | Counter/Gauge | `internal/license/manager.go` |

### Endpoint

`GET /metrics` on port 8081 (default 23411).

## Logging

- **Format:** JSON (structured)
- **Library:** `log/slog` (stdlib)
- **Handler:** `slog.NewJSONHandler` with `AddSource: true`
- **Trace correlation:** `telemetry.TraceHandler` wraps the JSON handler, injecting `trace_id` and `span_id`
- **Context propagation:** `monitor.FromContext(ctx)` / `monitor.WithContext(ctx, log)`

## Configuration

```yaml
telemetry:
  otlp_endpoint: "localhost:4317"
  pyroscope_endpoint: "http://localhost:4040"
```

## Observability Stack (docker-compose)

| Service | Port | Role |
|---------|------|------|
| Alloy | 4317-4318 | OpenTelemetry collector (OTLP receiver) |
| Grafana | 3000 | Dashboards |
| Loki | 3100 | Log storage |
| Tempo | 3200 | Trace storage |
| Mimir | 9009 | Metrics storage |
| Pyroscope | 4040 | Continuous profiling |

Configs: `configs/` directory (Alloy, Grafana datasources, Loki, etc.)
