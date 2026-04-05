<!-- generated: 2026-04-04, template: security.md -->
# Security

## Trust Boundaries

```
┌─────────────────────────────────────────────────────┐
│  External Network                                    │
│  ┌─────────────┐                                     │
│  │ gRPC Client  │──→ :8080                           │
│  └─────────────┘                                     │
├─────────────────────────────────────────────────────┤
│  EasyP Service (container)                           │
│  ┌──────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │ gRPC API  │→│ Core (pool)  │→│ Docker Socket   │  │
│  └──────────┘  └──────────────┘  └───────┬───────┘  │
├─────────────────────────────────────────────────────┤
│  Plugin Containers (sandboxed)            │          │
│  ┌────────────────────────────────────────▼────────┐ │
│  │ --network=none --memory=128m --cpus=1.0         │ │
│  │ --user=nobody  (scratch image, static binary)   │ │
│  └─────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────┤
│  Internal Network                                    │
│  PostgreSQL :5432 │ Docker Registry :5005             │
└─────────────────────────────────────────────────────┘
```

## Plugin Sandboxing

Every plugin runs in an isolated Docker container with:

| Constraint | Value | Purpose |
|-----------|-------|---------|
| `--network=none` | No network | Prevents data exfiltration |
| `--memory=128m` | 128 MB limit | Prevents OOM of host |
| `--cpus=1.0` | 1 CPU | Prevents CPU starvation |
| `--user=nobody` | Non-root | Limits filesystem access |
| Base image | `scratch` | Minimal attack surface |
| Binary | Static, UPX-compressed | No dynamic libraries |

Plugin images are pre-built as multi-stage Docker images (`golang:alpine` → `scratch`).

## Rate Limiting

Per-client IP token bucket via `internal/ratelimiter/`:

| Parameter | Default | Config key |
|-----------|---------|------------|
| Tokens/sec | 10.0 | `rate_limit.requests_per_second` |
| Burst | 20 | `rate_limit.burst` |
| Cleanup | 10min | `rate_limit.cleanup_interval` |

- Key extraction: `PeerIPExtractor` uses `peer.FromContext()` (set by `realip` interceptor)
- Empty key → fail-open (no limiting)
- Gated by `FeatureGate.Enabled(FeatureRateLimiting)`
- Returns `RESOURCE_EXHAUSTED` when denied
- Sets rate limit headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`

## License Token Security

- **Algorithm:** PASETO v4.public (Ed25519 signatures)
- **Public key:** embedded at compile time via `-ldflags`
- **No symmetric secrets** stored on server — only public key for verification
- **Expiration watcher:** 60s ticker, reverts to Community on expiry
- **Token sources:** env var (`LICENSE_KEY`) or file (`LICENSE_FILE`), env takes priority

## gRPC Security

- **Panic recovery:** `grpc_recovery` interceptor catches panics → `INTERNAL` (never leaks stack traces)
- **Validation:** `grpc_validator` rejects malformed requests before handler
- **Error sanitization:** `ErrorToStatus()` maps internal errors to safe gRPC codes
- **Keepalive:** server enforces min 30s between pings, 10s timeout

## Audit Trail

All gRPC calls to `GenerateCode` and `Plugins` produce audit entries with:
- Caller IP address, operation type, plugin name
- Status (success/error), error code/message
- Duration, metadata, timestamps
- Stored in PostgreSQL `audit_log` table
- Non-blocking: buffer overflow drops events (logged as warning)

## Database

- Connection via DSN, `sslmode=disable` in dev (configure `sslmode=require` in production)
- No raw SQL outside `database.SQL` wrapper
- Transactions auto-rollback on panic

## Docker Socket

⚠️ The service requires Docker socket access (`/var/run/docker.sock`). In production, consider:
- Running with a restricted Docker socket proxy
- Using rootless Docker
- Applying AppArmor/SELinux profiles
