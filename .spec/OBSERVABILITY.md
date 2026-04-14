<!-- generated: 2026-04-14, template: infrastructure.md -->
# Observability

## 1. Overview

Full observability stack: OpenTelemetry for traces and metrics, Prometheus for scraping, Pyroscope for profiling, Grafana for visualization. All backends are part of the Grafana LGTM stack.

```
Service (OTLP gRPC)
  → Alloy (OTEL collector, :4317)
    → Tempo (traces)
    → Mimir (metrics)
    → Loki (logs)
  → Pyroscope (profiles, :4040)
  → Prometheus (scraped, :8081/metrics)
    → Grafana (:3000) ← dashboards
```

## 2. Components

| Component | Image | Port | Config | Purpose |
|-----------|-------|------|--------|---------|
| Alloy | grafana/alloy | 4317 | `configs/alloy/config.alloy` | OTEL collector (receives OTLP, forwards to backends) |
| Tempo | grafana/tempo | — | `configs/tempo/tempo.yaml` | Distributed trace storage |
| Mimir | grafana/mimir | — | `configs/mimir/mimir.yaml` | Long-term metrics storage (Cortex-compatible) |
| Loki | grafana/loki | — | `configs/loki/loki.yml` | Log aggregation |
| Pyroscope | grafana/pyroscope | 4040 | `configs/pyroscope/pyroscope.yaml` | Continuous profiling |
| Grafana | grafana/grafana | 3000 | `configs/grafana/` | Dashboards, data source provisioning |
| RustFS | — | 9000-9001 | — | S3-compatible storage for Tempo/Mimir/Loki/Pyroscope |

## 3. Backend Integration

### Initialization (`internal/telemetry/telemetry.go`)

```go
func Init(ctx context.Context, cfg Config, baseHandler slog.Handler) (ShutdownFunc, *slog.Logger, error)
```

Creates:
- **TracerProvider**: OTLP gRPC exporter → Alloy → Tempo
- **MeterProvider**: OTLP periodic reader (15s interval) → Alloy → Mimir
- **Propagator**: W3C TraceContext
- **Pyroscope profiler**: CPU, allocations, goroutines, inuse objects/space
- **Telemetry-enriched logger**: slog handler that adds trace_id to log entries

### Configuration
```go
type Config struct {
    OTLPEndpoint      string  // default "localhost:4317"
    ServiceName       string  // default "easyp-api-service"
    PyroscopeEndpoint string  // default "http://localhost:4040"
}
```

## 4. Instrumentation Layers

| Layer | Package | Technology | Span/Metric Pattern |
|-------|---------|------------|---------------------|
| gRPC transport | `grpchelper` | otelgrpc StatsHandler | Auto: `/{service}/{method}` |
| gRPC interceptors | `grpchelper` | go-grpc-middleware/prometheus | `grpc_server_*` metrics |
| Business logic | `telemetry` | TracingCore decorator | `Core.Generate`, `Core.ListPlugins`, etc. |
| Registry | `telemetry` | TracingRegistry decorator | `Registry.Get`, `Registry.List`, etc. |
| Plugin execution | `telemetry` | TracingPlugin decorator | `Plugin.Generate` |
| Database | `database` | Custom metrics wrapper | `dal_*` operation metrics |
| Rate limiter | `ratelimiter` | Prometheus counters | `rate_limit_requests_total` |
| Audit | `adapters/audit` | Prometheus counters | `audit_events_lost_total` |
| License | `license` | Prometheus gauges | `license_valid`, `license_expiry_*` |
| Worker pool | `core` | Prometheus gauges/counters | `pool_active_workers`, `pool_jobs_total` |

## 5. Trace-Log Correlation

`TraceLoggingUnaryServerInterceptor` (`internal/grpchelper/trace_logging.go`):
- Extracts `trace_id` from OpenTelemetry span context
- Injects into slog logger attached to request context
- All downstream log calls include `trace_id` field

