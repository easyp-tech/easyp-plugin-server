<!-- generated: 2026-04-04, template: observability.md -->
# Observability

## Stack

```
App → Alloy (collector) → Mimir (metrics) + Loki (logs) + Tempo (traces)
App → Pyroscope (profiles)
Grafana → Mimir + Loki + Tempo + Pyroscope (visualization)
```

All backends use **RustFS** (S3-compatible) for storage.

## Telemetry Init (`internal/telemetry/telemetry.go`)

`telemetry.Init()` configures:
1. **TracerProvider** — OTLP gRPC exporter to Alloy
2. **MeterProvider** — OTLP gRPC exporter, 15s periodic reader
3. **W3C TraceContext** propagation
4. **Pyroscope** profiler (CPU, alloc, inuse, goroutines)
5. **slog handler** wrapping with trace/span ID injection

Config:
```go
type Config struct {
    OTLPEndpoint      string // default "localhost:4317"
    ServiceName       string // default "easyp-api-service"
    PyroscopeEndpoint string // default "http://localhost:4040"
}
```

Graceful degradation: if any exporter fails to connect, service continues without that signal.

## Tracing

### Decorator Pattern

Three tracing decorators — wrap core interfaces without modifying business logic:

| Decorator | File | Wraps |
|-----------|------|-------|
| `TracingCore` | `telemetry/tracing_core.go` | `core.CoreService` |
| `TracingRegistry` | `telemetry/tracing_registry.go` | `core.Registry` |
| `TracingPlugin` | `telemetry/tracing_plugin.go` | `core.Plugin` |

Example:
```go
// telemetry/tracing_core.go
func (c *TracingCore) Generate(ctx context.Context, req core.GenerateCodeRequest) (*core.GenerateCodeResponse, error) {
    ctx, span := c.tracer.Start(ctx, "core.Generate",
        trace.WithAttributes(attribute.String("plugin.name", req.PluginName)))
    defer span.End()
    resp, err := c.inner.Generate(ctx, req)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }
    return resp, err
}
```

**TracingCore spans:** `core.Generate`, `core.ListPlugins`, `core.CreatePlugin`, `core.UpdatePlugin`, `core.DeletePlugin`

**TracingRegistry spans:** `registry.Get`, `registry.List`, `registry.Create`, `registry.Update`, `registry.Delete`

**TracingPlugin spans:** `plugin.Generate`, `plugin.Info`

### gRPC Tracing

- `otelgrpc.NewServerHandler()` as gRPC stats handler (automatic span creation per RPC)
- `TraceLoggingUnaryServerInterceptor` injects trace_id/span_id into slog context

### Database Tracing

`database.SQL.Tx()` and `NoTxContext()` create spans named after the calling method (via `internal.CallerMethodName`).

### Audit Worker Tracing

Each `audit.Worker.saveEntry()` creates a span `"audit.save"` with `db.system=postgresql` attribute.

### Worker Pool Tracing

`WorkerPool.Get()` creates span `"pool.Get"` with plugin group/name/version attributes and queue wait time.

## Metrics

### Prometheus Endpoints

Port **8081**, path `/metrics`.

### gRPC Metrics

Via `grpc-prometheus` ServerMetrics interceptor (position 3 in chain):
- `grpc_server_handled_total`
- `grpc_server_handling_seconds`
- `grpc_server_msg_received_total` / `grpc_server_msg_sent_total`

### Database Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `{ns}_{subsystem}_errors_total` | Counter | `func` |
| `{ns}_{subsystem}_call_duration_seconds` | Histogram | `func` |

Plus standard `sql.DBStats` exported via `database.SQL.UnderlyingDB()`.

### Worker Pool Metrics

| Metric | Type |
|--------|------|
| `pool_active_workers` | Gauge |
| `pool_rejected_total` | Counter |
| `pool_jobs_total` | Counter |
| `pool_queue_depth` | GaugeFunc |

### Rate Limiter Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `easyp_rate_limit_requests_total` | Counter | `status`, `client_ip` |
| `easyp_rate_limit_active_clients` | Gauge | — |

### Audit Metrics

| Metric | Type |
|--------|------|
| `audit_events_lost_total` | Counter |
| `audit_queue_depth` | GaugeFunc |

### License Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `license_tier` | Gauge | `tier` |
| `license_valid` | Gauge | — |
| `license_expiry_seconds` | Gauge | — |
| `license_feature_denied_total` | Counter | `feature` |

### Panic Metrics

`grpc_recovery` interceptor increments `panics_total` counter via `grpchelper.Metrics.PanicsTotal()`.

## Logging

Structured JSON via `slog`. The `TraceHandler` (`telemetry/trace_handler.go`) injects `trace_id` and `span_id` into every log record when a span is active.

Log level configurable via `-log_level` flag (debug/info/warn/error).

## Profiling

Pyroscope continuous profiling:
- CPU, AllocObjects, AllocSpace, InuseObjects, InuseSpace, Goroutines
- `pyroscope.TagWrapper` used in WorkerPool to label operations

## Grafana Dashboards

Pre-provisioned via `configs/grafana/provisioning/`. Data sources: Mimir, Loki, Tempo, Pyroscope.

Access: `http://localhost:3000` (or `easyp.grafana.localhost` via Traefik).
