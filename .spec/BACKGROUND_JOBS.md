<!-- generated: 2026-04-04, template: background_jobs.md -->
# Background Jobs

## Overview

Two background subsystems run as goroutines alongside the gRPC server:

1. **Worker Pool** — bounded concurrency for Docker plugin execution
2. **Audit Worker** — async persistence of audit log entries

## Worker Pool (`internal/core/pool.go`)

### Purpose

Limits parallel Docker container executions to prevent resource exhaustion. Implements `core.Registry` interface, wrapping the real Registry.

### Configuration

```yaml
worker_pool:
  workers: 4               # Number of goroutines
  queue_size: 16            # Buffered channel capacity
  generation_timeout: 120s  # Per-generation context timeout
  max_retries: 3            # Retry attempts for transient errors
  shutdown_timeout: 30s     # Max wait for in-flight jobs
```

Normalization: `workers < 1 → 1`, `queue_size < 0 → 0`, `generation_timeout == 0 → 120s`, `max_retries == 0 → 2`, `shutdown_timeout == 0 → 30s`.

### Flow

```
Client → pool.Get() ─select→ jobs channel ─worker→ inner.Get() → poolPlugin
                      │                              ↓
                      └─default→ ErrServerOverloaded  poolPlugin.Generate()
                                                       ├─ timeout (120s)
                                                       ├─ retry (transient)
                                                       └─ metrics
```

1. `Get()` creates a job and attempts non-blocking send to buffered channel
2. If channel full → `ErrServerOverloaded` (gRPC `RESOURCE_EXHAUSTED`)
3. Worker picks job, calls `inner.Get()` to fetch plugin from registry
4. Returns `poolPlugin` wrapper that adds timeout + retry to `Generate()`

### Retry Logic

`poolPlugin.Generate()` retries on transient errors:

```go
func isTransient(err error) bool {
    // context.DeadlineExceeded → NOT transient
    // exec.ExitError with codes 125, 126, 127 → transient (Docker errors)
    // Error message contains "connection refused", "daemon", "temporary failure" → transient
}
```

### Shutdown

```go
func (p *WorkerPool) Shutdown(timeout time.Duration) int {
    p.closed.Store(true)  // Reject new jobs with ErrShuttingDown
    close(p.jobs)          // Signal workers to drain
    // Wait for workers or timeout
    // Returns count of lost jobs
}
```

### Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `pool_active_workers` | Gauge | Workers currently processing |
| `pool_rejected_total` | Counter | Jobs rejected (queue full) |
| `pool_jobs_total` | Counter | Jobs accepted |
| `pool_queue_depth` | GaugeFunc | Current queue length |

### Tracing

- `pool.Get` span with plugin attributes + `pool.queue_wait_ms`
- Pyroscope tag: `operation=worker.process_job`

## Audit Worker (`internal/adapters/audit/worker.go`)

### Purpose

Decouples audit log persistence from the gRPC request path. Reads entries from a buffered channel and writes to PostgreSQL.

### Architecture

```
gRPC Request → AuditInterceptor → chan core.AuditEntry (buffered) → Worker → audit.Store → PostgreSQL
                 (non-blocking send)                                  (blocking)
```

### Configuration

- Buffer size: set at `NewWorker()` call (typically 1000)
- Channel is shared: `NewWorker()` returns `(worker, chan<- core.AuditEntry)`

### AuditInterceptor (`internal/api/audit_interceptor.go`)

For each gRPC call:
1. Map method → operation type (`GENERATE_CODE` / `LIST_PLUGINS`)
2. Extract peer IP address
3. Call handler, measure duration
4. Build `core.AuditEntry` with UUID, metadata
5. Non-blocking send to channel (`select` with `default` → log warning)

### Worker Loop

```go
func (w *Worker) Run(ctx context.Context) {
    defer close(w.done)
    for entry := range w.entries {
        w.saveEntry(ctx, entry)  // With tracing span
    }
}
```

### Shutdown

```go
func (w *Worker) Shutdown(timeout time.Duration) int {
    close(w.entriesCh)  // Stop accepting new entries
    // Wait for done signal or timeout
    // Returns count of lost events
}
```

### Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `audit_events_lost_total` | Counter | Events dropped (timeout) |
| `audit_queue_depth` | GaugeFunc | Current buffer occupancy |

### Tracing

Each `saveEntry()` creates span `"audit.save"` with attributes: `db.system=postgresql`, `audit.operation_type`, `audit.entry_id`.

## License Expiration Watcher

A third background goroutine:

```go
// internal/license/manager.go
func (lm *LicenseManager) StartExpirationWatcher(ctx context.Context) {
    // Ticker every 60s → checkExpiration()
    // Reverts to CommunityDefaults on expiry
}
```

Stopped via `lm.Stop()` or context cancellation.
