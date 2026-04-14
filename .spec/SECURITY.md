<!-- generated: 2026-04-14, template: security.md -->
# Security

## 1. Security Overview

Service executes untrusted code (protoc plugins) in isolated Docker containers. No user authentication — licensing via PASETO v4 tokens. gRPC API with rate limiting.

```
┌─────────────────────────────────────────────────┐
│  External (untrusted)                            │
│  ┌──────────┐  ┌──────────┐                     │
│  │ easyp CLI│  │ SDK      │                     │
│  └────┬─────┘  └────┬─────┘                     │
│       └──────┬───────┘                           │
├──────────────┼───────────────────────────────────┤
│  Transport   │                                    │
│  ┌───────────▼────────────────────┐              │
│  │  gRPC Server + Interceptors   │              │
│  │  (rate limit, license, audit) │              │
│  └───────────┬────────────────────┘              │
├──────────────┼───────────────────────────────────┤
│  Internal    │                                    │
│  ┌───────────▼──────────┐  ┌──────────────────┐ │
│  │  Core + WorkerPool   │  │  PostgreSQL      │ │
│  └───────────┬──────────┘  └──────────────────┘ │
│              │                                    │
│  ┌───────────▼──────────────────┐                │
│  │  Docker (isolated plugins)   │                │
│  │  --network=none --memory=128m│                │
│  │  --cpus=1.0 --user=nobody    │                │
│  └──────────────────────────────┘                │
└─────────────────────────────────────────────────┘
```

## 2. Input Validation

- **Protobuf validation**: Automatic via `grpc_validator.UnaryServerInterceptor()`
- **Business validation**: Plugin name regex `^[a-z][a-z0-9-]*$`, version `^v\d+\.\d+\.\d+$`
- **Input sanitization**: `strings.TrimSpace()` on all string inputs
- **SQL injection**: Prevented — all queries use parameterized `sqlx` queries, never string concatenation
- **Command injection**: Docker container names derived from DB-stored config, validated at registration

| Input | Location | Validation |
|-------|----------|------------|
| plugin_name | GenerateCode RPC | Regex: `^[a-z][a-z0-9-]*/[a-z][a-z0-9-]*:(v\d+\.\d+\.\d+\|latest)$` |
| group, name | CreatePlugin RPC | Regex: `^[a-z][a-z0-9-]*$` |
| version | CreatePlugin RPC | Regex: `^v\d+\.\d+\.\d+$` |
| tags | CreatePlugin/Update | TrimSpace + empty filter |
| config | CreatePlugin/Update | JSONB marshaling validation |

## 3. Plugin Container Isolation

Plugins execute in **strongly isolated Docker containers**:

| Control | Setting | Purpose |
|---------|---------|---------|
| Network | `--network=none` | No network access |
| Memory | `--memory=128m` | OOM protection |
| CPU | `--cpus=1.0` | CPU throttling |
| User | `--user=nobody` | Non-root execution |
| Timeout | Configurable (120s default) | Prevent infinite execution |
| Filesystem | `--rm` (auto-cleanup) | No persistent state |
| I/O | stdin/stdout only | Protobuf request/response pipe |

## 4. Rate Limiting

- Per-IP token bucket via `golang.org/x/time/rate`
- Feature-gated (requires `FeatureRateLimiting`)
- Default: 10 req/sec, burst 20
- Background cleanup of stale buckets (10m interval)
- Returns standard rate limit headers

## 5. License Security

- **PASETO v4.public**: Asymmetric tokens (Ed25519), not JWT
- Public key injected at build time — cannot be overridden at runtime
- Token signature verified on every parse
- Expiration checked every 60 seconds
- Fallback to Community mode on any license error (fail-safe)

## 6. Secrets Management

| Secret | Storage | Risk |
|--------|---------|------|
| DB connection string | Config file / env var | Contains credentials |
| License key | Config file / env var | PASETO token |
| License public key | Compiled into binary | Ed25519 public key (not secret) |

Recommendations:
- Use environment variables in production, not config files
- Rotate DB credentials regularly
- Never log connection strings or license tokens

## 7. OWASP Top 10 Mapping

| # | Category | Mitigation | Status |
|---|----------|-----------|--------|
| A01 | Broken Access Control | License-based FeatureGate, no user model | ✅ Mitigated |
| A02 | Cryptographic Failures | PASETO v4 (Ed25519), no custom crypto | ✅ Mitigated |
| A03 | Injection | Parameterized SQL queries, no string concat | ✅ Mitigated |
| A04 | Insecure Design | Docker isolation, worker pool limits, rate limiting | ✅ Mitigated |
| A05 | Security Misconfiguration | Sensible defaults, explicit config | ✅ Mitigated |
| A06 | Vulnerable Components | `go.sum` lock file committed | ⚠️ Partial (no automated scanning) |
| A07 | Auth Failures | N/A — no user authentication | N/A |
| A08 | Data Integrity Failures | License token signature verification | ✅ Mitigated |
| A09 | Logging & Monitoring | Full OTEL stack, audit logging, structured logs | ✅ Mitigated |
| A10 | SSRF | `--network=none` on plugin containers, no user-controlled URLs | ✅ Mitigated |

## 8. Audit Trail

All API operations are recorded to `audit_log` table:
- Operation type, plugin name, caller IP
- Status (success/error), error details
- Duration, metadata (file counts, etc.)
- Async write via buffered channel (cap 1000)

See `DATABASE.md` for schema details.
