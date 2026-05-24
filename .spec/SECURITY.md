<!-- generated: 2026-05-24, template: security.md -->
# Security

Security considerations for EasyP Service.

## Plugin Isolation

Plugin binaries are executed as local processes with the following characteristics:

| Aspect | Detail |
|--------|--------|
| Execution | Local binary execution via stdin/stdout |
| Source | Built from multi-stage Dockerfiles in `registry/` |
| Binary type | Static, compressed with `upx` |
| Base image (build) | `golang:alpine` → `scratch` |
| User (build) | `nobody` (non-root) |
| Communication | stdin (CodeGeneratorRequest) → stdout (CodeGeneratorResponse) |
| Timeout | Configurable per-request deadline (`generation_timeout: 120s`) |
| Output limit | `max_output_size: 67108864` (64MB) |

**Note:** Plugin binaries are built from Dockerfiles but executed as local processes, not in Docker containers at runtime. The WorkerPool limits concurrent executions.

## Input Validation

### Plugin Name Format

Strictly validated via regex: `^[a-z][a-z0-9-]*/[a-z][a-z0-9-]*:(v\d+\.\d+\.\d+|latest)$`

### Protobuf Validation

gRPC interceptor validates all incoming protobuf messages before handler execution.

### String Sanitization

All string inputs are `strings.TrimSpace()` cleaned before processing.

## Rate Limiting

Per-IP token bucket rate limiter (`internal/ratelimiter`):

| Parameter | Default |
|-----------|---------|
| Requests per second | 10.0 |
| Burst | 20 |
| Cleanup interval | 10m |

Rate limits are integrated with FeatureGate for tier-based configuration.

## Secrets Management

- **Database credentials** — via environment variable `DB_POSTGRES_DSN` or YAML config
- **License keys** — via PASETO v4 tokens (public-key cryptography), passed via `LICENSE_KEY` env var
- **No secrets in source** — `.env.example` provides template; `.gitignore` excludes `.env`
- **Plugin binaries** — built from auditable Dockerfiles in `registry/`

## Error Information Exposure

- gRPC responses include error messages from domain errors
- Internal stack traces are NOT exposed to clients
- Panic recovery interceptor catches panics and returns `Internal` status

## Audit Trail

All operations are audit-logged asynchronously:
- Caller IP address
- Operation type
- Plugin name
- Success/error status
- Error code and message
- Duration
- Custom metadata

See `BACKGROUND_JOBS.md` for audit worker details.

## WorkerPool Security

The WorkerPool provides execution containment:
- **Bounded concurrency** — limits parallel plugin executions (default: 4 workers)
- **Non-blocking backpressure** — returns `ErrServerOverloaded` when queue is full
- **Timeout enforcement** — per-request deadline prevents runaway processes
- **Retry limits** — configurable max retries for transient failures
