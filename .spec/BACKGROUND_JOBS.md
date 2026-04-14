<!-- generated: 2026-04-14, template: background-jobs.md -->
# Background Jobs

## 1. Overview

Two background processing systems:
- **WorkerPool** — Bounded concurrency for Docker plugin execution (channel-based goroutine pool)
- **AuditWorker** — Async audit log writer (single goroutine consuming from buffered channel)

Both are in-process (same binary), use Go channels as transport, no external queue.

## 2. Job Inventory

| Job | Trigger | Concurrency | Timeout | Retry | Priority |
|-----|---------|-------------|---------|-------|----------|
| Plugin execution | `GenerateCode` RPC | N workers (default 4) | 120s | 2× | normal |
| Audit log write | gRPC interceptor | 1 worker | — | 0 | low |
| License expiration check | Timer | 1 (ticker) | — | 0 | low |
| Rate limiter cleanup | Timer | 1 (ticker) | — | 0 | low |

## 3. Architecture

### Worker Pool (Plugin Execution)

```
API.GenerateCode()
  → Core.Generate()
    → WorkerPool.Get() [enqueue job to channel]
      ↓ (non-blocking, ErrServerOverloaded if full)
    Worker goroutine #1..N
      → Registry.Get() [SQL query]
      → poolPlugin.Generate() [Docker run with retry]
        ↓ (timeout context)
      ← Result / Error
    ← Return to caller via result channel
```

### Audit Worker

```
AuditInterceptor (gRPC)
  → channel send (non-blocking, cap 1000)
    ↓ (overflow → log warning, increment lost counter)
  AuditWorker goroutine
    → AuditStore.Save() [SQL INSERT]
```

## 4. Retry & Error Handling

### WorkerPool Retries
- **Max retries**: Configurable (default 2)
- **Backoff**: None (immediate retry)
- **Transient errors**: Docker daemon errors, connection refused → retry
- **Permanent errors**: `context.DeadlineExceeded` → no retry
- **Timeout**: Generation timeout per attempt (default 120s)

### Audit Worker
- **No retry**: Single attempt per event
- **Overflow**: Channel full → event dropped, `audit_events_lost_total` incremented, warning logged
- **Graceful shutdown**: Drain remaining events with 5s timeout, return lost count

## 5. Concurrency & Ordering

### WorkerPool
- **Worker count**: Configurable (default 4), overridden by license `MaxWorkers`
- **Queue size**: Configurable (default 16)
- **Parallelism**: Multiple jobs execute concurrently (one per worker)
- **Ordering**: No ordering guarantees — FIFO within queue, but workers process independently
- **Backpressure**: `Get()` is non-blocking — returns `ErrServerOverloaded` immediately when queue full

### Audit Worker
- **Single goroutine**: Sequential writes, preserves order within channel
- **Channel capacity**: Fixed at 1000

## 6. Monitoring

| Metric | Type | Job | Description |
|--------|------|-----|-------------|
| `pool_active_workers` | Gauge | WorkerPool | Currently busy workers |
| `pool_rejected_total` | Counter | WorkerPool | Jobs rejected (queue full) |
| `pool_jobs_total` | Counter | WorkerPool | Total jobs processed |
| `pool_queue_depth` | Gauge | WorkerPool | Current queue depth |
| `audit_events_lost_total` | Counter | Audit | Events dropped (channel overflow) |
| `audit_queue_depth` | Gauge | Audit | Current audit channel depth |

## 7. Scaling

- **WorkerPool**: Scale workers via `worker_pool.workers` config or license claims
- **Queue size**: Via `worker_pool.queue_size` config
- **No horizontal scaling**: Both systems are in-process, single-instance
- **Backpressure**: WorkerPool rejects immediately, Audit drops events silently

## 8. Configuration

```yaml
worker_pool:
  workers: 4               # Number of goroutines
  queue_size: 16            # Buffered channel capacity
  generation_timeout: 120s  # Per-plugin execution timeout
  max_retries: 2            # Retry count for transient errors
  shutdown_timeout: 30s     # Graceful shutdown drain time
```

Audit worker: Fixed 1000-event channel, 5s shutdown timeout (hardcoded in `cmd/main.go`).

## 9. Key Files

| File | Description |
|------|-------------|
| `internal/core/pool.go` | WorkerPool implementation |
| `internal/core/pool_test.go` | WorkerPool tests |
| `internal/adapters/audit/worker.go` | AuditWorker implementation |
| `internal/adapters/audit/audit.go` | AuditStore (SQL persistence) |
| `internal/api/audit_interceptor.go` | Audit event producer |
