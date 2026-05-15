<!-- generated: 2026-05-15, template: security.md -->
# Security

Security considerations for EasyP Service.

## Docker Isolation

Plugin containers run with strict isolation:

| Constraint | Value |
|-----------|-------|
| Network | `--network=none` (no network access) |
| Memory | `--memory=128m` |
| CPU | `--cpus=1.0` |
| User | `nobody` (non-root) |
| Base image | `scratch` (minimal attack surface) |
| Binary | Static, compressed with `upx` |

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
- **License keys** — via PASETO v4 tokens (public-key cryptography)
- **No secrets in source** — `.env.example` provides template; `.gitignore` excludes `.env`
- **Docker socket** — required mount, represents a security boundary

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

## Docker Socket Access

**Security consideration:** the service requires `/var/run/docker.sock` mount. This gives the service full Docker daemon access. In production:
- Run the service with minimal Docker permissions
- Consider using Docker-in-Docker or rootless Docker
- Network policy: restrict outbound from the service host
