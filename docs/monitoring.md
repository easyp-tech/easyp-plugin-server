# Monitoring and Observability of EasyP API Service

## Observability Architecture

```
Service (OTEL SDK) ──→ Alloy (collector) ──→ Mimir (metrics)
                                          ──→ Loki (logs)
                                          ──→ Tempo (traces)

Service (Pyroscope SDK) ──→ Pyroscope (profiling)

Grafana ──→ Mimir / Loki / Tempo / Pyroscope (visualization)
```

All backends (Mimir, Loki, Tempo, Pyroscope) use S3-compatible storage RustFS.

## Telemetry Flows

### Traces

```
Service (OTLP gRPC) → Alloy → Tempo (S3 backend via RustFS)
```

### Metrics

```
Service (OTLP gRPC) → Alloy → Mimir (S3 backend)
Service (:8081/metrics) → Alloy (Prometheus scraping) → Mimir
```

### Logs

```
Docker container logs → Alloy → Loki (S3 backend)
```

### Profiling

```
Service (Pyroscope SDK) → Pyroscope server (S3 backend)
```

## Endpoints

| Service | URL | Description |
|---------|-----|-------------|
| Grafana | `http://localhost:3000` | Dashboards (admin/admin) |
| Prometheus (Mimir) | via Grafana | Metrics storage |
| Health | `http://localhost:8082/health` | Health check |
| Metrics | `http://localhost:8081/metrics` | Prometheus scrape endpoint |
| Alloy UI | `http://localhost:12345` | Collector status |
| RustFS Console | `http://localhost:9001` | S3 storage UI |

## Metrics

### Code Generation

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `generated_plugin_code_total` | Counter | `plugin` | Number of generations per plugin |
| `generation_duration_seconds` | Histogram | `plugin` | Generation latency |
| `generation_errors_total` | Counter | `plugin`, `error_type` | Errors (transient/permanent) |
| `generation_retries_total` | Counter | `plugin` | Number of retries |

### Worker Pool

| Metric | Type | Description |
|--------|------|-------------|
| `pool_active_workers` | Gauge | Current number of active workers |
| `pool_queue_depth` | Gauge | Number of tasks in the queue |
| `pool_rejected_total` | Counter | Rejected tasks (queue full) |
| `pool_jobs_total` | Counter | Total accepted tasks |

### Database

| Metric | Type | Description |
|--------|------|-------------|
| `db_open_connections` | Gauge | Open connections |
| `db_idle_connections` | Gauge | Idle connections |
| `db_wait_count_total` | Counter | Number of connection waits |
| `db_wait_duration_seconds_total` | Counter | Total wait time |

### Business Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `business_plugins_total` | Gauge | — | Total number of registered plugins |
| `business_plugins_by_group` | Gauge | `group` | Plugins by group |
| `business_audit_log_total` | Gauge | — | Total audit log entries |
| `business_audit_log_last_24h` | Gauge | — | Audit log entries in the last 24 hours |
| `business_audit_log_by_operation` | Gauge | `operation` | Audit by operation type |
| `business_audit_log_by_status` | Gauge | `status` | Audit by status |

### Audit

| Metric | Type | Description |
|--------|------|-------------|
| `audit_events_lost_total` | Counter | Lost audit events (channel overflow) |
| `audit_queue_depth` | Gauge | Audit queue depth |

### Rate Limiting

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `easyp_rate_limit_requests_total` | Counter | `status` (allowed/denied), `client_ip` | Total requests processed by rate limiter |
| `easyp_rate_limit_active_clients` | Gauge | — | Current number of active client buckets |

### License

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `easyp_license_valid` | Gauge | — | 1 when the license is valid, 0 when invalid or absent |
| `easyp_license_expiry_timestamp_seconds` | Gauge | — | Unix timestamp of the license expiration |
| `easyp_license_feature_denied_total` | Counter | `feature` | Number of feature access denials per feature |

### System

| Metric | Type | Description |
|--------|------|-------------|
| `panics_total` | Counter | Recovered panics in gRPC handlers |
| gRPC server metrics | — | Standard grpc-ecosystem/go-grpc-middleware metrics |

## Tracing

All key operations are covered by traces:

| Operation | Description |
|-----------|-------------|
| `Core.Generate` | Full code generation cycle |
| `Core.ListPlugins` | Retrieving the list of plugins |
| `Registry.Get` | Retrieving a plugin from the registry |
| `Registry.List` | Listing plugins from the registry |
| `Plugin.Generate` | Executing a plugin (Docker) |
| `pool.Get` | Retrieving a task from the pool |
| `audit.save` | Saving an audit event |

### Span Attributes

| Attribute | Description |
|-----------|-------------|
| `plugin.group` | Plugin group |
| `plugin.name` | Plugin name |
| `plugin.version` | Plugin version |
| `db.system` | Database system (postgresql) |
| `db.operation` | Operation type (SELECT, INSERT, etc.) |

## Profiling

Pyroscope provides continuous profiling:

| Profile | Description |
|---------|-------------|
| CPU | CPU usage |
| Alloc objects | Number of allocations |
| Alloc space | Allocation volume |
| Inuse objects | Objects in memory |
| Inuse space | Memory in use |
| Goroutines | Number of goroutines |

Worker pool tasks are tagged with `operation=worker.process_job` for filtering profiles by generation.

## Example PromQL Queries

### Generation Rate (RPS)

```promql
rate(generated_plugin_code_total[5m])
```

### 95th Percentile Generation Latency

```promql
histogram_quantile(0.95, rate(generation_duration_seconds_bucket[5m]))
```

### Error Rate

```promql
rate(generation_errors_total[5m]) / rate(generated_plugin_code_total[5m]) * 100
```

### Worker Pool Utilization

```promql
pool_active_workers / 4 * 100  # utilization percentage (with 4 workers)
```

### Database Connection Pool Usage

```promql
db_open_connections / 50 * 100  # usage percentage
```

### License Status

```promql
easyp_license_valid
```

### Time Until License Expiration

```promql
easyp_license_expiry_timestamp_seconds - time()
```

### Feature Denial Rate

```promql
rate(easyp_license_feature_denied_total[5m])
```

### Top Denied Features

```promql
topk(5, sum by (feature) (rate(easyp_license_feature_denied_total[5m])))
```

### Rate Limit Denial Rate

```promql
rate(easyp_rate_limit_requests_total{status="denied"}[5m])
```

### Rate Limit Allow/Deny Ratio

```promql
sum(rate(easyp_rate_limit_requests_total{status="denied"}[5m])) / sum(rate(easyp_rate_limit_requests_total[5m])) * 100
```

### Top Rate-Limited Clients

```promql
topk(10, sum by (client_ip) (rate(easyp_rate_limit_requests_total{status="denied"}[5m])))
```

### Active Rate Limit Buckets

```promql
easyp_rate_limit_active_clients
```
