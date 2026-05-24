<!-- generated: 2026-05-24, template: auth.md -->
# Auth & Licensing

Authentication and licensing system for EasyP Service.

## Overview

EasyP uses **PASETO v4.public** tokens for license management. There is no user authentication — the service is designed to run within a trusted network. Access control is based on **license tiers** (community vs enterprise).

## License Tiers

| Tier | MaxWorkers | MaxPlugins | Features |
|------|-----------|------------|----------|
| Community | 4 | 10 | CodeGeneration, PluginListing, MCPServerTools, RateLimiting, PluginCRUD |
| Enterprise | Configurable | Unlimited (-1) | All community + MultiTenancy, ResponseCaching, Audit |

## Architecture

```
LicenseClient (gRPC)
  → Manager (cache + refresh watcher)
    → FeatureGate (feature checks)
      → Core (business logic)
      → RateLimiter (tier-based rates)
      → LicenseInterceptor (gRPC middleware)
```

## Components

### LicenseClient (`internal/license`)

Interface for communicating with the license server:

```go
type LicenseClient interface {
    ValidateLicense(ctx context.Context) (LicenseClaims, error)
}
```

**Current implementation:** `MockLicenseClient` — always returns Enterprise claims. This is a development placeholder; will be replaced with a real gRPC client when the license server is ready.

### Manager (`internal/license`)

- Caches license claims with configurable TTL
- Runs a background refresh watcher (`StartRefreshWatcher`)
- Reports metrics: cache hits/misses, validation duration, active tier

### FeatureGate (`internal/license`)

```go
type FeatureGate interface {
    Enabled(feature Feature) bool
    MaxWorkers() int
    MaxPlugins() int
}
```

Used by:
- `Core.checkFeature()` — before CRUD operations
- `RateLimiter` — for tier-based rate limits
- `WorkerPool` — for worker count limits
- `LicenseInterceptor` — gRPC middleware

### LicenseInterceptor (`internal/api`)

gRPC interceptor that checks feature availability before request processing. Applies to both unary and streaming RPCs.

## Configuration

```yaml
license:
  cache_ttl: 5m    # How long to cache license claims
```

Environment: `LICENSE_CACHE_TTL`

## PASETO Token Format

Tokens are `v4.public` PASETO tokens containing `LicenseClaims`:
- `tier` — "community" or "enterprise"
- `features` — list of Feature enum values
- `max_workers` — concurrent worker limit
- `max_plugins` — registered plugin limit