## 6. Tracing Decorators

Non-invasive tracing via Decorator pattern:

```go
// telemetry/tracing_core.go
type TracingCore struct {
    inner core.CoreService
}
func (t *TracingCore) Generate(ctx context.Context, req GenerateCodeRequest) (*GenerateCodeResponse, error) {
    ctx, span := tracer.Start(ctx, "Core.Generate")
    defer span.End()
    return t.inner.Generate(ctx, req)
}
```

Same pattern for `TracingRegistry` and `TracingPlugin`.

## 7. Prometheus Metrics

**Scrape endpoint**: `http://host:8081/metrics`

**Business metrics** (`internal/adapters/metrics/`):
| Metric | Type | Labels |
|--------|------|--------|
| `generated_plugin_code_total` | Counter | plugin |
| `generation_duration_seconds` | Histogram | plugin |
| `generation_errors_total` | Counter | plugin, error_type |
| `generation_retries_total` | Counter | plugin |
| `plugins_total` | Gauge | — |
| `plugins_by_group` | Gauge | group |
| `audit_log_total` | Gauge | — |
| `audit_log_by_operation` | Gauge | operation |
| `audit_log_by_status` | Gauge | status |
| `plugin_versions_count` | Gauge | group, name |
| `audit_log_last_24h` | Gauge | — |

**Infrastructure metrics:**
| Metric | Type | Source |
|--------|------|--------|
| `db_open_connections` | Gauge | DB pool |
| `db_idle_connections` | Gauge | DB pool |
| `db_wait_count_total` | Counter | DB pool |
| `db_wait_duration_seconds_total` | Counter | DB pool |
| `rate_limit_requests_total` | Counter | Rate limiter |
| `rate_limit_active_clients` | Gauge | Rate limiter |
| `pool_active_workers` | Gauge | Worker pool |
| `pool_rejected_total` | Counter | Worker pool |
| `pool_jobs_total` | Counter | Worker pool |
| `pool_queue_depth` | Gauge | Worker pool |
| `audit_events_lost_total` | Counter | Audit worker |
| `audit_queue_depth` | Gauge | Audit worker |
| `panics_total` | Counter | gRPC recovery |
| `license_valid` | Gauge | License manager |
| `license_expiry_timestamp_seconds` | Gauge | License manager |

## 8. Grafana

**Access**: http://localhost:3000
**Config**: `configs/grafana/config.ini`
**Data sources**: Auto-provisioned via `configs/grafana/provisioning/datasources/`
**Dashboards**: Auto-provisioned via `configs/grafana/provisioning/dashboards/`

## 9. Key Files

| File | Description |
|------|-------------|
| `internal/telemetry/telemetry.go` | OTLP + Pyroscope initialization |
| `internal/telemetry/config.go` | Telemetry configuration struct |
| `internal/telemetry/tracing_core.go` | TracingCore decorator |
| `internal/telemetry/tracing_registry.go` | TracingRegistry decorator |
| `internal/telemetry/tracing_plugin.go` | TracingPlugin decorator |
| `internal/telemetry/trace_handler.go` | Trace context slog handler |
| `internal/adapters/metrics/metrics.go` | Business metrics (core.Metrics impl) |
| `internal/adapters/metrics/business_collector.go` | DB-sourced business metrics |
| `internal/adapters/metrics/db_collector.go` | Connection pool metrics |
| `internal/grpchelper/metrics.go` | gRPC server metrics factory |
| `internal/grpchelper/trace_logging.go` | Trace-log correlation interceptor |
| `configs/alloy/config.alloy` | Alloy OTEL collector config |
| `configs/tempo/tempo.yaml` | Tempo trace backend config |
| `configs/mimir/mimir.yaml` | Mimir metrics backend config |
| `configs/loki/loki.yml` | Loki log backend config |
| `configs/pyroscope/pyroscope.yaml` | Pyroscope profiling config |
| `configs/grafana/` | Grafana config + provisioning |
