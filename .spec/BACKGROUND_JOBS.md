<!-- generated: 2026-05-15, template: background-jobs.md -->
# Background Jobs

Background workers and async processing in EasyP Service.

## WorkerPool (`internal/core/pool.go`)

Bounded concurrency for Docker plugin execution.

### Architecture

```
Generate() request
  → Core.Generate()
    → WorkerPool.Get() — non-blocking job submission
      → buffered channel (QueueSize)
        → N worker goroutines (Workers)
          → Registry.Get() → Docker execution
        ← Plugin instance (poolPlugin wrapper)
      ← poolPlugin with timeout + retry
    → poolPlugin.Generate() — with timeout + retry
  ← CodeGeneratorResponse
```

### Configuration

```yaml
worker_pool:
  workers: 4              # Number of concurrent worker goroutines
  queue_size: 16           # Buffered channel capacity
  generation_timeout: 120s # Per-request Docker execution timeout
  max_retries: 2           # Retry count for transient errors
  shutdown_timeout: 30s    # Graceful shutdown deadline
```

### Backpressure

`WorkerPool.Get()` is **non-blocking** — if the queue is full, it returns `ErrServerOverloaded` immediately. It never blocks the calling goroutine.

### Retry Logic

`poolPlugin.Generate()` retries transient Docker errors (see `ERRORS.md` for transient classification):
- Max attempts = `MaxRetries + 1`
- Metrics: `IncGenerationRetries()` counter incremented on each retry

### Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `pool_active_workers` | Gauge | Workers currently processing |
| `pool_queue_depth` | Gauge | Jobs waiting in queue |
| `pool_rejected_total` | Counter | Jobs rejected (queue full) |
| `pool_jobs_total` | Counter | Jobs accepted |

### Graceful Shutdown

1. `closed` flag set → new `Get()` calls return `ErrShuttingDown`
2. `jobs` channel closed → workers drain remaining jobs
3. `wg.Wait()` with timeout
4. Returns count of lost jobs

## Audit Worker (`internal/adapters/audit`)

Async audit log persistence.

### Architecture

```
Core operations
  → sendAudit() — non-blocking channel send
    → auditCh (buffered, capacity=1000)
      → Worker.Run() goroutine
        → AuditLog.Save() → PostgreSQL
```

### Backpressure

Channel capacity is fixed at 1000. If exceeded:
- Events are silently dropped
- Warning logged

### Graceful Shutdown

`Worker.Shutdown(timeout)` drains remaining events and returns count of lost entries.

## License Refresh Watcher (`internal/license`)

Background goroutine that periodically refreshes license claims:

```go
lm.StartRefreshWatcher(ctx)
```

- Runs until context is cancelled
- Refresh interval based on `CacheTTL`
- On error: logs warning, continues with cached claims

## Rate Limiter Cleanup (`internal/ratelimiter`)

Background goroutine that cleans up stale per-IP rate limit buckets:

```go
rl.StartCleanup(ctx)
```

- Interval: configurable (`cleanup_interval`, default 10m)
- Removes buckets that haven't been accessed recently
